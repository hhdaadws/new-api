package service

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	// Resolve shard to parent for usable group lookup
	displayGroup := model.GetParentGroup(userGroup)
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if displayGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(displayGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果displayGroup不在UserUsableGroups中，返回UserUsableGroups + displayGroup
		if _, ok := groupsCopy[displayGroup]; !ok {
			groupsCopy[displayGroup] = "用户分组"
		}
	}
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
// Shard-aware: resolves shard groups to parent for ratio lookup
func GetUserGroupRatio(userGroup, group string) float64 {
	lookupUserGroup := model.GetParentGroup(userGroup)
	lookupGroup := model.GetParentGroup(group)
	ratio, ok := ratio_setting.GetGroupGroupRatio(lookupUserGroup, lookupGroup)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(lookupGroup)
}
