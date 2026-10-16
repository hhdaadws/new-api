package service

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestChannelCacheSampleCountsCacheInsideTotalInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name        string
		relayFormat types.RelayFormat
		channelType int
		usage       *dto.Usage
		want        model.ChannelCacheCounters
	}{
		{
			name:        "openai prompt already includes cache reads",
			relayFormat: types.RelayFormatOpenAI,
			usage:       &dto.Usage{PromptTokens: 1000, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 600}},
			want:        model.ChannelCacheCounters{RequestCount: 1, HitCount: 1, InputTokens: 1000, CacheReadTokens: 600},
		},
		{
			name:        "claude input excludes cache reads and writes",
			relayFormat: types.RelayFormatClaude,
			usage:       &dto.Usage{PromptTokens: 100, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 800, CachedCreationTokens: 100}},
			want:        model.ChannelCacheCounters{RequestCount: 1, HitCount: 1, InputTokens: 1000, CacheReadTokens: 800, CacheWriteTokens: 100},
		},
		{
			name:        "openai usage derived from claude excludes cache",
			relayFormat: types.RelayFormatOpenAI,
			usage:       &dto.Usage{PromptTokens: 62, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 3544}, ClaudeCacheCreation5mTokens: 586},
			want:        model.ChannelCacheCounters{RequestCount: 1, HitCount: 1, InputTokens: 4192, CacheReadTokens: 3544, CacheWriteTokens: 586},
		},
		{
			name:        "openrouter claude restores the input split off for billing",
			relayFormat: types.RelayFormatOpenAI,
			channelType: constant.ChannelTypeOpenRouter,
			usage:       &dto.Usage{PromptTokens: 1000, UsageSemantic: "anthropic", PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 600, CachedCreationTokens: 100}},
			want:        model.ChannelCacheCounters{RequestCount: 1, HitCount: 1, InputTokens: 1000, CacheReadTokens: 600, CacheWriteTokens: 100},
		},
		{
			name:        "openai cache reads above prompt cap the rate at full",
			relayFormat: types.RelayFormatOpenAI,
			usage:       &dto.Usage{PromptTokens: 100, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 150}},
			want:        model.ChannelCacheCounters{RequestCount: 1, HitCount: 1, InputTokens: 150, CacheReadTokens: 150},
		},
		{
			name:        "uncached request is a miss",
			relayFormat: types.RelayFormatOpenAI,
			usage:       &dto.Usage{PromptTokens: 500},
			want:        model.ChannelCacheCounters{RequestCount: 1, InputTokens: 500},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			relayInfo := &relaycommon.RelayInfo{
				RelayFormat:     tc.relayFormat,
				OriginModelName: "cache-model",
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: tc.channelType},
				PriceData: hosttypes.PriceData{
					ModelRatio:         1,
					CompletionRatio:    1,
					CacheRatio:         0.1,
					CacheCreationRatio: 1.25,
					GroupRatioInfo:     hosttypes.GroupRatioInfo{GroupRatio: 1},
				},
				StartTime: time.Now(),
			}

			summary := calculateTextQuotaSummary(ctx, relayInfo, tc.usage)
			got := channelCacheSample(summary, isLegacyClaudeDerivedOpenAIUsage(relayInfo, tc.usage))

			assert.Equal(t, tc.want, got)
		})
	}
}

// The configured DSNs must point at isolated test databases: the test drops
// and recreates channel_cache_stats there.
func TestChannelCacheStatsDatabaseMatrix(t *testing.T) {
	for _, dialect := range []struct {
		name common.DatabaseType
		env  string
	}{
		{common.DatabaseTypeSQLite, ""},
		{common.DatabaseTypeMySQL, "TEST_MYSQL_DSN"},
		{common.DatabaseTypePostgreSQL, "TEST_POSTGRES_DSN"},
	} {
		t.Run(string(dialect.name), func(t *testing.T) {
			var driver gorm.Dialector = sqlite.Open(":memory:")
			if dialect.env != "" {
				dsn := os.Getenv(dialect.env)
				if dsn == "" {
					t.Skip(dialect.env + " is not configured")
				}
				if dialect.name == common.DatabaseTypeMySQL {
					driver = mysql.Open(dsn)
				} else {
					driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
				}
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			oldDB, oldMainType, oldLogType := model.DB, common.MainDatabaseType(), common.LogDatabaseType()
			model.DB = db
			common.SetDatabaseTypes(dialect.name, dialect.name)
			channelCacheStatsLock.Lock()
			oldPending := pendingChannelCacheStats
			pendingChannelCacheStats = map[channelCacheStatKey]model.ChannelCacheCounters{}
			channelCacheStatsLock.Unlock()
			t.Cleanup(func() {
				model.DB = oldDB
				common.SetDatabaseTypes(oldMainType, oldLogType)
				channelCacheStatsLock.Lock()
				pendingChannelCacheStats = oldPending
				channelCacheStatsLock.Unlock()
			})

			require.NoError(t, db.Migrator().DropTable(&model.ChannelCacheStat{}))
			require.NoError(t, db.AutoMigrate(&model.ChannelCacheStat{}))
			require.NoError(t, db.AutoMigrate(&model.ChannelCacheStat{}), "migration must be idempotent")
			versionQuery := "select version()"
			if dialect.name == common.DatabaseTypeSQLite {
				versionQuery = "select sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database: %s", version)

			hit := model.ChannelCacheCounters{RequestCount: 1, HitCount: 1, InputTokens: 1000, CacheReadTokens: 800, CacheWriteTokens: 50}
			miss := model.ChannelCacheCounters{RequestCount: 1, InputTokens: 300}

			ObserveChannelCacheUsage(1, hit)
			ObserveChannelCacheUsage(1, miss)
			ObserveChannelCacheUsage(2, hit)
			ObserveChannelCacheUsage(2, model.ChannelCacheCounters{RequestCount: 1})
			FlushChannelCacheStats()
			ObserveChannelCacheUsage(1, hit)

			stats, err := GetChannelCacheStats([]int{1, 2, 3}, 24)
			require.NoError(t, err)
			assert.Equal(t, map[int]model.ChannelCacheCounters{
				1: {RequestCount: 3, HitCount: 2, InputTokens: 2300, CacheReadTokens: 1600, CacheWriteTokens: 100},
				2: {RequestCount: 1, HitCount: 1, InputTokens: 1000, CacheReadTokens: 800, CacheWriteTokens: 50},
			}, stats, "flushed and pending counters add up; requests without input are ignored")

			FlushChannelCacheStats()
			stats, err = GetChannelCacheStats([]int{1}, 24)
			require.NoError(t, err)
			assert.Equal(t, int64(3), stats[1].RequestCount, "a second flush adds to the saved bucket")

			const oldBucket = int64(1_700_000_400)
			old := model.ChannelCacheCounters{RequestCount: 2, HitCount: 1, InputTokens: 900, CacheReadTokens: 400}
			require.NoError(t, model.UpsertChannelCacheStat(&model.ChannelCacheStat{ChannelId: 1, BucketTs: oldBucket, ChannelCacheCounters: old}))
			require.NoError(t, model.UpsertChannelCacheStat(&model.ChannelCacheStat{ChannelId: 1, BucketTs: oldBucket, ChannelCacheCounters: old}))
			var oldRows []model.ChannelCacheStat
			require.NoError(t, db.Where("channel_id = ? AND bucket_ts = ?", 1, oldBucket).Find(&oldRows).Error)
			require.Len(t, oldRows, 1, "one row per channel and hour")
			assert.Equal(t, model.ChannelCacheCounters{RequestCount: 4, HitCount: 2, InputTokens: 1800, CacheReadTokens: 800}, oldRows[0].ChannelCacheCounters)

			stats, err = GetChannelCacheStats([]int{1}, 24)
			require.NoError(t, err)
			assert.Equal(t, int64(3), stats[1].RequestCount, "buckets before the window are excluded")
			rows, err := model.SumChannelCacheStats([]int{1}, oldBucket)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, int64(7), rows[0].RequestCount)

			require.NoError(t, model.DeleteChannelCacheStatsBefore(oldBucket+1))
			rows, err = model.SumChannelCacheStats([]int{1}, 0)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, int64(3), rows[0].RequestCount, "cleanup removes only expired buckets")
		})
	}
}
