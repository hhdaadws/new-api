package model

import (
	"github.com/QuantumNous/new-api/common"
)

type TicketMessage struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	TicketId    int    `json:"ticket_id" gorm:"index;not null"`
	UserId      int    `json:"user_id" gorm:"index;not null"`
	Username    string `json:"username" gorm:"type:varchar(255)"`
	Role        int    `json:"role" gorm:"type:int;not null"`
	Content     string `json:"content" gorm:"type:text;not null"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
}

func (m *TicketMessage) Insert() error {
	m.CreatedTime = common.GetTimestamp()
	return DB.Create(m).Error
}

func GetTicketMessages(ticketId int) ([]*TicketMessage, error) {
	var messages []*TicketMessage
	err := DB.Where("ticket_id = ?", ticketId).Order("id asc").Find(&messages).Error
	return messages, err
}

func GetTicketMessageCount(ticketId int) (int64, error) {
	var count int64
	err := DB.Model(&TicketMessage{}).Where("ticket_id = ?", ticketId).Count(&count).Error
	return count, err
}
