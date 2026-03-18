package model

import (
	"errors"
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

type GroupShard struct {
	Id           int    `json:"id"`
	ParentGroup  string `json:"parent_group" gorm:"type:varchar(64);not null;index:idx_parent_group"`
	ShardGroup   string `json:"shard_group" gorm:"type:varchar(64);not null;uniqueIndex"`
	MaxUsers     int    `json:"max_users" gorm:"type:int;default:0"`
	CurrentUsers int    `json:"current_users" gorm:"type:int;default:0"`
	Enabled      bool   `json:"enabled" gorm:"default:true"`
	SortOrder    int    `json:"sort_order" gorm:"type:int;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

func (s *GroupShard) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *GroupShard) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

// In-memory shard cache
var (
	shardToParent  map[string]string
	parentToShards map[string][]GroupShard
	shardCacheLock sync.RWMutex
)

// InitGroupShardCache loads all group shards from DB and builds lookup maps.
func InitGroupShardCache() {
	var shards []GroupShard
	DB.Where("enabled = ?", true).Order("sort_order asc, id asc").Find(&shards)

	newShardToParent := make(map[string]string)
	newParentToShards := make(map[string][]GroupShard)
	for _, s := range shards {
		newShardToParent[s.ShardGroup] = s.ParentGroup
		newParentToShards[s.ParentGroup] = append(newParentToShards[s.ParentGroup], s)
	}

	shardCacheLock.Lock()
	shardToParent = newShardToParent
	parentToShards = newParentToShards
	shardCacheLock.Unlock()
}

// GetParentGroup returns the parent group name if group is a shard,
// otherwise returns the group itself.
func GetParentGroup(group string) string {
	shardCacheLock.RLock()
	defer shardCacheLock.RUnlock()
	if parent, ok := shardToParent[group]; ok {
		return parent
	}
	return group
}

// IsShardGroup returns true if the group is a shard of some parent group.
func IsShardGroup(group string) bool {
	shardCacheLock.RLock()
	defer shardCacheLock.RUnlock()
	_, ok := shardToParent[group]
	return ok
}

// IsParentGroup returns true if the group has any shards defined.
func IsParentGroup(group string) bool {
	shardCacheLock.RLock()
	defer shardCacheLock.RUnlock()
	shards, ok := parentToShards[group]
	return ok && len(shards) > 0
}

// GetShardsForParent returns all shards for a parent group (from cache).
func GetShardsForParent(parent string) []GroupShard {
	shardCacheLock.RLock()
	defer shardCacheLock.RUnlock()
	result := make([]GroupShard, len(parentToShards[parent]))
	copy(result, parentToShards[parent])
	return result
}

// GetAllShardGroupsForParent returns all shard group names (including the parent itself)
// for use in queries. If group is not a parent, returns just the group.
func GetAllShardGroupsForParent(group string) []string {
	shardCacheLock.RLock()
	defer shardCacheLock.RUnlock()
	shards, ok := parentToShards[group]
	if !ok || len(shards) == 0 {
		return []string{group}
	}
	result := make([]string, 0, len(shards)+1)
	result = append(result, group)
	for _, s := range shards {
		result = append(result, s.ShardGroup)
	}
	return result
}

// isUserInGroupOrShard checks if the user's current group matches the target group
// or is a shard of the target group.
func isUserInGroupOrShard(currentGroup, targetGroup string) bool {
	if currentGroup == targetGroup {
		return true
	}
	return GetParentGroup(currentGroup) == targetGroup
}

// assignToShardTx assigns a user to a shard of the parent group within a transaction.
// Returns the shard group name.
func assignToShardTx(tx *gorm.DB, parentGroup string) (string, error) {
	var shards []GroupShard
	queryOpt := ""
	if !common.UsingSQLite {
		queryOpt = "FOR UPDATE"
	}
	if err := tx.Set("gorm:query_option", queryOpt).
		Where("parent_group = ? AND enabled = ?", parentGroup, true).
		Order("sort_order asc, id asc").
		Find(&shards).Error; err != nil {
		return "", fmt.Errorf("failed to query shards for group %s: %w", parentGroup, err)
	}
	if len(shards) == 0 {
		return "", fmt.Errorf("no shards configured for group %s", parentGroup)
	}

	// First pass: find a shard with capacity
	for _, s := range shards {
		if s.MaxUsers == 0 || s.CurrentUsers < s.MaxUsers {
			if err := tx.Model(&GroupShard{}).Where("id = ?", s.Id).
				UpdateColumn("current_users", gorm.Expr("current_users + 1")).Error; err != nil {
				return "", err
			}
			return s.ShardGroup, nil
		}
	}

	// All shards full — soft limit: pick the shard with fewest users
	minUsers := shards[0].CurrentUsers
	minIdx := 0
	for i, s := range shards {
		if s.CurrentUsers < minUsers {
			minUsers = s.CurrentUsers
			minIdx = i
		}
	}
	selected := shards[minIdx]
	if err := tx.Model(&GroupShard{}).Where("id = ?", selected.Id).
		UpdateColumn("current_users", gorm.Expr("current_users + 1")).Error; err != nil {
		return "", err
	}
	return selected.ShardGroup, nil
}

// decrementShardUserCountTx decrements the current_users count for a shard within a transaction.
func decrementShardUserCountTx(tx *gorm.DB, shardGroup string) error {
	return tx.Model(&GroupShard{}).
		Where("shard_group = ? AND current_users > 0", shardGroup).
		UpdateColumn("current_users", gorm.Expr("current_users - 1")).Error
}

// incrementShardUserCountTx increments the current_users count for a shard within a transaction.
func incrementShardUserCountTx(tx *gorm.DB, shardGroup string) error {
	return tx.Model(&GroupShard{}).
		Where("shard_group = ?", shardGroup).
		UpdateColumn("current_users", gorm.Expr("current_users + 1")).Error
}

// --- CRUD operations ---

func GetAllGroupShards() ([]GroupShard, error) {
	var shards []GroupShard
	err := DB.Order("parent_group asc, sort_order asc, id asc").Find(&shards).Error
	return shards, err
}

func GetGroupShardById(id int) (*GroupShard, error) {
	var shard GroupShard
	err := DB.First(&shard, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &shard, nil
}

func GetGroupShardByShardGroup(shardGroup string) (*GroupShard, error) {
	var shard GroupShard
	err := DB.First(&shard, "shard_group = ?", shardGroup).Error
	if err != nil {
		return nil, err
	}
	return &shard, nil
}

func CreateGroupShard(shard *GroupShard) error {
	return DB.Create(shard).Error
}

func UpdateGroupShard(shard *GroupShard) error {
	return DB.Save(shard).Error
}

func DeleteGroupShard(id int) error {
	return DB.Delete(&GroupShard{}, "id = ?", id).Error
}

// RecountAllGroupShardUsers recounts current_users for all shards from actual user data.
func RecountAllGroupShardUsers() error {
	var shards []GroupShard
	if err := DB.Find(&shards).Error; err != nil {
		return err
	}
	for _, shard := range shards {
		var count int64
		if err := DB.Model(&User{}).Where(commonGroupCol+" = ?", shard.ShardGroup).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to count users for shard %s: %w", shard.ShardGroup, err)
		}
		if err := DB.Model(&GroupShard{}).Where("id = ?", shard.Id).
			Update("current_users", count).Error; err != nil {
			return fmt.Errorf("failed to update count for shard %s: %w", shard.ShardGroup, err)
		}
	}
	return nil
}

// AssignUserToShardManual allows admin to manually assign a user to a specific shard.
func AssignUserToShardManual(userId int, targetShardGroup string) error {
	if userId <= 0 || targetShardGroup == "" {
		return errors.New("invalid userId or shard group")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		// Get the target shard (verify it exists)
		var targetShard GroupShard
		if err := tx.Where("shard_group = ?", targetShardGroup).First(&targetShard).Error; err != nil {
			return fmt.Errorf("shard %s not found", targetShardGroup)
		}

		// Get user's current group
		currentGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return err
		}

		// If already in target shard, no-op
		if currentGroup == targetShardGroup {
			return nil
		}

		// If user is currently in a shard, decrement old shard count
		if IsShardGroup(currentGroup) {
			if err := decrementShardUserCountTx(tx, currentGroup); err != nil {
				return err
			}
		}

		// Update user's group
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("group", targetShardGroup).Error; err != nil {
			return err
		}

		// Increment target shard count
		if err := incrementShardUserCountTx(tx, targetShardGroup); err != nil {
			return err
		}

		return nil
	})
}
