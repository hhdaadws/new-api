package service

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
)

const (
	sessionSpoofMinTTL         = 3600 // 1 hour in seconds
	sessionSpoofMaxTTL         = 7200 // 2 hours in seconds
	sessionSpoofRedisKeyPrefix = "session_spoof:"
)

type spoofEntry struct {
	sessionId string
	expiresAt int64
}

var spoofCache sync.Map // channelId -> *spoofEntry

// GetSpoofSessionId returns the current fake session ID for a channel,
// generating a new one if expired or not yet created.
func GetSpoofSessionId(channelId int) string {
	if common.RedisEnabled {
		return redisSpoofSessionId(channelId)
	}
	return memSpoofSessionId(channelId)
}

func generateSpoofSessionId() string {
	return "session_" + uuid.New().String()
}

func randomSpoofTTLSeconds() int {
	return sessionSpoofMinTTL + rand.Intn(sessionSpoofMaxTTL-sessionSpoofMinTTL+1)
}

func redisSpoofSessionId(channelId int) string {
	key := fmt.Sprintf("%s%d", sessionSpoofRedisKeyPrefix, channelId)
	val, err := common.RedisGet(key)
	if err == nil && val != "" {
		return val
	}
	newId := generateSpoofSessionId()
	ttl := time.Duration(randomSpoofTTLSeconds()) * time.Second
	_ = common.RedisSet(key, newId, ttl)
	return newId
}

func memSpoofSessionId(channelId int) string {
	now := time.Now().Unix()
	if v, ok := spoofCache.Load(channelId); ok {
		entry := v.(*spoofEntry)
		if entry.expiresAt > now {
			return entry.sessionId
		}
	}
	newId := generateSpoofSessionId()
	spoofCache.Store(channelId, &spoofEntry{
		sessionId: newId,
		expiresAt: now + int64(randomSpoofTTLSeconds()),
	})
	return newId
}

// GetSpoofSessionInfo returns the current spoofed session ID and remaining TTL
// for a channel without generating a new one if it doesn't exist.
func GetSpoofSessionInfo(channelId int) (sessionId string, ttlSeconds int64) {
	if common.RedisEnabled {
		return redisSpoofSessionInfo(channelId)
	}
	return memSpoofSessionInfo(channelId)
}

func redisSpoofSessionInfo(channelId int) (string, int64) {
	key := fmt.Sprintf("%s%d", sessionSpoofRedisKeyPrefix, channelId)
	val, err := common.RedisGet(key)
	if err != nil || val == "" {
		return "", 0
	}
	ttl, err := common.RDB.TTL(context.Background(), key).Result()
	if err != nil || ttl <= 0 {
		return val, 0
	}
	return val, int64(ttl.Seconds())
}

func memSpoofSessionInfo(channelId int) (string, int64) {
	now := time.Now().Unix()
	if v, ok := spoofCache.Load(channelId); ok {
		entry := v.(*spoofEntry)
		if entry.expiresAt > now {
			return entry.sessionId, entry.expiresAt - now
		}
	}
	return "", 0
}
