package billing_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"

	"github.com/stretchr/testify/require"
)

func TestServiceTierRatioForModel(t *testing.T) {
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		RebuildServiceTierRatioIndex()
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"service_tier_setting.priority_ratio":        "2",
		"service_tier_setting.priority_model_ratios": `{"gpt-5.4*":2.1,"gpt-5.4-pro":2.2,"gpt-5.5":2.5}`,
	}))
	RebuildServiceTierRatioIndex()

	require.Equal(t, 2.2, GetServiceTierRatio("fast", "gpt-5.4-pro"))
	require.Equal(t, 2.1, GetServiceTierRatio("priority", "gpt-5.4-2026-03-05"))
	require.Equal(t, 2.5, GetServiceTierRatio("fast", "gpt-5.5"))
	require.Equal(t, 2.0, GetServiceTierRatio("priority", "gpt-5.6"))
	require.Equal(t, 1.0, GetServiceTierRatio("standard", "gpt-5.5"))
}
