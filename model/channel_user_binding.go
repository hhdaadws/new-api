package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm/clause"
)

type ChannelUserBinding struct {
	ChannelId    int   `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	UserId       int   `json:"user_id" gorm:"primaryKey;autoIncrement:false;index"`
	CreatedTime  int64 `json:"created_time" gorm:"bigint"`
	LastUsedTime int64 `json:"last_used_time" gorm:"bigint"`
}

type ChannelUserBindingInfo struct {
	ChannelUserBinding
	Username string `json:"username"`
}

func CreateOrUpdateChannelUserBinding(channelId, userId int) error {
	now := time.Now().Unix()
	binding := ChannelUserBinding{
		ChannelId:    channelId,
		UserId:       userId,
		CreatedTime:  now,
		LastUsedTime: now,
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_used_time"}),
	}).Create(&binding).Error
}

func CreateOrUpdateChannelUserBindingAsync(channelId, userId int) {
	gopool.Go(func() {
		if err := CreateOrUpdateChannelUserBinding(channelId, userId); err != nil {
			common.SysError("failed to create channel user binding: " + err.Error())
		}
	})
}

func GetChannelUserBindings(channelId int) ([]ChannelUserBinding, error) {
	var bindings []ChannelUserBinding
	err := DB.Where("channel_id = ?", channelId).Order("created_time ASC").Find(&bindings).Error
	return bindings, err
}

func GetChannelUserBindingsWithUsername(channelId int) ([]ChannelUserBindingInfo, error) {
	var results []ChannelUserBindingInfo
	err := DB.Table("channel_user_bindings").
		Select("channel_user_bindings.*, users.username").
		Joins("LEFT JOIN users ON channel_user_bindings.user_id = users.id").
		Where("channel_user_bindings.channel_id = ?", channelId).
		Order("channel_user_bindings.created_time ASC").
		Scan(&results).Error
	return results, err
}

func DeleteChannelUserBinding(channelId, userId int) error {
	return DB.Where("channel_id = ? AND user_id = ?", channelId, userId).Delete(&ChannelUserBinding{}).Error
}

func DeleteAllChannelUserBindings(channelId int) (int64, error) {
	result := DB.Where("channel_id = ?", channelId).Delete(&ChannelUserBinding{})
	return result.RowsAffected, result.Error
}

func DeleteBindingsByChannelIds(channelIds []int) error {
	if len(channelIds) == 0 {
		return nil
	}
	return DB.Where("channel_id IN ?", channelIds).Delete(&ChannelUserBinding{}).Error
}

func GetAllBindingsMap() (map[int]map[int]int64, error) {
	var bindings []ChannelUserBinding
	err := DB.Find(&bindings).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int]map[int]int64)
	for _, b := range bindings {
		if _, ok := result[b.ChannelId]; !ok {
			result[b.ChannelId] = make(map[int]int64)
		}
		result[b.ChannelId][b.UserId] = b.LastUsedTime
	}
	return result, nil
}

func CleanExpiredBindings(channelId int, expireMinutes int) (int64, error) {
	if expireMinutes <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	result := DB.Where("channel_id = ? AND last_used_time < ?", channelId, cutoff).Delete(&ChannelUserBinding{})
	return result.RowsAffected, result.Error
}
