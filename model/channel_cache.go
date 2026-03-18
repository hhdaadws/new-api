package model

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var channelSyncLock sync.RWMutex

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
	}
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := strings.Split(channel.Models, ",")
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
	InitChannelUserBindingCache()
	InitSessionBindingCache()
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, retry int, userId int, sessionId string) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannel(group, model, retry, userId)
	}

	// Phase 1: Read channel data under channelSyncLock
	var targetChannels []*Channel
	var singleChannel *Channel
	var channelErr error

	channelSyncLock.RLock()

	// First, try to find channels with the exact model name.
	channels := group2model2channels[group][model]

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = group2model2channels[group][normalizedModel]
	}

	if len(channels) == 0 {
		channelSyncLock.RUnlock()
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			singleChannel = channel
		} else {
			channelErr = fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
		}
		channelSyncLock.RUnlock()

		if channelErr != nil {
			return nil, channelErr
		}

		// Check user limit for single channel (outside lock, safe for Redis I/O)
		if userId > 0 {
			maxUsers := singleChannel.GetMaxUsers()
			if maxUsers > 0 {
				expireMinutes := singleChannel.GetUserBindExpireMinutes()
				if !CacheBindUserIfRoom(singleChannel.Id, userId, maxUsers, expireMinutes) {
					return nil, nil
				}
			}
		}
		// Check session limit for single channel
		if sessionId != "" {
			maxSessions := singleChannel.GetMaxSessions()
			if maxSessions > 0 {
				if !CacheBindSessionIfRoom(singleChannel.Id, sessionId, userId, maxSessions, singleChannel.GetSessionBindExpireMinutes()) {
					return nil, nil
				}
			}
		}
		return singleChannel, nil
	}

	// Multiple channels: filter by priority
	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			channelSyncLock.RUnlock()
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	startPri := retry
	if startPri >= len(sortedUniquePriorities) {
		startPri = len(sortedUniquePriorities) - 1
	}

	// Collect all channels grouped by priority under the read lock,
	// then release the lock before doing binding checks (Redis I/O).
	allPriorityChannels := make([][]*Channel, len(sortedUniquePriorities))
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			for idx, p := range sortedUniquePriorities {
				if channel.GetPriority() == int64(p) {
					allPriorityChannels[idx] = append(allPriorityChannels[idx], channel)
					break
				}
			}
		} else {
			channelSyncLock.RUnlock()
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	// Release channelSyncLock before doing binding checks (Redis I/O)
	// Channel pointers remain valid because channelsIDM sync creates new objects
	channelSyncLock.RUnlock()

	// Build flat channel map for override lookups
	allChannelsFlat := make(map[int]*Channel)
	for _, priorityChannels := range allPriorityChannels {
		for _, ch := range priorityChannels {
			allChannelsFlat[ch.Id] = ch
		}
	}

	// Before priority-based selection: if session is already bound to any candidate channel,
	// route directly to that channel (ignoring priority).
	if sessionId != "" {
		if boundChannelId, found := CacheGetSessionChannel(sessionId); found {
			if ch, ok := allChannelsFlat[boundChannelId]; ok {
				// Session appears bound to a candidate — atomically verify and refresh
				if CacheBindSessionIfRoom(ch.Id, sessionId, userId, ch.GetMaxSessions(), ch.GetSessionBindExpireMinutes()) {
					return ch, nil
				}
				// Binding expired or channel full, fall through
			}
		}
	}

	// Before priority-based selection: if user is already bound to any candidate channel,
	// route directly to that channel (ignoring priority), unless it's disabled.
	// Disabled channels are excluded from group2model2channels, so being in the candidate
	// list already implies the channel is enabled.
	var bindingData map[int]map[int]int64
	if userId > 0 {
		// Collect all channels with maxUsers > 0 across ALL priorities
		allBindingChannelIds := make([]int, 0)
		for _, priorityChannels := range allPriorityChannels {
			for _, ch := range priorityChannels {
				if ch.GetMaxUsers() > 0 {
					allBindingChannelIds = append(allBindingChannelIds, ch.Id)
				}
			}
		}
		if len(allBindingChannelIds) > 0 {
			bindingData = PreloadBindingData(allBindingChannelIds)
			// Check if user is bound to any candidate channel
			for _, chId := range allBindingChannelIds {
				ch := allChannelsFlat[chId]
				expireMinutes := ch.GetUserBindExpireMinutes()
				if isUserBoundFromData(bindingData, chId, userId, expireMinutes) {
					// User appears bound — atomically verify and refresh binding
					if CacheBindUserIfRoom(chId, userId, ch.GetMaxUsers(), expireMinutes) {
						return ch, nil
					}
					// Binding expired between preload and atomic check, or channel is full.
					// Fall through to normal priority-based selection.
					break
				}
			}
		}
	}
	// Try each priority level starting from startPri.
	// If all channels in a priority are filtered out by user/session binding limits,
	// fall back to the next (lower) priority level.
	// Note: bindingData may already be preloaded from the binding override check above.
	var sessionBindingData map[int]map[string]sessionBindingEntry
	for pri := startPri; pri < len(sortedUniquePriorities); pri++ {
		targetChannels = allPriorityChannels[pri]

		if len(targetChannels) == 0 {
			continue
		}

		// Phase 2: Filter by user bindings (no channelSyncLock, safe for Redis I/O)
		// Preload binding data if not already loaded from binding override check
		if bindingData == nil && userId > 0 {
			channelIds := make([]int, 0, len(targetChannels))
			for _, ch := range targetChannels {
				if ch.GetMaxUsers() > 0 {
					channelIds = append(channelIds, ch.Id)
				}
			}
			if len(channelIds) > 0 {
				bindingData = PreloadBindingData(channelIds)
			}
		}

		// Filter by user limit: remove channels that are full and user is not already bound
		if userId > 0 {
			targetChannels = filterChannelsByUserLimit(targetChannels, userId, bindingData)
		}

		if len(targetChannels) == 0 {
			continue
		}

		// Preload session binding data for channels with max_sessions > 0
		if sessionBindingData == nil && sessionId != "" {
			sessionChannelIds := make([]int, 0, len(targetChannels))
			for _, ch := range targetChannels {
				if ch.GetMaxSessions() > 0 {
					sessionChannelIds = append(sessionChannelIds, ch.Id)
				}
			}
			if len(sessionChannelIds) > 0 {
				sessionBindingData = PreloadSessionBindingData(sessionChannelIds)
			}
		}

		// Filter by session limit: remove channels that are full and session is not already bound
		if sessionId != "" {
			targetChannels = filterChannelsBySessionLimit(targetChannels, sessionId, sessionBindingData)
		}

		if len(targetChannels) == 0 {
			continue
		}

		// Among channels with max_users > 0, apply least-bindings load balancing
		targetChannels = applyLeastBindingsBalance(targetChannels, bindingData)

		// Among channels with max_sessions > 0, apply least-session-bindings load balancing
		targetChannels = applyLeastSessionBindingsBalance(targetChannels, sessionBindingData)

		// Atomic bind retry loop: select a channel and try to bind.
		// If bind fails (channel became full due to concurrent bind), remove and retry.
		for len(targetChannels) > 0 {
			selected := weightedRandomSelect(targetChannels)
			if selected == nil {
				break
			}

			// Create user binding if max_users is enabled (atomic check-and-bind)
			if userId > 0 && selected.GetMaxUsers() > 0 {
				if !CacheBindUserIfRoom(selected.Id, userId, selected.GetMaxUsers(), selected.GetUserBindExpireMinutes()) {
					// Bind failed: channel is full. Remove from candidates and retry.
					targetChannels = removeChannel(targetChannels, selected.Id)
					continue
				}
			}

			// Create session binding if max_sessions is enabled (atomic check-and-bind)
			if sessionId != "" && selected.GetMaxSessions() > 0 {
				if !CacheBindSessionIfRoom(selected.Id, sessionId, userId, selected.GetMaxSessions(), selected.GetSessionBindExpireMinutes()) {
					// Bind failed: channel is full. Remove from candidates and retry.
					targetChannels = removeChannel(targetChannels, selected.Id)
					continue
				}
			}

			return selected, nil
		}
		// All candidates at this priority exhausted, try next priority level
	}

	return nil, nil
}

// filterChannelsByUserLimit removes channels that have reached their user limit
// for a user who is not already bound to them.
// If the user is already bound to one of the candidate channels, only that channel
// (among those with user limits) is kept, preventing a user from binding to multiple channels.
// bindingData is preloaded binding data from PreloadBindingData (may be nil if no channels have user limits).
func filterChannelsByUserLimit(channels []*Channel, userId int, bindingData map[int]map[int]int64) []*Channel {
	// First pass: check if user is already bound to any candidate channel
	var boundChannel *Channel
	for _, ch := range channels {
		if ch.GetMaxUsers() <= 0 {
			continue
		}
		expireMinutes := ch.GetUserBindExpireMinutes()
		if isUserBoundFromData(bindingData, ch.Id, userId, expireMinutes) {
			boundChannel = ch
			break
		}
	}

	// Second pass: filter channels
	var available []*Channel
	for _, ch := range channels {
		maxUsers := ch.GetMaxUsers()
		if maxUsers <= 0 {
			// No user limit, always available
			available = append(available, ch)
			continue
		}
		if boundChannel != nil {
			// User is already bound to a channel: only keep the bound one
			if ch.Id == boundChannel.Id {
				available = append(available, ch)
			}
			// Skip other channels with user limits
			continue
		}
		// User has no existing binding: check capacity
		expireMinutes := ch.GetUserBindExpireMinutes()
		if getActiveCountFromData(bindingData, ch.Id, expireMinutes) < maxUsers {
			available = append(available, ch)
		}
	}
	return available
}

// applyLeastBindingsBalance filters channels to prefer those with the fewest active bindings.
// Among channels that have max_users > 0, keeps only those with the minimum binding count.
// Channels without user limits (max_users == 0) are always included.
// bindingData is preloaded binding data from PreloadBindingData (may be nil if no channels have user limits).
func applyLeastBindingsBalance(channels []*Channel, bindingData map[int]map[int]int64) []*Channel {
	if len(channels) <= 1 {
		return channels
	}

	// Separate channels with and without user limits
	var withLimit []*Channel
	var withoutLimit []*Channel
	for _, ch := range channels {
		if ch.GetMaxUsers() > 0 {
			withLimit = append(withLimit, ch)
		} else {
			withoutLimit = append(withoutLimit, ch)
		}
	}

	// If no channels have user limits, return all
	if len(withLimit) == 0 {
		return channels
	}

	// If there are channels without limits, include them all + all with limits
	// (no need to load balance, unlimited channels handle overflow)
	if len(withoutLimit) > 0 {
		return channels
	}

	// All channels have user limits: find minimum binding count, keep only those
	minCount := -1
	type channelWithCount struct {
		ch    *Channel
		count int
	}
	var withCounts []channelWithCount
	for _, ch := range withLimit {
		count := getActiveCountFromData(bindingData, ch.Id, ch.GetUserBindExpireMinutes())
		withCounts = append(withCounts, channelWithCount{ch, count})
		if minCount < 0 || count < minCount {
			minCount = count
		}
	}

	var result []*Channel
	for _, wc := range withCounts {
		if wc.count == minCount {
			result = append(result, wc.ch)
		}
	}
	return result
}

// filterChannelsBySessionLimit removes channels that have reached their session limit
// for a session that is not already bound to them.
// If the session is already bound to one of the candidate channels, only that channel
// (among those with session limits) is kept.
func filterChannelsBySessionLimit(channels []*Channel, sessionId string, sessionBindingData map[int]map[string]sessionBindingEntry) []*Channel {
	// First pass: check if session is already bound to any candidate channel
	var boundChannel *Channel
	for _, ch := range channels {
		if ch.GetMaxSessions() <= 0 {
			continue
		}
		expireMinutes := ch.GetSessionBindExpireMinutes()
		if isSessionBoundFromData(sessionBindingData, ch.Id, sessionId, expireMinutes) {
			boundChannel = ch
			break
		}
	}

	// Second pass: filter channels
	var available []*Channel
	for _, ch := range channels {
		maxSessions := ch.GetMaxSessions()
		if maxSessions <= 0 {
			// No session limit, always available
			available = append(available, ch)
			continue
		}
		if boundChannel != nil {
			// Session is already bound to a channel: only keep the bound one
			if ch.Id == boundChannel.Id {
				available = append(available, ch)
			}
			continue
		}
		// Session has no existing binding: check capacity
		expireMinutes := ch.GetSessionBindExpireMinutes()
		if getActiveSessionCountFromData(sessionBindingData, ch.Id, expireMinutes) < maxSessions {
			available = append(available, ch)
		}
	}
	return available
}

// applyLeastSessionBindingsBalance filters channels to prefer those with the fewest active session bindings.
// Among channels that have max_sessions > 0, keeps only those with the minimum session binding count.
// Channels without session limits (max_sessions == 0) are always included.
func applyLeastSessionBindingsBalance(channels []*Channel, sessionBindingData map[int]map[string]sessionBindingEntry) []*Channel {
	if len(channels) <= 1 {
		return channels
	}

	var withLimit []*Channel
	var withoutLimit []*Channel
	for _, ch := range channels {
		if ch.GetMaxSessions() > 0 {
			withLimit = append(withLimit, ch)
		} else {
			withoutLimit = append(withoutLimit, ch)
		}
	}

	if len(withLimit) == 0 {
		return channels
	}
	if len(withoutLimit) > 0 {
		return channels
	}

	// All channels have session limits: find minimum binding count, keep only those
	minCount := -1
	type channelWithCount struct {
		ch    *Channel
		count int
	}
	var withCounts []channelWithCount
	for _, ch := range withLimit {
		count := getActiveSessionCountFromData(sessionBindingData, ch.Id, ch.GetSessionBindExpireMinutes())
		withCounts = append(withCounts, channelWithCount{ch, count})
		if minCount < 0 || count < minCount {
			minCount = count
		}
	}

	var result []*Channel
	for _, wc := range withCounts {
		if wc.count == minCount {
			result = append(result, wc.ch)
		}
	}
	return result
}

// removeChannel returns a new slice with the channel of the given ID removed.
func removeChannel(channels []*Channel, id int) []*Channel {
	result := make([]*Channel, 0, len(channels)-1)
	for _, ch := range channels {
		if ch.Id != id {
			result = append(result, ch)
		}
	}
	return result
}

// weightedRandomSelect selects a channel using weight-based random selection.
func weightedRandomSelect(targetChannels []*Channel) *Channel {
	if len(targetChannels) == 0 {
		return nil
	}
	if len(targetChannels) == 1 {
		return targetChannels[0]
	}

	sumWeight := 0
	for _, ch := range targetChannels {
		sumWeight += ch.GetWeight()
	}

	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		smoothingFactor = 100
	}

	totalWeight := sumWeight * smoothingFactor
	randomWeight := rand.Intn(totalWeight)

	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel
		}
	}
	return nil
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	println("CacheUpdateChannel:", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)

	println("before:", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
	channelsIDM[channel.Id] = channel
	println("after :", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
}
