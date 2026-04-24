package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type ImageGeneration struct {
	ID            int64  `json:"id" gorm:"primaryKey;AUTO_INCREMENT"`
	CreatedAt     int64  `json:"created_at" gorm:"index"`
	UserId        int    `json:"user_id" gorm:"index"`
	Kind          string `json:"kind" gorm:"type:varchar(24);index"`
	Model         string `json:"model" gorm:"type:varchar(191);index"`
	Group         string `json:"group" gorm:"type:varchar(64);index"`
	Prompt        string `json:"prompt" gorm:"type:text"`
	Size          string `json:"size" gorm:"type:varchar(32)"`
	Quality       string `json:"quality" gorm:"type:varchar(32)"`
	OutputFormat  string `json:"output_format" gorm:"type:varchar(24)"`
	RevisedPrompt string `json:"revised_prompt" gorm:"type:text"`
	FilePath      string `json:"-" gorm:"type:text"`
	MimeType      string `json:"mime_type" gorm:"type:varchar(100)"`
	FileSize      int64  `json:"file_size"`
}

func CreateImageGeneration(record *ImageGeneration) error {
	if record.CreatedAt == 0 {
		record.CreatedAt = time.Now().Unix()
	}
	return DB.Create(record).Error
}

func GetUserImageGenerations(userId int, startIdx int, num int) (records []*ImageGeneration, total int64, err error) {
	query := DB.Where("user_id = ?", userId)
	if err = query.Model(&ImageGeneration{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&records).Error
	return records, total, err
}

func GetUserImageGenerationByID(userId int, id int64) (*ImageGeneration, error) {
	var record ImageGeneration
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func DeleteUserImageGeneration(userId int, id int64) (*ImageGeneration, error) {
	record, err := GetUserImageGenerationByID(userId, id)
	if err != nil {
		return nil, err
	}
	err = DB.Delete(&ImageGeneration{}, "id = ? AND user_id = ?", id, userId).Error
	if err != nil {
		return nil, err
	}
	return record, nil
}

func ImageGenerationNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
