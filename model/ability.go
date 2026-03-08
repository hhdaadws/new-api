package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType int `json:"channel_type"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	return abilities, err
}

func GetGroupEnabledModels(group string) []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where(commonGroupCol+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &models)
	return models
}

func GetEnabledModels() []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

// getAllPriorities returns all distinct priority levels for the given group/model,
// sorted in descending order (highest priority first).
func getAllPriorities(group string, model string) ([]int, error) {
	var priorities []int
	err := DB.Model(&Ability{}).
		Select("DISTINCT(priority)").
		Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Order("priority DESC").
		Pluck("priority", &priorities).Error
	if err != nil {
		return nil, err
	}
	return priorities, nil
}

func getPriority(group string, model string, retry int) (int, error) {
	priorities, err := getAllPriorities(group, model)
	if err != nil {
		return 0, err
	}

	if len(priorities) == 0 {
		// 如果没有查询到优先级，则返回错误
		return 0, errors.New("数据库一致性被破坏")
	}

	// 确定要使用的优先级
	var priorityToUse int
	if retry >= len(priorities) {
		// 如果重试次数大于优先级数，则使用最小的优先级
		priorityToUse = priorities[len(priorities)-1]
	} else {
		priorityToUse = priorities[retry]
	}
	return priorityToUse, nil
}

func getChannelQuery(group string, model string, retry int) (*gorm.DB, error) {
	maxPrioritySubQuery := DB.Model(&Ability{}).Select("MAX(priority)").Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true)
	channelQuery := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = (?)", group, model, true, maxPrioritySubQuery)
	if retry != 0 {
		priority, err := getPriority(group, model, retry)
		if err != nil {
			return nil, err
		} else {
			channelQuery = DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = ?", group, model, true, priority)
		}
	}

	return channelQuery, nil
}

func GetChannel(group string, model string, retry int, userId int) (*Channel, error) {
	var abilities []Ability

	// Get all priority levels to support fallback when user binding limits filter out all channels
	priorities, err := getAllPriorities(group, model)
	if err != nil || len(priorities) == 0 {
		return nil, err
	}

	startPri := retry
	if startPri >= len(priorities) {
		startPri = len(priorities) - 1
	}

	// Try each priority level starting from startPri.
	// If all channels in a priority are filtered out by user binding limits,
	// fall back to the next (lower) priority level.
	for pri := startPri; pri < len(priorities); pri++ {
		priorityToUse := priorities[pri]
		channelQuery := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = ?", group, model, true, priorityToUse)
		abilities = nil
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
		if err != nil {
			return nil, err
		}
		if len(abilities) == 0 {
			continue
		}

		// Filter by user limit if userId is provided
		if userId > 0 {
			abilities = filterAbilitiesByUserLimit(abilities, userId)
		}
		if len(abilities) > 0 {
			break
		}
		// All channels at this priority are full, try next priority level
	}

	if len(abilities) == 0 {
		return nil, nil
	}

	// Randomly choose one
	channel := Channel{}
	weightSum := uint(0)
	for _, ability_ := range abilities {
		weightSum += ability_.Weight + 10
	}
	weight := common.GetRandomInt(int(weightSum))
	for _, ability_ := range abilities {
		weight -= int(ability_.Weight) + 10
		if weight <= 0 {
			channel.Id = ability_.ChannelId
			break
		}
	}

	err = DB.First(&channel, "id = ?", channel.Id).Error
	if err != nil {
		return nil, err
	}

	// Create binding if max_users is enabled
	if userId > 0 && channel.GetMaxUsers() > 0 {
		go CacheBindUser(channel.Id, userId)
	}

	return &channel, nil
}

// filterAbilitiesByUserLimit filters abilities to exclude channels that have reached their user limit.
// If the user is already bound to one of the candidate channels, only that channel
// (among those with user limits) is kept, preventing a user from binding to multiple channels.
func filterAbilitiesByUserLimit(abilities []Ability, userId int) []Ability {
	// First pass: load channel configs and check if user is already bound to any candidate
	type channelInfo struct {
		maxUsers      int
		expireMinutes int
	}
	channelConfigs := make(map[int]channelInfo)
	boundChannelId := 0

	for _, ability := range abilities {
		var ch Channel
		if err := DB.Select("id, max_users, user_bind_expire_minutes").First(&ch, "id = ?", ability.ChannelId).Error; err != nil {
			continue
		}
		maxUsers := ch.GetMaxUsers()
		expireMinutes := ch.GetUserBindExpireMinutes()
		channelConfigs[ability.ChannelId] = channelInfo{maxUsers: maxUsers, expireMinutes: expireMinutes}

		if maxUsers <= 0 || boundChannelId > 0 {
			continue
		}
		// Check if user is already bound to this channel
		var existingBinding ChannelUserBinding
		err := DB.Where("channel_id = ? AND user_id = ?", ability.ChannelId, userId).First(&existingBinding).Error
		if err == nil {
			if expireMinutes <= 0 || existingBinding.LastUsedTime >= time.Now().Add(-time.Duration(expireMinutes)*time.Minute).Unix() {
				boundChannelId = ability.ChannelId
			}
		}
	}

	// Second pass: filter
	var filtered []Ability
	for _, ability := range abilities {
		cfg, ok := channelConfigs[ability.ChannelId]
		if !ok {
			continue
		}
		if cfg.maxUsers <= 0 {
			// No user limit, always available
			filtered = append(filtered, ability)
			continue
		}
		if boundChannelId > 0 {
			// User is already bound: only keep the bound channel
			if ability.ChannelId == boundChannelId {
				filtered = append(filtered, ability)
			}
			continue
		}
		// User has no existing binding: check capacity
		var count int64
		if cfg.expireMinutes <= 0 {
			DB.Model(&ChannelUserBinding{}).Where("channel_id = ?", ability.ChannelId).Count(&count)
		} else {
			cutoff := time.Now().Add(-time.Duration(cfg.expireMinutes) * time.Minute).Unix()
			DB.Model(&ChannelUserBinding{}).Where("channel_id = ? AND last_used_time >= ?", ability.ChannelId, cutoff).Count(&count)
		}
		if int(count) < cfg.maxUsers {
			filtered = append(filtered, ability)
		}
	}
	return filtered
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingSQLite {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
