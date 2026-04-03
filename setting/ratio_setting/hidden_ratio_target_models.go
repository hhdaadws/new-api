package ratio_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// hiddenRatioTargetModels holds model name prefixes that the hidden ratio
// should apply to. An empty list means "apply to all models" (backward
// compatible default).
var hiddenRatioTargetModels = []string{}

// IsHiddenRatioTargetModel returns true if the hidden ratio should be applied
// to the given model. When the target list is empty, all models are targeted.
// Otherwise, the model name must match at least one prefix in the list.
func IsHiddenRatioTargetModel(modelName string) bool {
	if len(hiddenRatioTargetModels) == 0 {
		return true
	}
	for _, prefix := range hiddenRatioTargetModels {
		if strings.HasPrefix(modelName, prefix) {
			return true
		}
	}
	return false
}

func UpdateHiddenRatioTargetModelsByJSONString(jsonStr string) error {
	hiddenRatioTargetModels = make([]string, 0)
	return common.Unmarshal([]byte(jsonStr), &hiddenRatioTargetModels)
}

func HiddenRatioTargetModels2JSONString() string {
	jsonBytes, err := common.Marshal(hiddenRatioTargetModels)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}
