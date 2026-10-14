package operation_setting

import (
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// ErrorKeywordRetryRule sends the retry of a failed attempt to RetryChannelID
// when the attempt ran on ChannelID in Group and its error contains any of
// Keywords, ignoring case.
type ErrorKeywordRetryRule struct {
	Group          string   `json:"group"`
	ChannelID      int      `json:"channel_id"`
	Keywords       []string `json:"keywords"`
	RetryChannelID int      `json:"retry_channel_id"`
}

type ErrorKeywordRetrySetting struct {
	Enabled bool `json:"enabled"`
	// Rules backs the option defaults and is replaced while the option map
	// is locked. Relays read ErrorKeywordRetryRules instead.
	Rules []ErrorKeywordRetryRule `json:"rules"`
}

const ErrorKeywordRetryRulesOptionKey = "error_keyword_retry.rules"

var errorKeywordRetrySetting = ErrorKeywordRetrySetting{
	Enabled: false,
	Rules:   []ErrorKeywordRetryRule{},
}

// errorKeywordRetryRules is the rule list the relay reads. Loading an option
// value swaps in a new list, so a relay never reads one that is being decoded.
var errorKeywordRetryRules atomic.Pointer[[]ErrorKeywordRetryRule]

func init() {
	config.GlobalConfig.Register("error_keyword_retry", &errorKeywordRetrySetting)
}

func GetErrorKeywordRetrySetting() *ErrorKeywordRetrySetting {
	return &errorKeywordRetrySetting
}

// ErrorKeywordRetryRules returns the saved rules in their configured order.
// Callers must not modify the result.
func ErrorKeywordRetryRules() []ErrorKeywordRetryRule {
	if rules := errorKeywordRetryRules.Load(); rules != nil {
		return *rules
	}
	return nil
}

// LoadErrorKeywordRetryRules replaces the rules with a saved option value. A
// value that fails validation loads no rules, so no request is redirected by
// a rule the administrator never saved.
func LoadErrorKeywordRetryRules(value string) {
	rules, err := ParseErrorKeywordRetryRules(value)
	if err != nil {
		common.SysError(common.LogText("failed to load error keyword retry rules: %s", err.Error()))
		rules = []ErrorKeywordRetryRule{}
	}
	errorKeywordRetrySetting.Rules = rules
	errorKeywordRetryRules.Store(&rules)
}

// ParseErrorKeywordRetryRules decodes and checks a rules option value. Group
// names and keywords are trimmed and blank keywords are dropped.
func ParseErrorKeywordRetryRules(value string) ([]ErrorKeywordRetryRule, error) {
	var rules []ErrorKeywordRetryRule
	if err := common.UnmarshalJsonStr(value, &rules); err != nil {
		return nil, common.NewMessage("Error keyword retry rules must be a JSON array of rules")
	}
	for i := range rules {
		rule := &rules[i]
		position := map[string]any{"index": i + 1}
		rule.Group = strings.TrimSpace(rule.Group)
		if rule.Group == "" {
			return nil, common.NewMessage("Rule {{index}}: choose a group", position)
		}
		if rule.ChannelID <= 0 || rule.RetryChannelID <= 0 {
			return nil, common.NewMessage("Rule {{index}}: choose the channel and the retry channel", position)
		}
		if rule.ChannelID == rule.RetryChannelID {
			return nil, common.NewMessage("Rule {{index}}: the retry channel must differ from the channel", position)
		}
		keywords := make([]string, 0, len(rule.Keywords))
		for _, keyword := range rule.Keywords {
			if keyword = strings.TrimSpace(keyword); keyword != "" {
				keywords = append(keywords, keyword)
			}
		}
		if len(keywords) == 0 {
			return nil, common.NewMessage("Rule {{index}}: enter at least one keyword", position)
		}
		rule.Keywords = keywords
	}
	if rules == nil {
		rules = []ErrorKeywordRetryRule{}
	}
	return rules, nil
}

// ValidateErrorKeywordRetryOption rejects a rules value the relay could not
// apply.
func ValidateErrorKeywordRetryOption(key string, value string) error {
	if key != ErrorKeywordRetryRulesOptionKey {
		return nil
	}
	_, err := ParseErrorKeywordRetryRules(value)
	return err
}
