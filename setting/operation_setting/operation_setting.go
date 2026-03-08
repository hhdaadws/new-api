package operation_setting

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = []string{}
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableKeywords = append(AutomaticDisableKeywords, k)
		}
	}
}

var AutomaticDisableErrorCodes = []string{
	"invalid_api_key",
	"account_deactivated",
	"billing_not_active",
	"pre_consume_token_quota_failed",
	"arrearage",
}

func AutomaticDisableErrorCodesToString() string {
	return strings.Join(AutomaticDisableErrorCodes, "\n")
}

func AutomaticDisableErrorCodesFromString(s string) {
	AutomaticDisableErrorCodes = []string{}
	parts := strings.Split(s, "\n")
	for _, k := range parts {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableErrorCodes = append(AutomaticDisableErrorCodes, k)
		}
	}
}

func ShouldDisableByErrorCode(code string) bool {
	code = strings.ToLower(code)
	for _, c := range AutomaticDisableErrorCodes {
		if c == code {
			return true
		}
	}
	return false
}

var AutomaticDisableErrorTypes = []string{
	"insufficient_quota",
	"insufficient_user_quota",
	"authentication_error",
	"permission_error",
	"forbidden",
}

func AutomaticDisableErrorTypesToString() string {
	return strings.Join(AutomaticDisableErrorTypes, "\n")
}

func AutomaticDisableErrorTypesFromString(s string) {
	AutomaticDisableErrorTypes = []string{}
	parts := strings.Split(s, "\n")
	for _, k := range parts {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableErrorTypes = append(AutomaticDisableErrorTypes, k)
		}
	}
}

func ShouldDisableByErrorType(typ string) bool {
	typ = strings.ToLower(typ)
	for _, t := range AutomaticDisableErrorTypes {
		if t == typ {
			return true
		}
	}
	return false
}
