package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	channelCacheStatBucketSeconds = 3600
	channelCacheStatFlushInterval = time.Minute
	channelCacheStatRetention     = 30 * 24 * time.Hour
	// MaxChannelCacheStatHours is the longest window the retained buckets cover.
	MaxChannelCacheStatHours = 30 * 24
)

type channelCacheStatKey struct {
	channelId int
	bucketTs  int64
}

var (
	channelCacheStatsLock    sync.Mutex
	pendingChannelCacheStats = map[channelCacheStatKey]model.ChannelCacheCounters{}
)

// channelCacheSample turns one settled text request into cache counters.
// It follows the billing split: Anthropic usage (and OpenAI usage derived from
// it) reports cache reads and writes outside the prompt tokens, while other
// semantics already count cache reads inside them.
func channelCacheSample(summary textQuotaSummary, legacyClaudeDerived bool) model.ChannelCacheCounters {
	promptTokens := int64(max(summary.PromptTokens, 0))
	cacheReadTokens := int64(max(summary.CacheTokens, 0))
	cacheWriteTokens := int64(max(cacheWriteTokensTotal(summary), 0))
	inputTokens := max(promptTokens, cacheReadTokens)
	if summary.IsClaudeUsageSemantic || legacyClaudeDerived {
		inputTokens = promptTokens + cacheReadTokens + cacheWriteTokens
	}
	sample := model.ChannelCacheCounters{
		RequestCount:     1,
		InputTokens:      inputTokens,
		CacheReadTokens:  cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
	}
	if cacheReadTokens > 0 {
		sample.HitCount = 1
	}
	return sample
}

// ObserveChannelCacheUsage adds one request to the channel's current hourly
// bucket. Requests without input tokens say nothing about caching.
func ObserveChannelCacheUsage(channelId int, sample model.ChannelCacheCounters) {
	if channelId <= 0 || sample.InputTokens <= 0 {
		return
	}
	now := time.Now().Unix()
	key := channelCacheStatKey{channelId: channelId, bucketTs: now - now%channelCacheStatBucketSeconds}
	channelCacheStatsLock.Lock()
	defer channelCacheStatsLock.Unlock()
	counters := pendingChannelCacheStats[key]
	counters.Add(sample)
	pendingChannelCacheStats[key] = counters
}

func StartChannelCacheStatsTask() {
	go func() {
		ticker := time.NewTicker(channelCacheStatFlushInterval)
		defer ticker.Stop()
		var lastCleanup time.Time
		for range ticker.C {
			FlushChannelCacheStats()
			if !common.IsMasterNode || time.Since(lastCleanup) < time.Hour {
				continue
			}
			lastCleanup = time.Now()
			cutoff := lastCleanup.Add(-channelCacheStatRetention).Unix()
			if err := model.DeleteChannelCacheStatsBefore(cutoff); err != nil {
				common.SysError(common.LogText("failed to clean up channel cache stats: %s", err.Error()))
			}
		}
	}()
}

// FlushChannelCacheStats adds the pending counters to the database. Buckets
// that fail to save stay pending for the next flush.
func FlushChannelCacheStats() {
	channelCacheStatsLock.Lock()
	pending := pendingChannelCacheStats
	pendingChannelCacheStats = make(map[channelCacheStatKey]model.ChannelCacheCounters, len(pending))
	channelCacheStatsLock.Unlock()

	for key, counters := range pending {
		err := model.UpsertChannelCacheStat(&model.ChannelCacheStat{
			ChannelId:            key.channelId,
			BucketTs:             key.bucketTs,
			ChannelCacheCounters: counters,
		})
		if err == nil {
			continue
		}
		common.SysError(common.LogText("failed to save channel cache stats channel=%d bucket=%d: %s", key.channelId, key.bucketTs, err.Error()))
		channelCacheStatsLock.Lock()
		merged := pendingChannelCacheStats[key]
		merged.Add(counters)
		pendingChannelCacheStats[key] = merged
		channelCacheStatsLock.Unlock()
	}
}

// GetChannelCacheStats totals each channel's counters over the current hour
// and the hours-1 before it, including what this node has not flushed yet.
func GetChannelCacheStats(channelIds []int, hours int) (map[int]model.ChannelCacheCounters, error) {
	now := time.Now().Unix()
	startTs := now - now%channelCacheStatBucketSeconds - int64(hours-1)*channelCacheStatBucketSeconds
	rows, err := model.SumChannelCacheStats(channelIds, startTs)
	if err != nil {
		return nil, err
	}
	stats := make(map[int]model.ChannelCacheCounters, len(channelIds))
	for _, row := range rows {
		stats[row.ChannelId] = row.ChannelCacheCounters
	}

	requested := make(map[int]struct{}, len(channelIds))
	for _, id := range channelIds {
		requested[id] = struct{}{}
	}
	channelCacheStatsLock.Lock()
	defer channelCacheStatsLock.Unlock()
	for key, counters := range pendingChannelCacheStats {
		if _, ok := requested[key.channelId]; !ok || key.bucketTs < startTs {
			continue
		}
		merged := stats[key.channelId]
		merged.Add(counters)
		stats[key.channelId] = merged
	}
	return stats, nil
}
