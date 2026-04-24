package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

func TestModelPriceHelperAppliesServiceTierModelRatioToPreConsume(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		billing_setting.RebuildServiceTierRatioIndex()
	})
	savedModelRatio := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedModelRatio))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"service_tier_setting.priority_ratio":        "2",
		"service_tier_setting.priority_model_ratios": `{"gpt-5.4":2,"gpt-5.5":2.5}`,
		"billing_setting.billing_mode":               `{}`,
	}))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-5.4":1,"gpt-5.5":1}`))
	billing_setting.RebuildServiceTierRatioIndex()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("group", "default")

	info54 := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4",
		UserGroup:       "default",
		UsingGroup:      "default",
		ServiceTier:     "fast",
	}
	price54, err := ModelPriceHelper(ctx, info54, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, int(float64(1000)*price54.ModelRatio*2), price54.QuotaToPreConsume)
	require.Equal(t, 2.0, price54.OtherRatios["service_tier"])

	info55 := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.5",
		UserGroup:       "default",
		UsingGroup:      "default",
		ServiceTier:     "priority",
	}
	price55, err := ModelPriceHelper(ctx, info55, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, int(float64(1000)*price55.ModelRatio*2.5), price55.QuotaToPreConsume)
	require.Equal(t, 2.5, price55.OtherRatios["service_tier"])
}
