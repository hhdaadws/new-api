package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// Ticket status constants
const (
	TicketStatusOpen       = 1
	TicketStatusInProgress = 2
	TicketStatusResolved   = 3
	TicketStatusClosed     = 4
)

// Ticket category constants
const (
	TicketCategoryAccount        = 1
	TicketCategoryBilling        = 2
	TicketCategoryTechnical      = 3
	TicketCategoryFeatureRequest = 4
	TicketCategoryOther          = 5
)

// Ticket priority constants
const (
	TicketPriorityLow    = 1
	TicketPriorityMedium = 2
	TicketPriorityHigh   = 3
	TicketPriorityUrgent = 4
)

type Ticket struct {
	Id           int            `json:"id" gorm:"primaryKey"`
	UserId       int            `json:"user_id" gorm:"index;not null"`
	Username     string         `json:"username" gorm:"type:varchar(255)"`
	Title        string         `json:"title" gorm:"type:varchar(255);not null"`
	Content      string         `json:"content" gorm:"type:text;not null"`
	Category     int            `json:"category" gorm:"type:int;default:5;index"`
	Priority     int            `json:"priority" gorm:"type:int;default:2;index"`
	Status       int            `json:"status" gorm:"type:int;default:1;index"`
	AssignedTo   int            `json:"assigned_to" gorm:"type:int;default:0;index"`
	AssignedName string         `json:"assigned_name" gorm:"type:varchar(255)"`
	Rating       int            `json:"rating" gorm:"type:int;default:0"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64          `json:"updated_time" gorm:"bigint"`
	ClosedTime   int64          `json:"closed_time" gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (t *Ticket) Insert() error {
	t.CreatedTime = common.GetTimestamp()
	t.UpdatedTime = t.CreatedTime
	return DB.Create(t).Error
}

func GetTicketById(id int) (*Ticket, error) {
	var ticket Ticket
	err := DB.Where("id = ?", id).First(&ticket).Error
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func GetUserTickets(userId int, status int, pageInfo *common.PageInfo) (tickets []*Ticket, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Ticket{}).Where("user_id = ?", userId)
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&tickets).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

func SearchUserTickets(userId int, keyword string, status int, pageInfo *common.PageInfo) (tickets []*Ticket, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Ticket{}).Where("user_id = ?", userId)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		like := "%%" + keyword + "%%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&tickets).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

func GetAllTickets(status int, priority int, category int, assignedTo int, pageInfo *common.PageInfo) (tickets []*Ticket, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Ticket{})
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if priority > 0 {
		query = query.Where("priority = ?", priority)
	}
	if category > 0 {
		query = query.Where("category = ?", category)
	}
	if assignedTo > 0 {
		query = query.Where("assigned_to = ?", assignedTo)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&tickets).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

func SearchTickets(keyword string, status int, priority int, category int, pageInfo *common.PageInfo) (tickets []*Ticket, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Ticket{})
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if priority > 0 {
		query = query.Where("priority = ?", priority)
	}
	if category > 0 {
		query = query.Where("category = ?", category)
	}
	if keyword != "" {
		like := "%%" + keyword + "%%"
		query = query.Where("title LIKE ? OR content LIKE ? OR username LIKE ?", like, like, like)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&tickets).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

func UpdateTicketStatus(id int, status int) error {
	updates := map[string]interface{}{
		"status":       status,
		"updated_time": common.GetTimestamp(),
	}
	if status == TicketStatusClosed {
		updates["closed_time"] = common.GetTimestamp()
	}
	return DB.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error
}

func AssignTicket(ticketId int, adminId int, adminName string) error {
	return DB.Model(&Ticket{}).Where("id = ?", ticketId).Updates(map[string]interface{}{
		"assigned_to":   adminId,
		"assigned_name": adminName,
		"updated_time":  common.GetTimestamp(),
	}).Error
}

func CloseTicket(ticketId int, userId int) error {
	result := DB.Model(&Ticket{}).Where("id = ? AND user_id = ?", ticketId, userId).Updates(map[string]interface{}{
		"status":       TicketStatusClosed,
		"updated_time": common.GetTimestamp(),
		"closed_time":  common.GetTimestamp(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("工单不存在或无权操作")
	}
	return nil
}

func RateTicket(ticketId int, userId int, rating int) error {
	if rating < 1 || rating > 5 {
		return errors.New("评分必须在1-5之间")
	}

	var ticket Ticket
	if err := DB.Where("id = ? AND user_id = ?", ticketId, userId).First(&ticket).Error; err != nil {
		return errors.New("工单不存在或无权操作")
	}

	if ticket.Status != TicketStatusResolved && ticket.Status != TicketStatusClosed {
		return errors.New("只能评价已解决或已关闭的工单")
	}

	return DB.Model(&Ticket{}).Where("id = ?", ticketId).Updates(map[string]interface{}{
		"rating":       rating,
		"updated_time": common.GetTimestamp(),
	}).Error
}

func GetTicketStats() (map[string]int64, error) {
	stats := make(map[string]int64)

	var total int64
	if err := DB.Model(&Ticket{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	statuses := []struct {
		name   string
		status int
	}{
		{"open", TicketStatusOpen},
		{"in_progress", TicketStatusInProgress},
		{"resolved", TicketStatusResolved},
		{"closed", TicketStatusClosed},
	}

	for _, s := range statuses {
		var count int64
		if err := DB.Model(&Ticket{}).Where("status = ?", s.status).Count(&count).Error; err != nil {
			return nil, err
		}
		stats[s.name] = count
	}

	return stats, nil
}

func GetAdminUsers() ([]*User, error) {
	var users []*User
	err := DB.Where("role >= ? AND status = ?", common.RoleAdminUser, common.UserStatusEnabled).
		Select("id, username, display_name, email, role").
		Find(&users).Error
	return users, err
}

func GetTicketWithOwner(ticketId int) (*Ticket, *User, error) {
	ticket, err := GetTicketById(ticketId)
	if err != nil {
		return nil, nil, fmt.Errorf("工单不存在")
	}

	user, err := GetUserById(ticket.UserId, false)
	if err != nil {
		return ticket, nil, nil
	}

	return ticket, user, nil
}
