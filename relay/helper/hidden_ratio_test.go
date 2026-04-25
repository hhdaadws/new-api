package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestApplyHiddenRatioNilSafeAndIdempotent(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 40,
		TotalTokens:      140,
	}

	require.False(t, ApplyHiddenRatio(nil, usage))

	info := &relaycommon.RelayInfo{
		OriginModelName: "hidden-ratio-test",
		PriceData: types.PriceData{
			HiddenRatio: 1.5,
		},
	}

	require.True(t, ApplyHiddenRatio(info, usage))
	require.Equal(t, 150, usage.PromptTokens)
	require.Equal(t, 60, usage.CompletionTokens)
	require.Equal(t, 210, usage.TotalTokens)
	require.True(t, usage.HiddenRatioApplied)

	require.False(t, ApplyHiddenRatio(info, usage))
	require.Equal(t, 150, usage.PromptTokens)
	require.Equal(t, 60, usage.CompletionTokens)
	require.Equal(t, 210, usage.TotalTokens)

	require.False(t, ApplyHiddenRatio(info, nil))
}

func TestApplyHiddenRatioHonorsTargetModelsAndOutputLimit(t *testing.T) {
	savedTargets := ratio_setting.HiddenRatioTargetModels2JSONString()
	savedMaxOutput := ratio_setting.ModelMaxOutput2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateHiddenRatioTargetModelsByJSONString(savedTargets))
		require.NoError(t, ratio_setting.UpdateModelMaxOutputByJSONString(savedMaxOutput))
	})

	require.NoError(t, ratio_setting.UpdateHiddenRatioTargetModelsByJSONString(`["allowed"]`))

	blockedUsage := &dto.Usage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140}
	blockedInfo := &relaycommon.RelayInfo{
		OriginModelName: "blocked-model",
		PriceData:       types.PriceData{HiddenRatio: 2},
	}
	require.False(t, ApplyHiddenRatio(blockedInfo, blockedUsage))
	require.Equal(t, 100, blockedUsage.PromptTokens)
	require.Equal(t, 40, blockedUsage.CompletionTokens)

	require.NoError(t, ratio_setting.UpdateHiddenRatioTargetModelsByJSONString(`[]`))
	require.NoError(t, ratio_setting.UpdateModelMaxOutputByJSONString(`{"limited":100}`))

	limitedUsage := &dto.Usage{PromptTokens: 100, CompletionTokens: 80, TotalTokens: 180}
	limitedInfo := &relaycommon.RelayInfo{
		OriginModelName: "limited-model",
		PriceData:       types.PriceData{HiddenRatio: 2},
	}
	require.True(t, ApplyHiddenRatio(limitedInfo, limitedUsage))
	require.Equal(t, 119, limitedUsage.PromptTokens)
	require.Equal(t, 95, limitedUsage.CompletionTokens)
	require.Equal(t, 214, limitedUsage.TotalTokens)
}
