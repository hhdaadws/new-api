package ratio_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/types"
)

var defaultModelContextLimit = map[string]int{
	"claude-opus-4":       200000,
	"claude-sonnet-4":     200000,
	"claude-haiku-4":      200000,
	"claude-3-5-sonnet":   200000,
	"claude-3-opus":       200000,
	"gpt-4o":              128000,
	"gpt-4o-mini":         128000,
	"gpt-4-turbo":         128000,
	"o1":                  200000,
	"o3":                  200000,
	"o4-mini":             200000,
	"deepseek-chat":       128000,
	"deepseek-reasoner":   128000,
	"gemini-2.0-flash":    1048576,
	"gemini-2.5-pro":      1048576,
	"gemini-2.5-flash":    1048576,
}

var modelContextLimitMap = types.NewRWMap[string, int]()

func init() {
	modelContextLimitMap.AddAll(defaultModelContextLimit)
}

// GetModelContextLimit returns the context window size for a model.
// It tries exact match first, then progressively strips trailing segments
// (e.g. "claude-opus-4-6-20250514" -> "claude-opus-4-6" -> "claude-opus-4").
// Returns 0 if no match is found (meaning no truncation).
func GetModelContextLimit(name string) int {
	// exact match
	if limit, ok := modelContextLimitMap.Get(name); ok {
		return limit
	}
	// progressive suffix stripping
	remaining := name
	for {
		idx := strings.LastIndex(remaining, "-")
		if idx < 0 {
			break
		}
		remaining = remaining[:idx]
		if limit, ok := modelContextLimitMap.Get(remaining); ok {
			return limit
		}
	}
	return 0
}

func ModelContextLimit2JSONString() string {
	return modelContextLimitMap.MarshalJSONString()
}

func UpdateModelContextLimitByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(modelContextLimitMap, jsonStr)
}
