package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
)

// ProcessTopUpCommission processes inviter commission after a successful top-up.
// It runs asynchronously to avoid blocking the payment callback.
// discount is the order's discount factor (e.g. 0.95 for 95% price); commission is based on discounted amount.
func ProcessTopUpCommission(userId int, quotaAdded int, discount float64, tradeNo string) {
	if !common.InviterCommissionEnabled || !common.HasInviterCommissionRates() {
		return
	}

	gopool.Go(func() {
		processCommission(userId, quotaAdded, discount, tradeNo)
	})
}

func processCommission(userId int, quotaAdded int, discount float64, tradeNo string) {
	// Get the user's inviter
	user, err := GetUserById(userId, false)
	if err != nil || user == nil || user.InviterId == 0 {
		return
	}
	inviterId := user.InviterId

	// Count completed top-up orders for this user (current order is already saved as success)
	var orderCount int64
	err = DB.Model(&TopUp{}).Where("user_id = ? AND status = ?", userId, common.TopUpStatusSuccess).Count(&orderCount).Error
	if err != nil {
		common.SysError(fmt.Sprintf("failed to count top-up orders for user %d: %s", userId, err.Error()))
		return
	}

	orderNumber := int(orderCount)
	rate, ok := common.GetInviterCommissionRate(orderNumber)
	if !ok || rate <= 0 {
		return
	}

	// Apply discount: commission is based on actual payment, not original amount
	if discount <= 0 {
		discount = 1.0
	}
	commissionBasis := int(float64(quotaAdded) * discount)
	commission := int(float64(commissionBasis) * rate / 100)
	if commission <= 0 {
		return
	}

	inviterUsername, _ := GetUsernameById(inviterId, false)
	inviteeUsername, _ := GetUsernameById(userId, false)

	// Create pending commission record for admin review
	record := &Commission{
		UserId:         userId,
		InviterId:      inviterId,
		InviterUsername: inviterUsername,
		InviteeUsername: inviteeUsername,
		QuotaAdded:     quotaAdded,
		Discount:       discount,
		Rate:           rate,
		OrderNumber:    orderNumber,
		Commission:     commission,
		TradeNo:        tradeNo,
		Status:         CommissionStatusPending,
		CreatedTime:    common.GetTimestamp(),
	}
	if err := record.Insert(); err != nil {
		common.SysError(fmt.Sprintf("failed to create commission record for inviter %d: %s", inviterId, err.Error()))
		return
	}

	RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("邀请返佣待审核：被邀请用户 %s（ID: %d）完成第 %d 笔充值，折扣 %.2f，返佣比例 %.1f%%，返佣额度: %v（待管理员审核）",
		inviteeUsername, userId, orderNumber, discount, rate, logger.LogQuota(commission)))
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("充值返佣通知：您的第 %d 笔充值已为邀请人 %s（ID: %d）产生 %.1f%% 返佣（折扣 %.2f），返佣额度: %v（待管理员审核）",
		orderNumber, inviterUsername, inviterId, rate, discount, logger.LogQuota(commission)))
}
