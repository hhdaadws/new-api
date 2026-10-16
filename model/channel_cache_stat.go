package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChannelCacheCounters are the prompt cache totals of settled text requests.
// InputTokens includes cache reads and writes, so CacheReadTokens/InputTokens
// is the token cache hit rate whatever usage semantic the upstream reported.
type ChannelCacheCounters struct {
	RequestCount     int64 `json:"request_count" gorm:"default:0"`
	HitCount         int64 `json:"hit_count" gorm:"default:0"`
	InputTokens      int64 `json:"input_tokens" gorm:"default:0"`
	CacheReadTokens  int64 `json:"cache_read_tokens" gorm:"default:0"`
	CacheWriteTokens int64 `json:"cache_write_tokens" gorm:"default:0"`
}

func (c *ChannelCacheCounters) Add(other ChannelCacheCounters) {
	c.RequestCount += other.RequestCount
	c.HitCount += other.HitCount
	c.InputTokens += other.InputTokens
	c.CacheReadTokens += other.CacheReadTokens
	c.CacheWriteTokens += other.CacheWriteTokens
}

// ChannelCacheStat stores one channel's prompt cache counters for one hour.
// Every node adds its own counts, so rows are only ever incremented.
type ChannelCacheStat struct {
	Id        int   `json:"-" gorm:"primaryKey"`
	ChannelId int   `json:"channel_id" gorm:"uniqueIndex:idx_channel_cache_stat_bucket,priority:1"`
	BucketTs  int64 `json:"bucket_ts" gorm:"uniqueIndex:idx_channel_cache_stat_bucket,priority:2;index:idx_channel_cache_stat_bucket_ts"`
	ChannelCacheCounters
}

func (ChannelCacheStat) TableName() string {
	return "channel_cache_stats"
}

func UpsertChannelCacheStat(stat *ChannelCacheStat) error {
	if stat == nil || stat.RequestCount == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "channel_id"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"request_count":      gorm.Expr("channel_cache_stats.request_count + ?", stat.RequestCount),
			"hit_count":          gorm.Expr("channel_cache_stats.hit_count + ?", stat.HitCount),
			"input_tokens":       gorm.Expr("channel_cache_stats.input_tokens + ?", stat.InputTokens),
			"cache_read_tokens":  gorm.Expr("channel_cache_stats.cache_read_tokens + ?", stat.CacheReadTokens),
			"cache_write_tokens": gorm.Expr("channel_cache_stats.cache_write_tokens + ?", stat.CacheWriteTokens),
		}),
	}).Create(stat).Error
}

type ChannelCacheStatSummary struct {
	ChannelId int `json:"channel_id"`
	ChannelCacheCounters
}

// SumChannelCacheStats totals the buckets starting at or after startTs for
// each of the given channels; channels without buckets are absent.
func SumChannelCacheStats(channelIds []int, startTs int64) ([]ChannelCacheStatSummary, error) {
	var summaries []ChannelCacheStatSummary
	if len(channelIds) == 0 {
		return summaries, nil
	}
	err := DB.Model(&ChannelCacheStat{}).
		Select("channel_id, SUM(request_count) AS request_count, SUM(hit_count) AS hit_count, SUM(input_tokens) AS input_tokens, SUM(cache_read_tokens) AS cache_read_tokens, SUM(cache_write_tokens) AS cache_write_tokens").
		Where("channel_id IN ? AND bucket_ts >= ?", channelIds, startTs).
		Group("channel_id").
		Find(&summaries).Error
	return summaries, err
}

func DeleteChannelCacheStatsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&ChannelCacheStat{}).Error
}
