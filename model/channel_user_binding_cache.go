package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
)

// channelUserBindings stores channel_id → user_id → last_used_time (unix seconds)
// Protected by channelSyncLock (defined in channel_cache.go)
var channelUserBindings map[int]map[int]int64

func InitChannelUserBindingCache() {
	// DB query outside lock to avoid holding lock during I/O
	bindingsMap, err := GetAllBindingsMap()
	if err != nil {
		common.SysError("failed to load channel user bindings: " + err.Error())
		bindingsMap = make(map[int]map[int]int64)
	}

	// Acquire lock before reading channelsIDM and writing channelUserBindings
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()

	// Clean expired bindings during init
	if channelsIDM != nil {
		for channelId, ch := range channelsIDM {
			expireMinutes := ch.GetUserBindExpireMinutes()
			if expireMinutes <= 0 {
				continue
			}
			cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
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
			expMin := expireMinutes
			gopool.Go(func() {
				_, _ = CleanExpiredBindings(chId, expMin)
			})
		}
	}

	channelUserBindings = bindingsMap
	common.SysLog("channel user bindings cache synced from database")
}

// CacheGetActiveBindingCount returns the number of non-expired bindings for a channel.
// Must be called with channelSyncLock held (at least RLock).
func CacheGetActiveBindingCount(channelId int, expireMinutes int) int {
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
// Acquires its own lock - safe to call from controllers.
func CacheGetSingleActiveBindingCount(channelId int) int {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	expireMinutes := 0
	if ch, ok := channelsIDM[channelId]; ok {
		expireMinutes = ch.GetUserBindExpireMinutes()
	}
	return CacheGetActiveBindingCount(channelId, expireMinutes)
}

// CacheIsUserBound checks if a user has an active (non-expired) binding to a channel.
// Must be called with channelSyncLock held (at least RLock).
func CacheIsUserBound(channelId, userId int, expireMinutes int) bool {
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

// CacheBindUser adds or updates a user-channel binding in memory and writes to DB asynchronously.
func CacheBindUser(channelId, userId int) {
	now := time.Now().Unix()

	channelSyncLock.Lock()
	if channelUserBindings == nil {
		channelUserBindings = make(map[int]map[int]int64)
	}
	if _, ok := channelUserBindings[channelId]; !ok {
		channelUserBindings[channelId] = make(map[int]int64)
	}
	channelUserBindings[channelId][userId] = now
	channelSyncLock.Unlock()

	CreateOrUpdateChannelUserBindingAsync(channelId, userId)
}

// CacheUpdateBindingLastUsed updates only the last_used_time for an existing binding.
func CacheUpdateBindingLastUsed(channelId, userId int) {
	now := time.Now().Unix()

	channelSyncLock.Lock()
	if users, ok := channelUserBindings[channelId]; ok {
		if _, bound := users[userId]; bound {
			users[userId] = now
		}
	}
	channelSyncLock.Unlock()

	// Async DB update
	CreateOrUpdateChannelUserBindingAsync(channelId, userId)
}

// CacheUnbindUser removes a user-channel binding from memory and DB.
func CacheUnbindUser(channelId, userId int) {
	channelSyncLock.Lock()
	if users, ok := channelUserBindings[channelId]; ok {
		delete(users, userId)
		if len(users) == 0 {
			delete(channelUserBindings, channelId)
		}
	}
	channelSyncLock.Unlock()

	gopool.Go(func() {
		if err := DeleteChannelUserBinding(channelId, userId); err != nil {
			common.SysError("failed to delete channel user binding: " + err.Error())
		}
	})
}

// CacheUnbindAllUsers removes all user bindings for a channel from memory and DB.
func CacheUnbindAllUsers(channelId int) {
	channelSyncLock.Lock()
	delete(channelUserBindings, channelId)
	channelSyncLock.Unlock()

	gopool.Go(func() {
		if _, err := DeleteAllChannelUserBindings(channelId); err != nil {
			common.SysError("failed to delete all channel user bindings: " + err.Error())
		}
	})
}

// CacheGetAllActiveBindingCounts returns a map of channel_id → active binding count.
// Used for API responses. Acquires its own lock.
func CacheGetAllActiveBindingCounts() map[int]int {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	result := make(map[int]int)
	for channelId, users := range channelUserBindings {
		expireMinutes := 0
		if ch, ok := channelsIDM[channelId]; ok {
			expireMinutes = ch.GetUserBindExpireMinutes()
		}
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
