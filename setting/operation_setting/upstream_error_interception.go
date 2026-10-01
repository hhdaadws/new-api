package operation_setting

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// UpstreamErrorInterceptionSetting replaces upstream error responses that
// contain any configured keyword with a fixed status code and message before
// they reach the client.
type UpstreamErrorInterceptionSetting struct {
	Enabled      bool     `json:"enabled"`
	Keywords     []string `json:"keywords"`
	StatusCode   int      `json:"status_code"`
	Message      string   `json:"message"`
	RetryOnMatch bool     `json:"retry_on_match"`
}

const (
	UpstreamErrorInterceptionStatusCodeOptionKey = "upstream_error_interception.status_code"
	UpstreamErrorInterceptionMessageOptionKey    = "upstream_error_interception.message"

	DefaultUpstreamErrorInterceptionStatusCode = http.StatusBadGateway
	DefaultUpstreamErrorInterceptionMessage    = "bad response"
)

var upstreamErrorInterceptionSetting = UpstreamErrorInterceptionSetting{
	Enabled:      false,
	Keywords:     []string{},
	StatusCode:   DefaultUpstreamErrorInterceptionStatusCode,
	Message:      DefaultUpstreamErrorInterceptionMessage,
	RetryOnMatch: false,
}

func init() {
	config.GlobalConfig.Register("upstream_error_interception", &upstreamErrorInterceptionSetting)
}

func GetUpstreamErrorInterceptionSetting() *UpstreamErrorInterceptionSetting {
	return &upstreamErrorInterceptionSetting
}

// ValidateUpstreamErrorInterceptionOption rejects values that would make the
// intercepted response unusable for clients.
func ValidateUpstreamErrorInterceptionOption(key string, value string) error {
	switch key {
	case UpstreamErrorInterceptionStatusCodeOptionKey:
		code, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || code < 400 || code > 599 {
			return fmt.Errorf("upstream error interception status code must be between 400 and 599")
		}
	case UpstreamErrorInterceptionMessageOptionKey:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("upstream error interception message must not be empty")
		}
	}
	return nil
}
