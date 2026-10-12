package operation_setting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// ChannelErrorAlertSetting sends a PushPlus message to every configured token
// when one channel reports more errors within a minute than the threshold.
type ChannelErrorAlertSetting struct {
	Enabled         bool     `json:"enabled"`
	Threshold       int      `json:"threshold"`
	CooldownMinutes int      `json:"cooldown_minutes"`
	PushPlusTokens  []string `json:"pushplus_tokens"`
}

const (
	ChannelErrorAlertThresholdOptionKey = "channel_error_alert.threshold"
	ChannelErrorAlertCooldownOptionKey  = "channel_error_alert.cooldown_minutes"
	// ChannelErrorAlertTokensOptionKey holds recipient credentials; it is
	// never returned to the web console and only root may change it.
	ChannelErrorAlertTokensOptionKey = "channel_error_alert.pushplus_tokens"

	DefaultChannelErrorAlertThreshold       = 10
	DefaultChannelErrorAlertCooldownMinutes = 10
	MaxChannelErrorAlertTokens              = 50
	MaxChannelErrorAlertCooldownMinutes     = 1440
)

var channelErrorAlertSetting = ChannelErrorAlertSetting{
	Enabled:         false,
	Threshold:       DefaultChannelErrorAlertThreshold,
	CooldownMinutes: DefaultChannelErrorAlertCooldownMinutes,
	PushPlusTokens:  []string{},
}

func init() {
	config.GlobalConfig.Register("channel_error_alert", &channelErrorAlertSetting)
}

func GetChannelErrorAlertSetting() *ChannelErrorAlertSetting {
	return &channelErrorAlertSetting
}

// ValidateChannelErrorAlertOption rejects values that would make the alert
// fire on every error or send to malformed recipients.
func ValidateChannelErrorAlertOption(key string, value string) error {
	switch key {
	case ChannelErrorAlertThresholdOptionKey:
		threshold, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || threshold < 1 {
			return fmt.Errorf("channel error alert threshold must be a positive integer")
		}
	case ChannelErrorAlertCooldownOptionKey:
		minutes, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || minutes < 0 || minutes > MaxChannelErrorAlertCooldownMinutes {
			return fmt.Errorf("channel error alert cooldown must be between 0 and %d minutes", MaxChannelErrorAlertCooldownMinutes)
		}
	case ChannelErrorAlertTokensOptionKey:
		var tokens []string
		if err := common.UnmarshalJsonStr(value, &tokens); err != nil {
			return fmt.Errorf("PushPlus tokens must be a JSON array of strings")
		}
		if len(tokens) > MaxChannelErrorAlertTokens {
			return fmt.Errorf("at most %d PushPlus tokens are allowed", MaxChannelErrorAlertTokens)
		}
		seen := make(map[string]struct{}, len(tokens))
		for _, token := range tokens {
			if token == "" || strings.TrimSpace(token) != token || strings.ContainsAny(token, " \t\r\n") {
				return fmt.Errorf("PushPlus tokens must be non-empty and must not contain whitespace")
			}
			if _, ok := seen[token]; ok {
				return fmt.Errorf("PushPlus tokens must not contain duplicates")
			}
			seen[token] = struct{}{}
		}
	}
	return nil
}
