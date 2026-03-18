package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetGroupShards(c *gin.Context) {
	shards, err := model.GetAllGroupShards()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    shards,
	})
}

func CreateGroupShard(c *gin.Context) {
	var shard model.GroupShard
	if err := c.ShouldBindJSON(&shard); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if shard.ParentGroup == "" || shard.ShardGroup == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "父分组和分片名称不能为空",
		})
		return
	}
	// Check that shard group name doesn't conflict with an existing parent group
	if model.IsParentGroup(shard.ShardGroup) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分片名称不能与已有的父分组名冲突",
		})
		return
	}
	if err := model.CreateGroupShard(&shard); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// Refresh caches
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateGroupShard(c *gin.Context) {
	var shard model.GroupShard
	if err := c.ShouldBindJSON(&shard); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if shard.Id == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的分片ID",
		})
		return
	}
	if err := model.UpdateGroupShard(&shard); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// Refresh caches
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteGroupShard(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的ID",
		})
		return
	}
	shard, err := model.GetGroupShardById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分片不存在",
		})
		return
	}
	if shard.CurrentUsers > 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该分片下仍有用户，无法删除。请先迁移用户或使用重新统计功能确认用户数。",
		})
		return
	}
	if err := model.DeleteGroupShard(id); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// Refresh caches
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func RecountGroupShardUsers(c *gin.Context) {
	if err := model.RecountAllGroupShardUsers(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// Refresh shard cache
	model.InitGroupShardCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "重新统计完成",
	})
}

func AssignUserToShard(c *gin.Context) {
	var req struct {
		UserId     int    `json:"user_id"`
		ShardGroup string `json:"shard_group"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if req.UserId <= 0 || req.ShardGroup == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户ID和分片名称不能为空",
		})
		return
	}
	if err := model.AssignUserToShardManual(req.UserId, req.ShardGroup); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// Update user group cache
	_ = model.UpdateUserGroupCache(req.UserId, req.ShardGroup)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户已分配到分片 " + req.ShardGroup,
	})
}
