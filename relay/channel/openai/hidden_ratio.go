package openai

import (
	"math"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// applyHiddenRatio multiplies all token counts in usage by the hidden ratio.
// Returns true if the usage was modified.
func applyHiddenRatio(info *relaycommon.RelayInfo, usage *dto.Usage) bool {
	hr := info.PriceData.HiddenRatio
	if hr == 0 || hr == 1.0 {
		return false
	}

	// Smart truncation: cap hr so inflated tokens don't exceed model limits
	actualTotal := usage.PromptTokens + usage.CompletionTokens
	if actualTotal > 0 {
		// Constraint 1: total (input+output) must not exceed context window
		modelLimit := ratio_setting.GetModelContextLimit(info.OriginModelName)
		if modelLimit > 0 {
			safeLimit := float64(modelLimit) * 0.95 // 5% safety margin
			maxRatio := safeLimit / float64(actualTotal)
			if hr > maxRatio {
				hr = maxRatio
			}
		}

		// Constraint 2: completion tokens must not exceed max output limit
		if usage.CompletionTokens > 0 {
			maxOutput := ratio_setting.GetModelMaxOutput(info.OriginModelName)
			if maxOutput > 0 {
				safeOutput := float64(maxOutput) * 0.95
				maxOutputRatio := safeOutput / float64(usage.CompletionTokens)
				if hr > maxOutputRatio {
					hr = maxOutputRatio
				}
			}
		}

		if hr <= 1.0 {
			return false
		}
	}

	usage.PromptTokens = int(math.Round(float64(usage.PromptTokens) * hr))
	usage.CompletionTokens = int(math.Round(float64(usage.CompletionTokens) * hr))
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	usage.InputTokens = int(math.Round(float64(usage.InputTokens) * hr))
	usage.OutputTokens = int(math.Round(float64(usage.OutputTokens) * hr))

	usage.PromptTokensDetails.CachedTokens = int(math.Round(float64(usage.PromptTokensDetails.CachedTokens) * hr))
	usage.PromptTokensDetails.TextTokens = int(math.Round(float64(usage.PromptTokensDetails.TextTokens) * hr))
	usage.PromptTokensDetails.AudioTokens = int(math.Round(float64(usage.PromptTokensDetails.AudioTokens) * hr))
	usage.PromptTokensDetails.ImageTokens = int(math.Round(float64(usage.PromptTokensDetails.ImageTokens) * hr))

	usage.CompletionTokenDetails.TextTokens = int(math.Round(float64(usage.CompletionTokenDetails.TextTokens) * hr))
	usage.CompletionTokenDetails.AudioTokens = int(math.Round(float64(usage.CompletionTokenDetails.AudioTokens) * hr))
	usage.CompletionTokenDetails.ReasoningTokens = int(math.Round(float64(usage.CompletionTokenDetails.ReasoningTokens) * hr))

	if usage.InputTokensDetails != nil {
		usage.InputTokensDetails.CachedTokens = int(math.Round(float64(usage.InputTokensDetails.CachedTokens) * hr))
		usage.InputTokensDetails.TextTokens = int(math.Round(float64(usage.InputTokensDetails.TextTokens) * hr))
		usage.InputTokensDetails.AudioTokens = int(math.Round(float64(usage.InputTokensDetails.AudioTokens) * hr))
		usage.InputTokensDetails.ImageTokens = int(math.Round(float64(usage.InputTokensDetails.ImageTokens) * hr))
	}

	usage.PromptCacheHitTokens = int(math.Round(float64(usage.PromptCacheHitTokens) * hr))
	usage.ClaudeCacheCreation5mTokens = int(math.Round(float64(usage.ClaudeCacheCreation5mTokens) * hr))
	usage.ClaudeCacheCreation1hTokens = int(math.Round(float64(usage.ClaudeCacheCreation1hTokens) * hr))

	return true
}
