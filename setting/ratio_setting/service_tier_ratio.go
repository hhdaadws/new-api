package ratio_setting

import "github.com/QuantumNous/new-api/setting/config"

type ServiceTierSetting struct {
	PriorityRatio float64 `json:"priority_ratio"` // priority 层级倍率，默认 2.0
}

var serviceTierSetting = ServiceTierSetting{
	PriorityRatio: 2.0,
}

func init() {
	config.GlobalConfig.Register("service_tier_setting", &serviceTierSetting)
}

func GetServiceTierSetting() *ServiceTierSetting {
	return &serviceTierSetting
}

// GetServiceTierRatio 根据 service_tier 值返回对应的计费倍率。
// "priority" 返回配置的 PriorityRatio，其余返回 1.0（不调整）。
func GetServiceTierRatio(tier string) float64 {
	if tier == "priority" {
		return serviceTierSetting.PriorityRatio
	}
	return 1.0
}
