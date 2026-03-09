package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

const (
	CommissionStatusPending  = 1
	CommissionStatusApproved = 2
	CommissionStatusRejected = 3
)

type Commission struct {
	Id             int     `json:"id" gorm:"primaryKey"`
	UserId         int     `json:"user_id" gorm:"index"`
	InviterId      int     `json:"inviter_id" gorm:"index"`
	InviterUsername string  `json:"inviter_username" gorm:"type:varchar(255)"`
	InviteeUsername string  `json:"invitee_username" gorm:"type:varchar(255)"`
	QuotaAdded     int     `json:"quota_added"`
	Discount       float64 `json:"discount" gorm:"type:double precision;default:1"`
	Rate           float64 `json:"rate" gorm:"type:double precision"`
	OrderNumber    int     `json:"order_number"`
	Commission     int     `json:"commission"`
	TradeNo        string  `json:"trade_no" gorm:"type:varchar(255);index"`
	Status         int     `json:"status" gorm:"default:1;index"`
	CreatedTime    int64   `json:"created_time" gorm:"bigint"`
	ReviewedTime   int64   `json:"reviewed_time" gorm:"bigint"`
	ReviewedBy     int     `json:"reviewed_by"`
	Remark         string  `json:"remark" gorm:"type:varchar(255)"`
}

func (c *Commission) Insert() error {
	return DB.Create(c).Error
}

func GetAllCommissions(status int, pageInfo *common.PageInfo) (commissions []*Commission, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Commission{})
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&commissions).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return commissions, total, nil
}

func SearchCommissions(keyword string, status int, pageInfo *common.PageInfo) (commissions []*Commission, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Commission{})
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		like := "%%" + keyword + "%%"
		query = query.Where("inviter_username LIKE ? OR invitee_username LIKE ? OR trade_no LIKE ?", like, like, like)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&commissions).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return commissions, total, nil
}

func ApproveCommission(id int, adminId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		commission := &Commission{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", id).First(commission).Error; err != nil {
			return errors.New("返佣记录不存在")
		}

		if commission.Status != CommissionStatusPending {
			return errors.New("该记录已审核，无法重复操作")
		}

		if err := tx.Model(&User{}).Where("id = ?", commission.InviterId).Update("quota", gorm.Expr("quota + ?", commission.Commission)).Error; err != nil {
			return fmt.Errorf("增加邀请人额度失败: %w", err)
		}

		commission.Status = CommissionStatusApproved
		commission.ReviewedTime = common.GetTimestamp()
		commission.ReviewedBy = adminId
		if err := tx.Save(commission).Error; err != nil {
			return err
		}

		RecordLog(commission.InviterId, LogTypeTopup, fmt.Sprintf("邀请返佣审核通过：被邀请用户 %s（ID: %d）第 %d 笔充值，返佣比例 %.1f%%，返佣额度: %v",
			commission.InviteeUsername, commission.UserId, commission.OrderNumber, commission.Rate, logger.LogQuota(commission.Commission)))
		RecordLog(commission.UserId, LogTypeTopup, fmt.Sprintf("充值返佣通知：您的第 %d 笔充值为邀请人 %s（ID: %d）产生的 %.1f%% 返佣已审核通过，返佣额度: %v",
			commission.OrderNumber, commission.InviterUsername, commission.InviterId, commission.Rate, logger.LogQuota(commission.Commission)))

		return nil
	})
}

func RejectCommission(id int, adminId int, remark string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		commission := &Commission{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", id).First(commission).Error; err != nil {
			return errors.New("返佣记录不存在")
		}

		if commission.Status != CommissionStatusPending {
			return errors.New("该记录已审核，无法重复操作")
		}

		commission.Status = CommissionStatusRejected
		commission.ReviewedTime = common.GetTimestamp()
		commission.ReviewedBy = adminId
		commission.Remark = remark
		if err := tx.Save(commission).Error; err != nil {
			return err
		}

		RecordLog(commission.InviterId, LogTypeTopup, fmt.Sprintf("邀请返佣审核拒绝：被邀请用户 %s（ID: %d）第 %d 笔充值，返佣额度: %v，原因: %s",
			commission.InviteeUsername, commission.UserId, commission.OrderNumber, logger.LogQuota(commission.Commission), remark))

		return nil
	})
}

func BatchApproveCommissions(ids []int, adminId int) (int, error) {
	count := 0
	for _, id := range ids {
		if err := ApproveCommission(id, adminId); err != nil {
			common.SysError(fmt.Sprintf("batch approve commission %d failed: %s", id, err.Error()))
			continue
		}
		count++
	}
	return count, nil
}

func BatchRejectCommissions(ids []int, adminId int, remark string) (int, error) {
	count := 0
	for _, id := range ids {
		if err := RejectCommission(id, adminId, remark); err != nil {
			common.SysError(fmt.Sprintf("batch reject commission %d failed: %s", id, err.Error()))
			continue
		}
		count++
	}
	return count, nil
}
