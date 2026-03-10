package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type CreateTicketRequest struct {
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
	Category int    `json:"category"`
	Priority int    `json:"priority"`
}

type AddMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type UpdateStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

type AssignTicketRequest struct {
	AdminId   int    `json:"admin_id" binding:"required"`
	AdminName string `json:"admin_name"`
}

type RateTicketRequest struct {
	Rating int `json:"rating" binding:"required"`
}

// CreateTicket 用户创建工单
func CreateTicket(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	userId := c.GetInt("id")
	username := c.GetString("username")

	ticket := &model.Ticket{
		UserId:   userId,
		Username: username,
		Title:    req.Title,
		Content:  req.Content,
		Category: req.Category,
		Priority: req.Priority,
		Status:   model.TicketStatusOpen,
	}

	if ticket.Category < 1 || ticket.Category > 5 {
		ticket.Category = model.TicketCategoryOther
	}
	if ticket.Priority < 1 || ticket.Priority > 4 {
		ticket.Priority = model.TicketPriorityMedium
	}

	if err := ticket.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	// 异步通知管理员
	go service.NotifyAdminsNewTicket(ticket)

	common.ApiSuccess(c, ticket)
}

// GetUserTickets 用户获取自己的工单列表
func GetUserTickets(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	status, _ := strconv.Atoi(c.Query("status"))

	tickets, total, err := model.GetUserTickets(userId, status, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

// SearchUserTickets 用户搜索自己的工单
func SearchUserTickets(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	status, _ := strconv.Atoi(c.Query("status"))

	tickets, total, err := model.SearchUserTickets(userId, keyword, status, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

// GetUserTicket 用户获取单个工单详情（含消息）
func GetUserTicket(c *gin.Context) {
	userId := c.GetInt("id")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	ticket, err := model.GetTicketById(ticketId)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}

	if ticket.UserId != userId {
		common.ApiErrorMsg(c, "无权查看此工单")
		return
	}

	messages, err := model.GetTicketMessages(ticketId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"ticket":   ticket,
		"messages": messages,
	})
}

// AddTicketMessage 用户回复工单
func AddTicketMessage(c *gin.Context) {
	userId := c.GetInt("id")
	username := c.GetString("username")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	ticket, err := model.GetTicketById(ticketId)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}

	if ticket.UserId != userId {
		common.ApiErrorMsg(c, "无权操作此工单")
		return
	}

	if ticket.Status == model.TicketStatusClosed {
		common.ApiErrorMsg(c, "工单已关闭，无法回复")
		return
	}

	var req AddMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	message := &model.TicketMessage{
		TicketId: ticketId,
		UserId:   userId,
		Username: username,
		Role:     common.RoleCommonUser,
		Content:  req.Content,
	}

	if err := message.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	// 更新工单时间
	_ = model.UpdateTicketStatus(ticketId, ticket.Status)

	// 异步通知管理员
	go service.NotifyAdminsTicketReply(ticket)

	common.ApiSuccess(c, message)
}

// CloseTicket 用户关闭工单
func CloseTicket(c *gin.Context) {
	userId := c.GetInt("id")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if err := model.CloseTicket(ticketId, userId); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// RateTicket 用户评价工单
func RateTicket(c *gin.Context) {
	userId := c.GetInt("id")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	var req RateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if err := model.RateTicket(ticketId, userId, req.Rating); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// GetAllTickets 管理员获取所有工单
func GetAllTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status, _ := strconv.Atoi(c.Query("status"))
	priority, _ := strconv.Atoi(c.Query("priority"))
	category, _ := strconv.Atoi(c.Query("category"))
	assignedTo, _ := strconv.Atoi(c.Query("assigned_to"))
	keyword := c.Query("keyword")

	var (
		tickets []*model.Ticket
		total   int64
		err     error
	)

	if keyword != "" {
		tickets, total, err = model.SearchTickets(keyword, status, priority, category, pageInfo)
	} else {
		tickets, total, err = model.GetAllTickets(status, priority, category, assignedTo, pageInfo)
	}

	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

// SearchTickets 管理员搜索工单
func SearchTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	status, _ := strconv.Atoi(c.Query("status"))
	priority, _ := strconv.Atoi(c.Query("priority"))
	category, _ := strconv.Atoi(c.Query("category"))

	tickets, total, err := model.SearchTickets(keyword, status, priority, category, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

// GetTicketDetail 管理员获取工单详情（含消息）
func GetTicketDetail(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	ticket, err := model.GetTicketById(ticketId)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}

	messages, err := model.GetTicketMessages(ticketId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"ticket":   ticket,
		"messages": messages,
	})
}

// UpdateTicketStatus 管理员修改工单状态
func UpdateTicketStatus(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if req.Status < model.TicketStatusOpen || req.Status > model.TicketStatusClosed {
		common.ApiErrorMsg(c, "无效的状态值")
		return
	}

	if err := model.UpdateTicketStatus(ticketId, req.Status); err != nil {
		common.ApiError(c, err)
		return
	}

	// 通知用户状态变更
	if req.Status == model.TicketStatusResolved {
		go func() {
			ticket, user, err := model.GetTicketWithOwner(ticketId)
			if err == nil && user != nil {
				service.NotifyUserTicketStatusChange(ticket, user)
			}
		}()
	}

	common.ApiSuccess(c, nil)
}

// AssignTicket 管理员分配工单
func AssignTicket(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	var req AssignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	// 获取被分配管理员的用户名
	adminName := req.AdminName
	if adminName == "" {
		admin, err := model.GetUserById(req.AdminId, false)
		if err != nil {
			common.ApiErrorMsg(c, "管理员用户不存在")
			return
		}
		adminName = admin.Username
	}

	if err := model.AssignTicket(ticketId, req.AdminId, adminName); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// AdminAddTicketMessage 管理员回复工单
func AdminAddTicketMessage(c *gin.Context) {
	adminId := c.GetInt("id")
	adminName := c.GetString("username")
	adminRole := c.GetInt("role")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	ticket, err := model.GetTicketById(ticketId)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}

	if ticket.Status == model.TicketStatusClosed {
		common.ApiErrorMsg(c, "工单已关闭，无法回复")
		return
	}

	var req AddMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	message := &model.TicketMessage{
		TicketId: ticketId,
		UserId:   adminId,
		Username: adminName,
		Role:     adminRole,
		Content:  req.Content,
	}

	if err := message.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	// 如果工单还是待处理状态，自动转为处理中
	if ticket.Status == model.TicketStatusOpen {
		_ = model.UpdateTicketStatus(ticketId, model.TicketStatusInProgress)
	} else {
		// 仅更新时间
		_ = model.UpdateTicketStatus(ticketId, ticket.Status)
	}

	// 异步通知用户
	go func() {
		_, user, err := model.GetTicketWithOwner(ticketId)
		if err == nil && user != nil {
			service.NotifyUserTicketReply(ticket, user)
		}
	}()

	common.ApiSuccess(c, message)
}

// GetTicketStats 管理员获取工单统计
func GetTicketStats(c *gin.Context) {
	stats, err := model.GetTicketStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, stats)
}

// GetAdminUsers 获取管理员用户列表（用于分配工单）
func GetAdminUsers(c *gin.Context) {
	admins, err := model.GetAdminUsers()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, admins)
}
