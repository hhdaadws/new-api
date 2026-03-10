package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// NotifyAdminsNewTicket 新工单通知管理员
func NotifyAdminsNewTicket(ticket *model.Ticket) {
	admins, err := model.GetAdminUsers()
	if err != nil {
		common.SysLog(fmt.Sprintf("获取管理员列表失败: %s", err.Error()))
		return
	}

	categoryNames := map[int]string{
		model.TicketCategoryAccount:        "账户问题",
		model.TicketCategoryBilling:        "计费问题",
		model.TicketCategoryTechnical:      "技术支持",
		model.TicketCategoryFeatureRequest: "功能建议",
		model.TicketCategoryOther:          "其他",
	}
	priorityNames := map[int]string{
		model.TicketPriorityLow:    "低",
		model.TicketPriorityMedium: "中",
		model.TicketPriorityHigh:   "高",
		model.TicketPriorityUrgent: "紧急",
	}

	category := categoryNames[ticket.Category]
	priority := priorityNames[ticket.Priority]

	subject := fmt.Sprintf("新工单 #%d: %s", ticket.Id, ticket.Title)
	content := fmt.Sprintf("用户 %s 提交了新工单\n分类: %s\n优先级: %s\n内容: %s",
		ticket.Username, category, priority, ticket.Content)

	notification := dto.NewNotify(dto.NotifyTypeTicket, subject, content, nil)

	for _, admin := range admins {
		userSetting := admin.GetSetting()
		if err := NotifyUser(admin.Id, admin.Email, userSetting, notification); err != nil {
			common.SysLog(fmt.Sprintf("通知管理员 %d 新工单失败: %s", admin.Id, err.Error()))
		}
	}
}

// NotifyUserTicketReply 管理员回复后通知用户
func NotifyUserTicketReply(ticket *model.Ticket, user *model.User) {
	if user == nil {
		return
	}

	subject := fmt.Sprintf("工单 #%d 有新回复: %s", ticket.Id, ticket.Title)
	content := fmt.Sprintf("您的工单「%s」收到了管理员的新回复，请登录查看。", ticket.Title)

	notification := dto.NewNotify(dto.NotifyTypeTicket, subject, content, nil)
	userSetting := user.GetSetting()
	if err := NotifyUser(user.Id, user.Email, userSetting, notification); err != nil {
		common.SysLog(fmt.Sprintf("通知用户 %d 工单回复失败: %s", user.Id, err.Error()))
	}
}

// NotifyUserTicketStatusChange 工单状态变更通知用户
func NotifyUserTicketStatusChange(ticket *model.Ticket, user *model.User) {
	if user == nil {
		return
	}

	statusNames := map[int]string{
		model.TicketStatusOpen:       "待处理",
		model.TicketStatusInProgress: "处理中",
		model.TicketStatusResolved:   "已解决",
		model.TicketStatusClosed:     "已关闭",
	}

	statusName := statusNames[ticket.Status]
	subject := fmt.Sprintf("工单 #%d 状态更新: %s", ticket.Id, statusName)
	content := fmt.Sprintf("您的工单「%s」状态已更新为: %s", ticket.Title, statusName)

	notification := dto.NewNotify(dto.NotifyTypeTicket, subject, content, nil)
	userSetting := user.GetSetting()
	if err := NotifyUser(user.Id, user.Email, userSetting, notification); err != nil {
		common.SysLog(fmt.Sprintf("通知用户 %d 工单状态变更失败: %s", user.Id, err.Error()))
	}
}

// NotifyAdminsTicketReply 用户回复后通知管理员（优先通知指派人）
func NotifyAdminsTicketReply(ticket *model.Ticket) {
	subject := fmt.Sprintf("工单 #%d 用户回复: %s", ticket.Id, ticket.Title)
	content := fmt.Sprintf("用户 %s 在工单「%s」中提交了新回复，请登录查看。", ticket.Username, ticket.Title)

	notification := dto.NewNotify(dto.NotifyTypeTicket, subject, content, nil)

	// 如果有指派人，优先通知指派人
	if ticket.AssignedTo > 0 {
		admin, err := model.GetUserById(ticket.AssignedTo, false)
		if err == nil && admin != nil {
			userSetting := admin.GetSetting()
			if err := NotifyUser(admin.Id, admin.Email, userSetting, notification); err != nil {
				common.SysLog(fmt.Sprintf("通知指派管理员 %d 工单回复失败: %s", admin.Id, err.Error()))
			}
			return
		}
	}

	// 没有指派人则通知所有管理员
	admins, err := model.GetAdminUsers()
	if err != nil {
		common.SysLog(fmt.Sprintf("获取管理员列表失败: %s", err.Error()))
		return
	}

	for _, admin := range admins {
		userSetting := admin.GetSetting()
		if err := NotifyUser(admin.Id, admin.Email, userSetting, notification); err != nil {
			common.SysLog(fmt.Sprintf("通知管理员 %d 工单回复失败: %s", admin.Id, err.Error()))
		}
	}
}
