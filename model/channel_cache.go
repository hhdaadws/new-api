package model

import (
	"errors"
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
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, retry int, userId int) (*Channel, error) {
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
				if !CacheIsUserBound(singleChannel.Id, userId, expireMinutes) &&
					CacheGetActiveBindingCount(singleChannel.Id, expireMinutes) >= maxUsers {
					return nil, nil
				}
				go CacheBindUser(singleChannel.Id, userId)
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

	// Try each priority level starting from startPri.
	// If all channels in a priority are filtered out by user binding limits,
	// fall back to the next (lower) priority level.
	var bindingData map[int]map[int]int64
	for pri := startPri; pri < len(sortedUniquePriorities); pri++ {
		targetChannels = allPriorityChannels[pri]

		if len(targetChannels) == 0 {
			continue
		}

		// Phase 2: Filter by user bindings (no channelSyncLock, safe for Redis I/O)
		// Preload binding data for all candidate channels in one pipeline
		bindingData = nil
		if userId > 0 {
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

		if len(targetChannels) > 0 {
			break
		}
		// All channels at this priority are full, try next priority level
	}

	if len(targetChannels) == 0 {
		return nil, nil
	}

	// Among channels with max_users > 0, apply least-bindings load balancing
	targetChannels = applyLeastBindingsBalance(targetChannels, bindingData)

	selected := weightedRandomSelect(targetChannels)
	if selected == nil {
		return nil, errors.New("channel not found")
	}

	// Create binding if max_users is enabled
	if userId > 0 && selected.GetMaxUsers() > 0 {
		go CacheBindUser(selected.Id, userId)
	}

	return selected, nil
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
