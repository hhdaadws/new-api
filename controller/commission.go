package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAllCommissions 管理员获取返佣审核记录
func GetAllCommissions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	status, _ := strconv.Atoi(c.Query("status"))

	var (
		commissions []*model.Commission
		total       int64
		err         error
	)
	if keyword != "" {
		commissions, total, err = model.SearchCommissions(keyword, status, pageInfo)
	} else {
		commissions, total, err = model.GetAllCommissions(status, pageInfo)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(commissions)
	common.ApiSuccess(c, pageInfo)
}

type ApproveCommissionRequest struct {
	Id int `json:"id"`
}

// ApproveCommission 审核通过单条返佣记录
func ApproveCommission(c *gin.Context) {
	var req ApproveCommissionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Id == 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	adminId := c.GetInt("id")
	if err := model.ApproveCommission(req.Id, adminId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type RejectCommissionRequest struct {
	Id     int    `json:"id"`
	Remark string `json:"remark"`
}

// RejectCommission 审核拒绝单条返佣记录
func RejectCommission(c *gin.Context) {
	var req RejectCommissionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Id == 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	adminId := c.GetInt("id")
	if err := model.RejectCommission(req.Id, adminId, req.Remark); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type BatchCommissionRequest struct {
	Ids    []int  `json:"ids"`
	Remark string `json:"remark"`
}

// BatchApproveCommissions 批量通过返佣记录
func BatchApproveCommissions(c *gin.Context) {
	var req BatchCommissionRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	adminId := c.GetInt("id")
	count, err := model.BatchApproveCommissions(req.Ids, adminId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, count)
}

// BatchRejectCommissions 批量拒绝返佣记录
func BatchRejectCommissions(c *gin.Context) {
	var req BatchCommissionRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	adminId := c.GetInt("id")
	count, err := model.BatchRejectCommissions(req.Ids, adminId, req.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, count)
}
