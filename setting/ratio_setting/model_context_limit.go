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

// Max output tokens per model — a separate constraint from context window.
// If completion * ratio exceeds this, the user will notice the anomaly.
var defaultModelMaxOutput = map[string]int{
	"claude-opus-4-6":   128000,
	"claude-opus-4-5":   64000,
	"claude-opus-4-1":   32000,
	"claude-opus-4":     32000,
	"claude-sonnet-4-6": 64000,
	"claude-sonnet-4-5": 64000,
	"claude-sonnet-4":   64000,
	"claude-haiku-4":    64000,
	"claude-3-5-sonnet": 8192,
	"claude-3-opus":     4096,
	"gpt-4o":            16384,
	"gpt-4o-mini":       16384,
	"gpt-4-turbo":       4096,
	"o1":                100000,
	"o3":                100000,
	"o4-mini":           100000,
	"deepseek-chat":     8192,
	"deepseek-reasoner": 8192,
	"gemini-2.0-flash":  8192,
	"gemini-2.5-pro":    65536,
	"gemini-2.5-flash":  65536,
}

var modelContextLimitMap = types.NewRWMap[string, int]()
var modelMaxOutputMap = types.NewRWMap[string, int]()

func init() {
	modelContextLimitMap.AddAll(defaultModelContextLimit)
	modelMaxOutputMap.AddAll(defaultModelMaxOutput)
}

// lookupWithSuffixStrip tries exact match first, then progressively strips
// trailing "-xxx" segments until a match is found. Returns 0 if no match.
func lookupWithSuffixStrip(m *types.RWMap[string, int], name string) int {
	if limit, ok := m.Get(name); ok {
		return limit
	}
	remaining := name
	for {
		idx := strings.LastIndex(remaining, "-")
		if idx < 0 {
			break
		}
		remaining = remaining[:idx]
		if limit, ok := m.Get(remaining); ok {
			return limit
		}
	}
	return 0
}

// GetModelContextLimit returns the context window size for a model.
// Returns 0 if no match is found (meaning no truncation).
func GetModelContextLimit(name string) int {
	return lookupWithSuffixStrip(modelContextLimitMap, name)
}

// GetModelMaxOutput returns the max output token limit for a model.
// Returns 0 if no match is found (meaning no output-based truncation).
func GetModelMaxOutput(name string) int {
	return lookupWithSuffixStrip(modelMaxOutputMap, name)
}

func ModelContextLimit2JSONString() string {
	return modelContextLimitMap.MarshalJSONString()
}

func UpdateModelContextLimitByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(modelContextLimitMap, jsonStr)
}

func ModelMaxOutput2JSONString() string {
	return modelMaxOutputMap.MarshalJSONString()
}

func UpdateModelMaxOutputByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(modelMaxOutputMap, jsonStr)
}
