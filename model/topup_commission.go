package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
)

// ProcessTopUpCommission processes inviter commission after a successful top-up.
// It runs asynchronously to avoid blocking the payment callback.
func ProcessTopUpCommission(userId int, quotaAdded int) {
	if !common.InviterCommissionEnabled || !common.HasInviterCommissionRates() {
		return
	}

	gopool.Go(func() {
		processCommission(userId, quotaAdded)
	})
}

func processCommission(userId int, quotaAdded int) {
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

	commission := int(float64(quotaAdded) * rate / 100)
	if commission <= 0 {
		return
	}

	err = IncreaseUserQuota(inviterId, commission, true)
	if err != nil {
		common.SysError(fmt.Sprintf("failed to increase inviter %d quota for commission: %s", inviterId, err.Error()))
		return
	}

	inviterUsername, _ := GetUsernameById(inviterId, false)
	inviteeUsername, _ := GetUsernameById(userId, false)

	RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("邀请返佣：被邀请用户 %s（ID: %d）完成第 %d 笔充值，返佣比例 %.1f%%，获得返佣额度: %v",
		inviteeUsername, userId, orderNumber, rate, logger.LogQuota(commission)))
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("充值返佣通知：您的第 %d 笔充值已为邀请人 %s（ID: %d）产生 %.1f%% 返佣，返佣额度: %v",
		orderNumber, inviterUsername, inviterId, rate, logger.LogQuota(commission)))
}
