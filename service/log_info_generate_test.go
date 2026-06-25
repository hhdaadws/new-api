package service

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestInjectTieredBillingInfoAddsServiceTierAndMultiplier(t *testing.T) {
	expr := `tier("standard", p * 2) * (param("service_tier") == "priority" ? 2 : 1)`
	other := map[string]interface{}{}
	relayInfo := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  expr,
		},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{"service_tier":"priority"}`),
		},
	}
	result := &billingexpr.TieredResult{
		MatchedTier:         "standard",
		EffectiveMultiplier: 2,
	}

	InjectTieredBillingInfo(other, relayInfo, result)

	if other["service_tier"] != "priority" {
		t.Fatalf("service_tier = %v, want priority", other["service_tier"])
	}
	if other["matched_tier"] != "standard" {
		t.Fatalf("matched_tier = %v, want standard", other["matched_tier"])
	}
	if other["tiered_multiplier"] != 2.0 {
		t.Fatalf("tiered_multiplier = %v, want 2", other["tiered_multiplier"])
	}
}

func TestInjectTieredBillingInfoAddsActualQuota(t *testing.T) {
	other := map[string]interface{}{}
	relayInfo := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("standard", p * 2)`,
		},
	}
	result := &billingexpr.TieredResult{
		MatchedTier:            "standard",
		ActualQuotaBeforeGroup: 1234.5,
		ActualQuotaAfterGroup:  432,
	}

	InjectTieredBillingInfo(other, relayInfo, result)

	if other["tiered_quota_before_group"] != 1234.5 {
		t.Fatalf("tiered_quota_before_group = %v, want 1234.5", other["tiered_quota_before_group"])
	}
	if other["tiered_quota_after_group"] != 432 {
		t.Fatalf("tiered_quota_after_group = %v, want 432", other["tiered_quota_after_group"])
	}
}
