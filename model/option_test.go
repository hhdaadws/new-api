package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestHiddenRatioLimitOptionsUpdateRuntimeMaps(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	savedOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()

	savedContextLimit := ratio_setting.ModelContextLimit2JSONString()
	savedMaxOutput := ratio_setting.ModelMaxOutput2JSONString()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = savedOptionMap
		common.OptionMapRWMutex.Unlock()
		require.NoError(t, ratio_setting.UpdateModelContextLimitByJSONString(savedContextLimit))
		require.NoError(t, ratio_setting.UpdateModelMaxOutputByJSONString(savedMaxOutput))
	})

	contextValue := `{"custom-hidden-model":321}`
	require.NoError(t, updateOptionMap("ModelContextLimit", contextValue))
	require.Equal(t, 321, ratio_setting.GetModelContextLimit("custom-hidden-model-v1"))

	maxOutputValue := `{"custom-hidden-model":123}`
	require.NoError(t, updateOptionMap("ModelMaxOutput", maxOutputValue))
	require.Equal(t, 123, ratio_setting.GetModelMaxOutput("custom-hidden-model-v1"))

	common.OptionMapRWMutex.RLock()
	require.Equal(t, contextValue, common.OptionMap["ModelContextLimit"])
	require.Equal(t, maxOutputValue, common.OptionMap["ModelMaxOutput"])
	common.OptionMapRWMutex.RUnlock()
}
