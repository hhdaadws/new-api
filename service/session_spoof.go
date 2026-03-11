package service

import (
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
