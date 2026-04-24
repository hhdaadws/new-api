package billing_setting

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const defaultPriorityServiceTierRatio = 2.0

type ServiceTierSetting struct {
	PriorityRatio       float64            `json:"priority_ratio"`
	PriorityModelRatios map[string]float64 `json:"priority_model_ratios"`
}

type serviceTierPrefixRatio struct {
	Prefix string
	Ratio  float64
}

type serviceTierRatioIndex struct {
	DefaultRatio float64
	Exact        map[string]float64
	Prefixes     []serviceTierPrefixRatio
}

var serviceTierSetting = ServiceTierSetting{
	PriorityRatio:       defaultPriorityServiceTierRatio,
	PriorityModelRatios: make(map[string]float64),
}

var currentServiceTierRatioIndex atomic.Pointer[serviceTierRatioIndex]

func init() {
	config.GlobalConfig.Register("service_tier_setting", &serviceTierSetting)
	RebuildServiceTierRatioIndex()
}

func GetServiceTierSetting() *ServiceTierSetting {
	return &serviceTierSetting
}

func ValidateServiceTierPriorityRatio(raw string) error {
	ratio, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fmt.Errorf("倍率必须是有效数字")
	}
	if ratio <= 0 {
		return fmt.Errorf("倍率必须大于 0")
	}
	return nil
}

func ValidateServiceTierModelRatios(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	modelRatios := make(map[string]float64)
	if err := common.UnmarshalJsonStr(raw, &modelRatios); err != nil {
		return fmt.Errorf("分模型倍率必须是 JSON 对象: %w", err)
	}

	for model, ratio := range modelRatios {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("模型名不能为空")
		}
		if ratio <= 0 {
			return fmt.Errorf("模型 %s 的倍率必须大于 0", model)
		}
	}
	return nil
}

func RebuildServiceTierRatioIndex() {
	defaultRatio := serviceTierSetting.PriorityRatio
	if defaultRatio <= 0 {
		defaultRatio = defaultPriorityServiceTierRatio
	}

	idx := &serviceTierRatioIndex{
		DefaultRatio: defaultRatio,
		Exact:        make(map[string]float64),
		Prefixes:     make([]serviceTierPrefixRatio, 0),
	}

	for model, ratio := range serviceTierSetting.PriorityModelRatios {
		model = strings.TrimSpace(model)
		if model == "" || ratio <= 0 {
			continue
		}
		if strings.HasSuffix(model, "*") {
			prefix := strings.TrimSuffix(model, "*")
			if prefix == "" {
				continue
			}
			idx.Prefixes = append(idx.Prefixes, serviceTierPrefixRatio{
				Prefix: prefix,
				Ratio:  ratio,
			})
			continue
		}
		idx.Exact[model] = ratio
	}

	sort.Slice(idx.Prefixes, func(i, j int) bool {
		return len(idx.Prefixes[i].Prefix) > len(idx.Prefixes[j].Prefix)
	})

	currentServiceTierRatioIndex.Store(idx)
}

func GetServiceTierRatio(tier string, modelName string) float64 {
	if !isPriorityServiceTier(tier) {
		return 1.0
	}

	idx := currentServiceTierRatioIndex.Load()
	if idx == nil {
		return defaultPriorityServiceTierRatio
	}

	if ratio, ok := idx.Exact[modelName]; ok {
		return ratio
	}
	for _, prefixRatio := range idx.Prefixes {
		if strings.HasPrefix(modelName, prefixRatio.Prefix) {
			return prefixRatio.Ratio
		}
	}
	return idx.DefaultRatio
}

func isPriorityServiceTier(tier string) bool {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "fast", "priority":
		return true
	default:
		return false
	}
}
