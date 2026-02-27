package openai

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
)

// applyHiddenRatio is a package-local wrapper for helper.ApplyHiddenRatio.
func applyHiddenRatio(info *relaycommon.RelayInfo, usage *dto.Usage) bool {
	return helper.ApplyHiddenRatio(info, usage)
}
