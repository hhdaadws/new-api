package middleware

import (
	"regexp"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"

	"github.com/gin-gonic/gin"
)

// regexCache caches compiled regex patterns to avoid recompilation on every request.
var regexCache sync.Map

func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := regexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.Store(pattern, compiled)
	return compiled, nil
}

// CheckHeaderAudit validates incoming request headers against the channel's audit rules.
// Returns ("", true) if all checks pass, or (error message, false) if a check fails.
func CheckHeaderAudit(c *gin.Context) (string, bool) {
	settingVal, exists := common.GetContextKey(c, constant.ContextKeyChannelSetting)
	if !exists {
		return "", true
	}
	setting, ok := settingVal.(dto.ChannelSettings)
	if !ok {
		return "", true
	}
	if !setting.HeaderAuditEnabled {
		return "", true
	}
	if len(setting.HeaderAuditRules) == 0 {
		return "", true
	}

	for headerName, pattern := range setting.HeaderAuditRules {
		headerValue := c.Request.Header.Get(headerName)
		if headerValue == "" {
			return i18n.T(c, i18n.MsgChannelHeaderAuditMissing, map[string]any{"Header": headerName}), false
		}

		re, err := getCompiledRegex(pattern)
		if err != nil {
			// Should not happen since we validate at save time, but handle gracefully
			return i18n.T(c, i18n.MsgChannelHeaderAuditInvalidRex, map[string]any{"Header": headerName, "Error": err.Error()}), false
		}

		if !re.MatchString(headerValue) {
			return i18n.T(c, i18n.MsgChannelHeaderAuditMismatch, map[string]any{"Header": headerName}), false
		}
	}

	return "", true
}
