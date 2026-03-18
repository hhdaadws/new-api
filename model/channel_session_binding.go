package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm/clause"
)

type ChannelSessionBinding struct {
	ChannelId    int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	SessionId    string `json:"session_id" gorm:"primaryKey;autoIncrement:false;type:varchar(128);index"`
	UserId       int    `json:"user_id" gorm:"index"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	LastUsedTime int64  `json:"last_used_time" gorm:"bigint"`
}

type sessionBindingEntry struct {
	UserId       int
	LastUsedTime int64
}

func CreateOrUpdateChannelSessionBinding(channelId int, sessionId string, userId int) error {
	now := time.Now().Unix()
	binding := ChannelSessionBinding{
		ChannelId:    channelId,
		SessionId:    sessionId,
		UserId:       userId,
		CreatedTime:  now,
		LastUsedTime: now,
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}, {Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_used_time"}),
	}).Create(&binding).Error
}

func CreateOrUpdateChannelSessionBindingAsync(channelId int, sessionId string, userId int) {
	gopool.Go(func() {
		if err := CreateOrUpdateChannelSessionBinding(channelId, sessionId, userId); err != nil {
			common.SysError("failed to create channel session binding: " + err.Error())
		}
	})
}

func GetChannelSessionBindings(channelId int) ([]ChannelSessionBinding, error) {
	var bindings []ChannelSessionBinding
	err := DB.Where("channel_id = ?", channelId).Order("created_time ASC").Find(&bindings).Error
	return bindings, err
}

func DeleteChannelSessionBinding(channelId int, sessionId string) error {
	return DB.Where("channel_id = ? AND session_id = ?", channelId, sessionId).Delete(&ChannelSessionBinding{}).Error
}

func DeleteAllChannelSessionBindings(channelId int) (int64, error) {
	result := DB.Where("channel_id = ?", channelId).Delete(&ChannelSessionBinding{})
	return result.RowsAffected, result.Error
}

func DeleteSessionBindingsByChannelIds(channelIds []int) error {
	if len(channelIds) == 0 {
		return nil
	}
	return DB.Where("channel_id IN ?", channelIds).Delete(&ChannelSessionBinding{}).Error
}

func GetAllSessionBindingsMap() (map[int]map[string]sessionBindingEntry, error) {
	var bindings []ChannelSessionBinding
	err := DB.Find(&bindings).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int]map[string]sessionBindingEntry)
	for _, b := range bindings {
		if _, ok := result[b.ChannelId]; !ok {
			result[b.ChannelId] = make(map[string]sessionBindingEntry)
		}
		result[b.ChannelId][b.SessionId] = sessionBindingEntry{
			UserId:       b.UserId,
			LastUsedTime: b.LastUsedTime,
		}
	}
	return result, nil
}

func CleanExpiredSessionBindings(channelId int, expireMinutes int) (int64, error) {
	if expireMinutes <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute).Unix()
	result := DB.Where("channel_id = ? AND last_used_time < ?", channelId, cutoff).Delete(&ChannelSessionBinding{})
	return result.RowsAffected, result.Error
}
