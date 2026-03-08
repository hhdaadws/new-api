package model

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/go-redis/redis/v8"
)

const channelBindingHashTTL = 24 * time.Hour

// channelUserBindings stores channel_id → user_id → last_used_time (unix seconds)
// Only used when Redis is disabled. Protected by bindingCacheLock.
var channelUserBindings map[int]map[int]int64

// bindingCacheLock protects channelUserBindings (only used when Redis is disabled)
var bindingCacheLock sync.RWMutex

func channelBindingRedisKey(channelId int) string {
	return fmt.Sprintf("channel_binding:%d", channelId)
}

func InitChannelUserBindingCache() {
	// DB query outside any lock to avoid holding lock during I/O
	bindingsMap, err := GetAllBindingsMap()
	if err != nil {
		common.SysError("failed to load channel user bindings: " + err.Error())
		bindingsMap = make(map[int]map[int]int64)
	}

	// Read channel config for expiration cleaning
	channelSyncLock.RLock()
	type channelExpireConfig struct {
		expireMinutes int
	}
	channelConfigs := make(map[int]channelExpireConfig)
	if channelsIDM != nil {
		for channelId, ch := range channelsIDM {
			expireMinutes := ch.GetUserBindExpireMinutes()
			if expireMinutes > 0 {
				channelConfigs[channelId] = channelExpireConfig{expireMinutes: expireMinutes}
			}
		}
	}
	channelSyncLock.RUnlock()

	// Clean expired bindings
	for channelId, cfg := range channelConfigs {
		cutoff := time.Now().Add(-time.Duration(cfg.expireMinutes) * time.Minute).Unix()
		if users, ok := bindingsMap[channelId]; ok {
			for userId, lastUsed := range users {
				if lastUsed < cutoff {
					delete(users, userId)
				}
			}
			if len(users) == 0 {
				delete(bindingsMap, channelId)
			}
		}
		// Async clean from DB
		chId := channelId
		expMin := cfg.expireMinutes
		gopool.Go(func() {
			_, _ = CleanExpiredBindings(chId, expMin)
		})
	}

	if common.RedisEnabled {
		// Write all bindings to Redis via pipeline
		ctx := context.Background()
		pipe := common.RDB.TxPipeline()
		for channelId, users := range bindingsMap {
			key := channelBindingRedisKey(channelId)
			fields := make(map[string]interface{}, len(users))
			for userId, lastUsed := range users {
				fields[strconv.Itoa(userId)] = strconv.FormatInt(lastUsed, 10)
			}
			pipe.Del(ctx, key) // clear old data first
			if len(fields) > 0 {
				pipe.HSet(ctx, key, fields)
				pipe.Expire(ctx, key, channelBindingHashTTL)
			}
		}
		if _, err := pipe.Exec(ctx); err != nil {
			common.SysError("failed to sync channel user bindings to Redis: " + err.Error())
		}
		common.SysLog("channel user bindings cache synced to Redis from database")
	} else {
		bindingCacheLock.Lock()
		channelUserBindings = bindingsMap
		bindingCacheLock.Unlock()
		common.SysLog("channel user bindings cache synced from database (in-memory)")
	}
}

// CacheGetActiveBindingCount returns the number of non-expired bindings for a channel.
// When Redis is enabled, makes a direct Redis call (no lock needed).
// When Redis is disabled, caller must hold bindingCacheLock (at least RLock).
func CacheGetActiveBindingCount(channelId int, expireMinutes int) int {
	if common.RedisEnabled {
		return redisGetActiveBindingCount(channelId, expireMinutes)
	}
	return memGetActiveBindingCount(channelId, expireMinutes)
}

func redisGetActiveBindingCount(channelId int, expireMinutes int) int {
	ctx := context.Background()
	result, err := common.RDB.HGetAll(ctx, channelBindingRedisKey(channelId)).Result()
	if err != nil {
		common.SysError("failed to get channel binding count from Redis: " + err.Error())
		return 0
	}
	if expireMinutes <= 0 {
		return len(result)
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	count := 0
	for _, val := range result {
		lastUsed, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			continue
		}
		if lastUsed >= cutoff {
			count++
		}
	}
	return count
}

func memGetActiveBindingCount(channelId int, expireMinutes int) int {
	users, ok := channelUserBindings[channelId]
	if !ok {
		return 0
	}
	if expireMinutes <= 0 {
		return len(users)
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	count := 0
	for _, lastUsed := range users {
		if lastUsed >= cutoff {
			count++
		}
	}
	return count
}

// CacheGetSingleActiveBindingCount returns the active binding count for a single channel.
// Safe to call from controllers.
func CacheGetSingleActiveBindingCount(channelId int) int {
	channelSyncLock.RLock()
	expireMinutes := 0
	if ch, ok := channelsIDM[channelId]; ok {
		expireMinutes = ch.GetUserBindExpireMinutes()
	}
	channelSyncLock.RUnlock()

	if common.RedisEnabled {
		return redisGetActiveBindingCount(channelId, expireMinutes)
	}

	bindingCacheLock.RLock()
	defer bindingCacheLock.RUnlock()
	return memGetActiveBindingCount(channelId, expireMinutes)
}

// CacheIsUserBound checks if a user has an active (non-expired) binding to a channel.
// When Redis is enabled, makes a direct Redis call (no lock needed).
// When Redis is disabled, caller must hold bindingCacheLock (at least RLock).
func CacheIsUserBound(channelId, userId int, expireMinutes int) bool {
	if common.RedisEnabled {
		return redisIsUserBound(channelId, userId, expireMinutes)
	}
	return memIsUserBound(channelId, userId, expireMinutes)
}

func redisIsUserBound(channelId, userId int, expireMinutes int) bool {
	ctx := context.Background()
	val, err := common.RDB.HGet(ctx, channelBindingRedisKey(channelId), strconv.Itoa(userId)).Result()
	if err != nil {
		return false
	}
	if expireMinutes <= 0 {
		return true
	}
	lastUsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return false
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	return lastUsed >= cutoff
}

func memIsUserBound(channelId, userId int, expireMinutes int) bool {
	users, ok := channelUserBindings[channelId]
	if !ok {
		return false
	}
	lastUsed, bound := users[userId]
	if !bound {
		return false
	}
	if expireMinutes <= 0 {
		return true
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	return lastUsed >= cutoff
}

// CacheBindUser adds or updates a user-channel binding and writes to DB asynchronously.
func CacheBindUser(channelId, userId int) {
	now := time.Now().Unix()

	if common.RedisEnabled {
		ctx := context.Background()
		key := channelBindingRedisKey(channelId)
		pipe := common.RDB.TxPipeline()
		pipe.HSet(ctx, key, strconv.Itoa(userId), strconv.FormatInt(now, 10))
		pipe.Expire(ctx, key, channelBindingHashTTL)
		if _, err := pipe.Exec(ctx); err != nil {
			common.SysError("failed to bind user in Redis: " + err.Error())
		}
	} else {
		bindingCacheLock.Lock()
		if channelUserBindings == nil {
			channelUserBindings = make(map[int]map[int]int64)
		}
		if _, ok := channelUserBindings[channelId]; !ok {
			channelUserBindings[channelId] = make(map[int]int64)
		}
		channelUserBindings[channelId][userId] = now
		bindingCacheLock.Unlock()
	}

	CreateOrUpdateChannelUserBindingAsync(channelId, userId)
}

// CacheUpdateBindingLastUsed updates only the last_used_time for an existing binding.
func CacheUpdateBindingLastUsed(channelId, userId int) {
	now := time.Now().Unix()

	if common.RedisEnabled {
		ctx := context.Background()
		key := channelBindingRedisKey(channelId)
		// Only update if the field exists
		exists, err := common.RDB.HExists(ctx, key, strconv.Itoa(userId)).Result()
		if err != nil {
			common.SysError("failed to check binding existence in Redis: " + err.Error())
		}
		if exists {
			pipe := common.RDB.TxPipeline()
			pipe.HSet(ctx, key, strconv.Itoa(userId), strconv.FormatInt(now, 10))
			pipe.Expire(ctx, key, channelBindingHashTTL)
			if _, err := pipe.Exec(ctx); err != nil {
				common.SysError("failed to update binding last used in Redis: " + err.Error())
			}
		}
	} else {
		bindingCacheLock.Lock()
		if users, ok := channelUserBindings[channelId]; ok {
			if _, bound := users[userId]; bound {
				users[userId] = now
			}
		}
		bindingCacheLock.Unlock()
	}

	// Async DB update
	CreateOrUpdateChannelUserBindingAsync(channelId, userId)
}

// CacheUnbindUser removes a user-channel binding from cache and DB.
func CacheUnbindUser(channelId, userId int) {
	if common.RedisEnabled {
		ctx := context.Background()
		if err := common.RDB.HDel(ctx, channelBindingRedisKey(channelId), strconv.Itoa(userId)).Err(); err != nil {
			common.SysError("failed to unbind user in Redis: " + err.Error())
		}
	} else {
		bindingCacheLock.Lock()
		if users, ok := channelUserBindings[channelId]; ok {
			delete(users, userId)
			if len(users) == 0 {
				delete(channelUserBindings, channelId)
			}
		}
		bindingCacheLock.Unlock()
	}

	gopool.Go(func() {
		if err := DeleteChannelUserBinding(channelId, userId); err != nil {
			common.SysError("failed to delete channel user binding: " + err.Error())
		}
	})
}

// CacheUnbindAllUsers removes all user bindings for a channel from cache and DB.
func CacheUnbindAllUsers(channelId int) {
	if common.RedisEnabled {
		ctx := context.Background()
		if err := common.RDB.Del(ctx, channelBindingRedisKey(channelId)).Err(); err != nil {
			common.SysError("failed to unbind all users in Redis: " + err.Error())
		}
	} else {
		bindingCacheLock.Lock()
		delete(channelUserBindings, channelId)
		bindingCacheLock.Unlock()
	}

	gopool.Go(func() {
		if _, err := DeleteAllChannelUserBindings(channelId); err != nil {
			common.SysError("failed to delete all channel user bindings: " + err.Error())
		}
	})
}

// CacheGetAllActiveBindingCounts returns a map of channel_id → active binding count.
// Used for API responses.
func CacheGetAllActiveBindingCounts() map[int]int {
	// Get channel configs for expiration
	channelSyncLock.RLock()
	configs := make(map[int]int) // channelId → expireMinutes
	if channelsIDM != nil {
		for channelId, ch := range channelsIDM {
			configs[channelId] = ch.GetUserBindExpireMinutes()
		}
	}
	channelSyncLock.RUnlock()

	if common.RedisEnabled {
		return redisGetAllActiveBindingCounts(configs)
	}
	return memGetAllActiveBindingCounts(configs)
}

func redisGetAllActiveBindingCounts(configs map[int]int) map[int]int {
	result := make(map[int]int)
	channelIds := make([]int, 0, len(configs))
	for channelId := range configs {
		channelIds = append(channelIds, channelId)
	}
	if len(channelIds) == 0 {
		return result
	}

	// Pipeline HGETALL for all channels
	ctx := context.Background()
	pipe := common.RDB.Pipeline()
	cmds := make([]*redis.StringStringMapCmd, len(channelIds))
	for i, channelId := range channelIds {
		cmds[i] = pipe.HGetAll(ctx, channelBindingRedisKey(channelId))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("failed to get all binding counts from Redis: " + err.Error())
		return result
	}

	for i, channelId := range channelIds {
		data, err := cmds[i].Result()
		if err != nil || len(data) == 0 {
			continue
		}
		expireMinutes := configs[channelId]
		count := 0
		if expireMinutes <= 0 {
			count = len(data)
		} else {
			cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
			for _, val := range data {
				lastUsed, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					continue
				}
				if lastUsed >= cutoff {
					count++
				}
			}
		}
		if count > 0 {
			result[channelId] = count
		}
	}
	return result
}

func memGetAllActiveBindingCounts(configs map[int]int) map[int]int {
	bindingCacheLock.RLock()
	defer bindingCacheLock.RUnlock()

	result := make(map[int]int)
	for channelId, users := range channelUserBindings {
		expireMinutes := configs[channelId]
		count := 0
		if expireMinutes <= 0 {
			count = len(users)
		} else {
			cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
			for _, lastUsed := range users {
				if lastUsed >= cutoff {
					count++
				}
			}
		}
		if count > 0 {
			result[channelId] = count
		}
	}
	return result
}

// PreloadBindingData fetches binding data for multiple channels in one Redis pipeline.
// Returns channelId → userId → lastUsedTime map.
// When Redis is disabled, reads from in-memory cache.
func PreloadBindingData(channelIds []int) map[int]map[int]int64 {
	if len(channelIds) == 0 {
		return nil
	}

	if common.RedisEnabled {
		return redisPreloadBindingData(channelIds)
	}
	return memPreloadBindingData(channelIds)
}

func redisPreloadBindingData(channelIds []int) map[int]map[int]int64 {
	ctx := context.Background()
	pipe := common.RDB.Pipeline()
	cmds := make([]*redis.StringStringMapCmd, len(channelIds))

	for i, channelId := range channelIds {
		cmds[i] = pipe.HGetAll(ctx, channelBindingRedisKey(channelId))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("failed to preload binding data from Redis: " + err.Error())
		return make(map[int]map[int]int64)
	}

	result := make(map[int]map[int]int64, len(channelIds))
	for i, channelId := range channelIds {
		data, err := cmds[i].Result()
		if err != nil || len(data) == 0 {
			continue
		}
		users := make(map[int]int64, len(data))
		for userIdStr, lastUsedStr := range data {
			userId, err := strconv.Atoi(userIdStr)
			if err != nil {
				continue
			}
			lastUsed, err := strconv.ParseInt(lastUsedStr, 10, 64)
			if err != nil {
				continue
			}
			users[userId] = lastUsed
		}
		if len(users) > 0 {
			result[channelId] = users
		}
	}
	return result
}

func memPreloadBindingData(channelIds []int) map[int]map[int]int64 {
	bindingCacheLock.RLock()
	defer bindingCacheLock.RUnlock()

	result := make(map[int]map[int]int64, len(channelIds))
	for _, channelId := range channelIds {
		if users, ok := channelUserBindings[channelId]; ok {
			// Copy to avoid holding reference to locked data
			usersCopy := make(map[int]int64, len(users))
			for k, v := range users {
				usersCopy[k] = v
			}
			result[channelId] = usersCopy
		}
	}
	return result
}

// isUserBoundFromData checks binding from preloaded data.
func isUserBoundFromData(data map[int]map[int]int64, channelId, userId int, expireMinutes int) bool {
	users, ok := data[channelId]
	if !ok {
		return false
	}
	lastUsed, bound := users[userId]
	if !bound {
		return false
	}
	if expireMinutes <= 0 {
		return true
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	return lastUsed >= cutoff
}

// getActiveCountFromData counts active bindings from preloaded data.
func getActiveCountFromData(data map[int]map[int]int64, channelId int, expireMinutes int) int {
	users, ok := data[channelId]
	if !ok {
		return 0
	}
	if expireMinutes <= 0 {
		return len(users)
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	count := 0
	for _, lastUsed := range users {
		if lastUsed >= cutoff {
			count++
		}
	}
	return count
}
