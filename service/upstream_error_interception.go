package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// gatewayLocalErrorCodes are produced by the gateway itself rather than read
// from an upstream response, so upstream error interception never rewrites them.
var gatewayLocalErrorCodes = map[types.ErrorCode]struct{}{
	types.ErrorCodeInvalidRequest:             {},
	types.ErrorCodeSensitiveWordsDetected:     {},
	types.ErrorCodeCountTokenFailed:           {},
	types.ErrorCodeModelPriceError:            {},
	types.ErrorCodeInvalidApiType:             {},
	types.ErrorCodeJsonMarshalFailed:          {},
	types.ErrorCodeGetChannelFailed:           {},
	types.ErrorCodeGenRelayInfoFailed:         {},
	types.ErrorCodeReadRequestBodyFailed:      {},
	types.ErrorCodeConvertRequestFailed:       {},
	types.ErrorCodeAccessDenied:               {},
	types.ErrorCodeBadRequestBody:             {},
	types.ErrorCodeModelNotFound:              {},
	types.ErrorCodeQueryDataError:             {},
	types.ErrorCodeUpdateDataError:            {},
	types.ErrorCodeInsufficientUserQuota:      {},
	types.ErrorCodePreConsumeTokenQuotaFailed: {},
	types.ErrorCodeUpstreamTimeout:            {},
}

// matchUpstreamErrorKeywordText reports the first configured keyword found in
// text, or "" when interception is disabled or nothing matches.
func matchUpstreamErrorKeywordText(text string) string {
	setting := operation_setting.GetUpstreamErrorInterceptionSetting()
	if !setting.Enabled || len(setting.Keywords) == 0 || text == "" {
		return ""
	}
	found, words := AcSearch(strings.ToLower(text), setting.Keywords, true)
	if !found || len(words) == 0 {
		return ""
	}
	return words[0]
}

// MatchUpstreamErrorKeyword reports whether an upstream error contains one of
// the configured interception keywords. Gateway-local errors never match.
func MatchUpstreamErrorKeyword(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	code := err.GetErrorCode()
	if types.IsChannelError(err) || IsViolationFeeCode(code) {
		return false
	}
	if _, local := gatewayLocalErrorCodes[code]; local {
		return false
	}
	text := err.Error()
	if message := err.ToOpenAIError().Message; message != text {
		text += "\n" + message
	}
	return matchUpstreamErrorKeywordText(text) != ""
}

// InterceptUpstreamError returns the configured replacement for an upstream
// error that matches an interception keyword, or nil when it does not match.
// The replacement is a fresh error because SetMessage does not change what
// clients see for upstream OpenAI/Claude errors.
func InterceptUpstreamError(err *types.NewAPIError) *types.NewAPIError {
	if !MatchUpstreamErrorKeyword(err) {
		return nil
	}
	setting := operation_setting.GetUpstreamErrorInterceptionSetting()
	statusCode := setting.StatusCode
	if statusCode < 400 || statusCode > 599 {
		statusCode = operation_setting.DefaultUpstreamErrorInterceptionStatusCode
	}
	message := strings.TrimSpace(setting.Message)
	if message == "" {
		message = operation_setting.DefaultUpstreamErrorInterceptionMessage
	}
	return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeBadResponse, statusCode, types.ErrOptionWithSkipRetry())
}
