package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type AnalyzeUsersRequest struct {
	Group             string `json:"group"`
	MaxCount          int    `json:"max_count"`
	WeeklyBudgetQuota int64  `json:"weekly_budget_quota"`
	ExcludeUserIds    []int  `json:"exclude_user_ids"`
}

func AnalyzeChannelUsers(c *gin.Context) {
	var req AnalyzeUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}

	if req.MaxCount <= 0 || req.MaxCount > 1000 {
		common.ApiErrorMsg(c, "max_count must be between 1 and 1000")
		return
	}
	if req.WeeklyBudgetQuota <= 0 {
		common.ApiErrorMsg(c, "weekly_budget_quota must be greater than 0")
		return
	}

	users, err := model.GetUserWeeklyUsage(req.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Filter out excluded users
	if len(req.ExcludeUserIds) > 0 {
		excludeSet := make(map[int]bool, len(req.ExcludeUserIds))
		for _, id := range req.ExcludeUserIds {
			excludeSet[id] = true
		}
		filtered := make([]model.UserWeeklyUsage, 0, len(users))
		for _, u := range users {
			if !excludeSet[u.UserId] {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}

	selected := model.SelectOptimalUsers(users, req.MaxCount, req.WeeklyBudgetQuota)

	common.ApiSuccess(c, selected)
}

type BatchBindUsersRequest struct {
	UserIds       []int `json:"user_ids"`
	ExpireMinutes int   `json:"expire_minutes"`
}

func BatchBindChannelUsers(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req BatchBindUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}

	if len(req.UserIds) == 0 {
		common.ApiErrorMsg(c, "user_ids must not be empty")
		return
	}
	if len(req.UserIds) > 100 {
		common.ApiErrorMsg(c, "cannot bind more than 100 users at once")
		return
	}

	channel, err := model.GetChannelById(channelId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	maxUsers := channel.GetMaxUsers()
	expireMinutes := channel.GetUserBindExpireMinutes()

	boundCount := 0
	for _, userId := range req.UserIds {
		if maxUsers <= 0 {
			model.CacheBindUser(channelId, userId)
			boundCount++
		} else if model.CacheBindUserIfRoom(channelId, userId, maxUsers, expireMinutes) {
			boundCount++
		}
	}

	// Update expire_minutes on channel if specified and different
	if req.ExpireMinutes > 0 && channel.GetUserBindExpireMinutes() != req.ExpireMinutes {
		model.DB.Model(&model.Channel{}).Where("id = ?", channelId).
			Update("user_bind_expire_minutes", req.ExpireMinutes)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"bound_count":   boundCount,
			"skipped_count": len(req.UserIds) - boundCount,
		},
	})
}
