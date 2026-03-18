package model

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/go-redis/redis/v8"
)

//go:embed lua/bind_session_if_room.lua
var bindSessionIfRoomScript string

var bindSessionIfRoomSHA string

const channelSessionBindingHashTTL = 24 * time.Hour

// channelSessionBindings stores channel_id → session_id → sessionBindingEntry
// Only used when Redis is disabled. Protected by sessionBindingCacheLock.
var channelSessionBindings map[int]map[string]sessionBindingEntry

// sessionToChannel stores session_id → channel_id (reverse index for O(1) lookup)
// Only used when Redis is disabled. Protected by sessionBindingCacheLock.
var sessionToChannel map[string]int

// sessionBindingCacheLock protects channelSessionBindings and sessionToChannel (only used when Redis is disabled)
var sessionBindingCacheLock sync.RWMutex

func channelSessionBindingRedisKey(channelId int) string {
	return fmt.Sprintf("channel_session_binding:%d", channelId)
}

func sessionChannelRedisKey(sessionId string) string {
	return fmt.Sprintf("session_channel:%s", sessionId)
}

func encodeSessionBindingValue(userId int, lastUsedTime int64) string {
	return fmt.Sprintf("%d,%d", userId, lastUsedTime)
}

func decodeSessionBindingValue(val string) (userId int, lastUsedTime int64, ok bool) {
	parts := strings.SplitN(val, ",", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	uid, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return uid, ts, true
}

func InitBindSessionScript() {
	if common.RedisEnabled {
		sha, err := common.RDB.ScriptLoad(context.Background(), bindSessionIfRoomScript).Result()
		if err != nil {
			common.SysError("failed to load bind_session_if_room script: " + err.Error())
		}
		bindSessionIfRoomSHA = sha
	}
}

func InitSessionBindingCache() {
	InitBindSessionScript()
	// DB query outside any lock to avoid holding lock during I/O
	bindingsMap, err := GetAllSessionBindingsMap()
	if err != nil {
		common.SysError("failed to load channel session bindings: " + err.Error())
		bindingsMap = make(map[int]map[string]sessionBindingEntry)
	}

	// Read channel config for expiration cleaning
	channelSyncLock.RLock()
	type channelExpireConfig struct {
		expireMinutes int
	}
	channelConfigs := make(map[int]channelExpireConfig)
	if channelsIDM != nil {
		for channelId, ch := range channelsIDM {
			expireMinutes := ch.GetSessionBindExpireMinutes()
			if expireMinutes > 0 {
				channelConfigs[channelId] = channelExpireConfig{expireMinutes: expireMinutes}
			}
		}
	}
	channelSyncLock.RUnlock()

	// Build reverse index and clean expired bindings
	reverseIndex := make(map[string]int)
	for channelId, sessions := range bindingsMap {
		cfg, hasCfg := channelConfigs[channelId]
		for sid, entry := range sessions {
			if hasCfg {
				cutoff := time.Now().Add(-time.Duration(cfg.expireMinutes) * time.Minute).Unix()
				if entry.LastUsedTime < cutoff {
					delete(sessions, sid)
					continue
				}
			}
			reverseIndex[sid] = channelId
		}
		if len(sessions) == 0 {
			delete(bindingsMap, channelId)
		}
		// Async clean from DB
		if hasCfg {
			chId := channelId
			expMin := cfg.expireMinutes
			gopool.Go(func() {
				_, _ = CleanExpiredSessionBindings(chId, expMin)
			})
		}
	}

	if common.RedisEnabled {
		// Write all bindings to Redis via pipeline
		ctx := context.Background()
		pipe := common.RDB.TxPipeline()
		for channelId, sessions := range bindingsMap {
			key := channelSessionBindingRedisKey(channelId)
			fields := make(map[string]interface{}, len(sessions))
			for sid, entry := range sessions {
				fields[sid] = encodeSessionBindingValue(entry.UserId, entry.LastUsedTime)
			}
			pipe.Del(ctx, key) // clear old data first
			if len(fields) > 0 {
				pipe.HSet(ctx, key, fields)
				pipe.Expire(ctx, key, channelSessionBindingHashTTL)
			}
		}
		// Set reverse index keys
		for sid, channelId := range reverseIndex {
			rKey := sessionChannelRedisKey(sid)
			pipe.Set(ctx, rKey, strconv.Itoa(channelId), channelSessionBindingHashTTL)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			common.SysError("failed to sync channel session bindings to Redis: " + err.Error())
		}
		common.SysLog("channel session bindings cache synced to Redis from database")
	} else {
		sessionBindingCacheLock.Lock()
		channelSessionBindings = bindingsMap
		sessionToChannel = reverseIndex
		sessionBindingCacheLock.Unlock()
		common.SysLog("channel session bindings cache synced from database (in-memory)")
	}
}

// CacheGetSessionChannel returns the channel ID for a session binding.
// Checks expiration based on channel configuration.
func CacheGetSessionChannel(sessionId string) (int, bool) {
	if sessionId == "" {
		return 0, false
	}
	if common.RedisEnabled {
		return redisGetSessionChannel(sessionId)
	}
	return memGetSessionChannel(sessionId)
}

func redisGetSessionChannel(sessionId string) (int, bool) {
	ctx := context.Background()
	// O(1) lookup via reverse index
	val, err := common.RDB.Get(ctx, sessionChannelRedisKey(sessionId)).Result()
	if err != nil {
		return 0, false
	}
	channelId, err := strconv.Atoi(val)
	if err != nil {
		return 0, false
	}

	// Verify the session still exists in the channel hash and check expiration
	entryVal, err := common.RDB.HGet(ctx, channelSessionBindingRedisKey(channelId), sessionId).Result()
	if err != nil {
		// Reverse index stale, clean it up
		common.RDB.Del(ctx, sessionChannelRedisKey(sessionId))
		return 0, false
	}

	_, lastUsedTime, ok := decodeSessionBindingValue(entryVal)
	if !ok {
		return 0, false
	}

	// Check expiration
	channelSyncLock.RLock()
	expireMinutes := 0
	if ch, ok := channelsIDM[channelId]; ok {
		expireMinutes = ch.GetSessionBindExpireMinutes()
	}
	channelSyncLock.RUnlock()

	if expireMinutes > 0 {
		cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
		if lastUsedTime < cutoff {
			// Expired, clean up
			redisUnbindSession(channelId, sessionId)
			return 0, false
		}
	}

	return channelId, true
}

func memGetSessionChannel(sessionId string) (int, bool) {
	sessionBindingCacheLock.RLock()
	channelId, ok := sessionToChannel[sessionId]
	if !ok {
		sessionBindingCacheLock.RUnlock()
		return 0, false
	}

	// Verify session exists in channel's bindings
	sessions, exists := channelSessionBindings[channelId]
	if !exists {
		sessionBindingCacheLock.RUnlock()
		return 0, false
	}
	entry, bound := sessions[sessionId]
	sessionBindingCacheLock.RUnlock()

	if !bound {
		return 0, false
	}

	// Acquire channelSyncLock separately (not nested) to read expire config
	channelSyncLock.RLock()
	expireMinutes := 0
	if ch, ok := channelsIDM[channelId]; ok {
		expireMinutes = ch.GetSessionBindExpireMinutes()
	}
	channelSyncLock.RUnlock()

	if expireMinutes > 0 {
		cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
		if entry.LastUsedTime < cutoff {
			return 0, false
		}
	}

	return channelId, true
}

// cacheGetActiveSessionBindingCount returns the number of non-expired session bindings for a channel.
// Internal use only — caller must hold sessionBindingCacheLock (at least RLock) when Redis is disabled.
func cacheGetActiveSessionBindingCount(channelId int, expireMinutes int) int {
	if common.RedisEnabled {
		return redisGetActiveSessionBindingCount(channelId, expireMinutes)
	}
	return memGetActiveSessionBindingCount(channelId, expireMinutes)
}

func redisGetActiveSessionBindingCount(channelId int, expireMinutes int) int {
	ctx := context.Background()
	result, err := common.RDB.HGetAll(ctx, channelSessionBindingRedisKey(channelId)).Result()
	if err != nil {
		common.SysError("failed to get session binding count from Redis: " + err.Error())
		return 0
	}
	if expireMinutes <= 0 {
		return len(result)
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	count := 0
	for _, val := range result {
		_, lastUsed, ok := decodeSessionBindingValue(val)
		if !ok {
			continue
		}
		if lastUsed >= cutoff {
			count++
		}
	}
	return count
}

func memGetActiveSessionBindingCount(channelId int, expireMinutes int) int {
	sessions, ok := channelSessionBindings[channelId]
	if !ok {
		return 0
	}
	if expireMinutes <= 0 {
		return len(sessions)
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	count := 0
	for _, entry := range sessions {
		if entry.LastUsedTime >= cutoff {
			count++
		}
	}
	return count
}

// CacheGetSingleActiveSessionBindingCount returns the active session binding count for a single channel.
// Safe to call from controllers.
func CacheGetSingleActiveSessionBindingCount(channelId int) int {
	channelSyncLock.RLock()
	expireMinutes := 0
	if ch, ok := channelsIDM[channelId]; ok {
		expireMinutes = ch.GetSessionBindExpireMinutes()
	}
	channelSyncLock.RUnlock()

	if common.RedisEnabled {
		return redisGetActiveSessionBindingCount(channelId, expireMinutes)
	}

	sessionBindingCacheLock.RLock()
	defer sessionBindingCacheLock.RUnlock()
	return memGetActiveSessionBindingCount(channelId, expireMinutes)
}

// CacheBindSessionIfRoom atomically checks capacity and creates a session binding.
// Returns true if the session was bound (or was already actively bound), false if channel is full.
func CacheBindSessionIfRoom(channelId int, sessionId string, userId, maxSessions, expireMinutes int) bool {
	var bound bool
	if common.RedisEnabled {
		bound = redisBindSessionIfRoom(channelId, sessionId, userId, maxSessions, expireMinutes)
	} else {
		bound = memBindSessionIfRoom(channelId, sessionId, userId, maxSessions, expireMinutes)
	}
	if bound {
		CreateOrUpdateChannelSessionBindingAsync(channelId, sessionId, userId)
	}
	return bound
}

func redisBindSessionIfRoom(channelId int, sessionId string, userId, maxSessions, expireMinutes int) bool {
	ctx := context.Background()
	key := channelSessionBindingRedisKey(channelId)
	now := time.Now().Unix()
	var cutoff int64
	if expireMinutes > 0 {
		cutoff = time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	}

	keys := []string{key}
	args := []interface{}{
		sessionId,
		strconv.FormatInt(now, 10),
		strconv.Itoa(maxSessions),
		strconv.FormatInt(cutoff, 10),
		strconv.Itoa(userId),
	}

	var result int64
	var err error

	// Try EvalSha first, fall back to Eval on NOSCRIPT
	if bindSessionIfRoomSHA != "" {
		result, err = common.RDB.EvalSha(ctx, bindSessionIfRoomSHA, keys, args...).Int64()
		if err != nil && err.Error() == "NOSCRIPT No matching script. Please use EVAL." {
			result, err = common.RDB.Eval(ctx, bindSessionIfRoomScript, keys, args...).Int64()
		}
	} else {
		result, err = common.RDB.Eval(ctx, bindSessionIfRoomScript, keys, args...).Int64()
	}

	if err != nil {
		common.SysError("failed to run bind_session_if_room script: " + err.Error())
		return false
	}

	if result == 1 {
		// Update reverse index
		rKey := sessionChannelRedisKey(sessionId)
		ttl := channelSessionBindingHashTTL
		if expireMinutes > 0 {
			ttl = time.Duration(expireMinutes) * time.Minute
		}
		common.RDB.Set(ctx, rKey, strconv.Itoa(channelId), ttl)
	}

	return result == 1
}

func memBindSessionIfRoom(channelId int, sessionId string, userId, maxSessions, expireMinutes int) bool {
	now := time.Now().Unix()

	sessionBindingCacheLock.Lock()
	defer sessionBindingCacheLock.Unlock()

	if channelSessionBindings == nil {
		channelSessionBindings = make(map[int]map[string]sessionBindingEntry)
	}
	if sessionToChannel == nil {
		sessionToChannel = make(map[string]int)
	}

	sessions, ok := channelSessionBindings[channelId]
	if !ok {
		sessions = make(map[string]sessionBindingEntry)
		channelSessionBindings[channelId] = sessions
	}

	// Check if session is already actively bound
	if entry, bound := sessions[sessionId]; bound {
		if expireMinutes <= 0 || entry.LastUsedTime >= time.Now().Add(-time.Duration(expireMinutes)*time.Minute).Unix() {
			sessions[sessionId] = sessionBindingEntry{UserId: userId, LastUsedTime: now}
			sessionToChannel[sessionId] = channelId
			return true
		}
	}

	// Count active bindings
	activeCount := 0
	if expireMinutes <= 0 {
		activeCount = len(sessions)
	} else {
		cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
		for _, entry := range sessions {
			if entry.LastUsedTime >= cutoff {
				activeCount++
			}
		}
	}

	if activeCount >= maxSessions {
		return false
	}

	// Bind session
	sessions[sessionId] = sessionBindingEntry{UserId: userId, LastUsedTime: now}
	sessionToChannel[sessionId] = channelId
	return true
}

// CacheUpdateSessionBindingLastUsed updates only the last_used_time for an existing session binding.
// Only writes to DB if the binding actually exists in cache, preventing phantom DB records.
func CacheUpdateSessionBindingLastUsed(channelId int, sessionId string, userId int) {
	now := time.Now().Unix()
	bindingExists := false

	if common.RedisEnabled {
		ctx := context.Background()
		key := channelSessionBindingRedisKey(channelId)
		// Only update if the field exists
		exists, err := common.RDB.HExists(ctx, key, sessionId).Result()
		if err != nil {
			common.SysError("failed to check session binding existence in Redis: " + err.Error())
		}
		if exists {
			pipe := common.RDB.TxPipeline()
			pipe.HSet(ctx, key, sessionId, encodeSessionBindingValue(userId, now))
			pipe.Expire(ctx, key, channelSessionBindingHashTTL)
			if _, err := pipe.Exec(ctx); err != nil {
				common.SysError("failed to update session binding last used in Redis: " + err.Error())
			}
			// Refresh reverse index TTL
			channelSyncLock.RLock()
			expireMinutes := 0
			if ch, ok := channelsIDM[channelId]; ok {
				expireMinutes = ch.GetSessionBindExpireMinutes()
			}
			channelSyncLock.RUnlock()
			ttl := channelSessionBindingHashTTL
			if expireMinutes > 0 {
				ttl = time.Duration(expireMinutes) * time.Minute
			}
			common.RDB.Expire(ctx, sessionChannelRedisKey(sessionId), ttl)
			bindingExists = true
		}
	} else {
		sessionBindingCacheLock.Lock()
		if sessions, ok := channelSessionBindings[channelId]; ok {
			if _, bound := sessions[sessionId]; bound {
				sessions[sessionId] = sessionBindingEntry{UserId: userId, LastUsedTime: now}
				bindingExists = true
			}
		}
		sessionBindingCacheLock.Unlock()
	}

	// Only write to DB if binding actually exists in cache
	if bindingExists {
		CreateOrUpdateChannelSessionBindingAsync(channelId, sessionId, userId)
	}
}

// CacheUnbindSession removes a session binding from cache and DB.
func CacheUnbindSession(channelId int, sessionId string) {
	if common.RedisEnabled {
		redisUnbindSession(channelId, sessionId)
	} else {
		sessionBindingCacheLock.Lock()
		if sessions, ok := channelSessionBindings[channelId]; ok {
			delete(sessions, sessionId)
			if len(sessions) == 0 {
				delete(channelSessionBindings, channelId)
			}
		}
		delete(sessionToChannel, sessionId)
		sessionBindingCacheLock.Unlock()
	}

	gopool.Go(func() {
		if err := DeleteChannelSessionBinding(channelId, sessionId); err != nil {
			common.SysError("failed to delete channel session binding: " + err.Error())
		}
	})
}

func redisUnbindSession(channelId int, sessionId string) {
	ctx := context.Background()
	if err := common.RDB.HDel(ctx, channelSessionBindingRedisKey(channelId), sessionId).Err(); err != nil {
		common.SysError("failed to unbind session in Redis: " + err.Error())
	}
	if err := common.RDB.Del(ctx, sessionChannelRedisKey(sessionId)).Err(); err != nil {
		common.SysError("failed to delete session reverse index in Redis: " + err.Error())
	}
}

// CacheUnbindAllSessions removes all session bindings for a channel from cache and DB.
func CacheUnbindAllSessions(channelId int) {
	if common.RedisEnabled {
		ctx := context.Background()
		// Get all session IDs for this channel to clean up reverse index
		result, err := common.RDB.HGetAll(ctx, channelSessionBindingRedisKey(channelId)).Result()
		if err == nil && len(result) > 0 {
			pipe := common.RDB.Pipeline()
			for sid := range result {
				pipe.Del(ctx, sessionChannelRedisKey(sid))
			}
			pipe.Del(ctx, channelSessionBindingRedisKey(channelId))
			if _, err := pipe.Exec(ctx); err != nil {
				common.SysError("failed to unbind all sessions in Redis: " + err.Error())
			}
		} else {
			// Just try to delete the hash key
			common.RDB.Del(ctx, channelSessionBindingRedisKey(channelId))
		}
	} else {
		sessionBindingCacheLock.Lock()
		if sessions, ok := channelSessionBindings[channelId]; ok {
			for sid := range sessions {
				delete(sessionToChannel, sid)
			}
			delete(channelSessionBindings, channelId)
		}
		sessionBindingCacheLock.Unlock()
	}

	gopool.Go(func() {
		if _, err := DeleteAllChannelSessionBindings(channelId); err != nil {
			common.SysError("failed to delete all channel session bindings: " + err.Error())
		}
	})
}

// CacheGetAllActiveSessionBindingCounts returns a map of channel_id → active session binding count.
// Used for API responses.
func CacheGetAllActiveSessionBindingCounts() map[int]int {
	channelSyncLock.RLock()
	configs := make(map[int]int) // channelId → expireMinutes
	if channelsIDM != nil {
		for channelId, ch := range channelsIDM {
			configs[channelId] = ch.GetSessionBindExpireMinutes()
		}
	}
	channelSyncLock.RUnlock()

	if common.RedisEnabled {
		return redisGetAllActiveSessionBindingCounts(configs)
	}
	return memGetAllActiveSessionBindingCounts(configs)
}

func redisGetAllActiveSessionBindingCounts(configs map[int]int) map[int]int {
	result := make(map[int]int)
	channelIds := make([]int, 0, len(configs))
	for channelId := range configs {
		channelIds = append(channelIds, channelId)
	}
	if len(channelIds) == 0 {
		return result
	}

	ctx := context.Background()
	pipe := common.RDB.Pipeline()
	cmds := make([]*redis.StringStringMapCmd, len(channelIds))
	for i, channelId := range channelIds {
		cmds[i] = pipe.HGetAll(ctx, channelSessionBindingRedisKey(channelId))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("failed to get all session binding counts from Redis: " + err.Error())
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
				_, lastUsed, ok := decodeSessionBindingValue(val)
				if !ok {
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

func memGetAllActiveSessionBindingCounts(configs map[int]int) map[int]int {
	sessionBindingCacheLock.RLock()
	defer sessionBindingCacheLock.RUnlock()

	result := make(map[int]int)
	for channelId, sessions := range channelSessionBindings {
		expireMinutes := configs[channelId]
		count := 0
		if expireMinutes <= 0 {
			count = len(sessions)
		} else {
			cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
			for _, entry := range sessions {
				if entry.LastUsedTime >= cutoff {
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

// PreloadSessionBindingData fetches session binding data for multiple channels in one Redis pipeline.
// Returns channelId → sessionId → sessionBindingEntry map.
func PreloadSessionBindingData(channelIds []int) map[int]map[string]sessionBindingEntry {
	if len(channelIds) == 0 {
		return nil
	}

	if common.RedisEnabled {
		return redisPreloadSessionBindingData(channelIds)
	}
	return memPreloadSessionBindingData(channelIds)
}

func redisPreloadSessionBindingData(channelIds []int) map[int]map[string]sessionBindingEntry {
	ctx := context.Background()
	pipe := common.RDB.Pipeline()
	cmds := make([]*redis.StringStringMapCmd, len(channelIds))

	for i, channelId := range channelIds {
		cmds[i] = pipe.HGetAll(ctx, channelSessionBindingRedisKey(channelId))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("failed to preload session binding data from Redis: " + err.Error())
		return make(map[int]map[string]sessionBindingEntry)
	}

	result := make(map[int]map[string]sessionBindingEntry, len(channelIds))
	for i, channelId := range channelIds {
		data, err := cmds[i].Result()
		if err != nil || len(data) == 0 {
			continue
		}
		sessions := make(map[string]sessionBindingEntry, len(data))
		for sid, val := range data {
			userId, lastUsed, ok := decodeSessionBindingValue(val)
			if !ok {
				continue
			}
			sessions[sid] = sessionBindingEntry{UserId: userId, LastUsedTime: lastUsed}
		}
		if len(sessions) > 0 {
			result[channelId] = sessions
		}
	}
	return result
}

func memPreloadSessionBindingData(channelIds []int) map[int]map[string]sessionBindingEntry {
	sessionBindingCacheLock.RLock()
	defer sessionBindingCacheLock.RUnlock()

	result := make(map[int]map[string]sessionBindingEntry, len(channelIds))
	for _, channelId := range channelIds {
		if sessions, ok := channelSessionBindings[channelId]; ok {
			sessionsCopy := make(map[string]sessionBindingEntry, len(sessions))
			for k, v := range sessions {
				sessionsCopy[k] = v
			}
			result[channelId] = sessionsCopy
		}
	}
	return result
}

// isSessionBoundFromData checks session binding from preloaded data.
func isSessionBoundFromData(data map[int]map[string]sessionBindingEntry, channelId int, sessionId string, expireMinutes int) bool {
	sessions, ok := data[channelId]
	if !ok {
		return false
	}
	entry, bound := sessions[sessionId]
	if !bound {
		return false
	}
	if expireMinutes <= 0 {
		return true
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	return entry.LastUsedTime >= cutoff
}

// getActiveSessionCountFromData counts active session bindings from preloaded data.
func getActiveSessionCountFromData(data map[int]map[string]sessionBindingEntry, channelId int, expireMinutes int) int {
	sessions, ok := data[channelId]
	if !ok {
		return 0
	}
	if expireMinutes <= 0 {
		return len(sessions)
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	count := 0
	for _, entry := range sessions {
		if entry.LastUsedTime >= cutoff {
			count++
		}
	}
	return count
}
