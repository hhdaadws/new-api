package service

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// errorKeywordRetry is the redirect an error keyword retry rule chose for a
// request. The attempt on channel is the last attempt of the request.
type errorKeywordRetry struct {
	channel *model.Channel
	group   string
	started bool
}

// keywordRetryLog holds the error log of an attempt that an error keyword retry
// rule moved to another channel. Users read only the log of the retry attempt,
// so administrators find this attempt's error in the admin details of that log
// rather than in a log of its own.
type keywordRetryLog struct {
	err     *types.NewAPIError
	details map[string]any
	// write records the attempt's own error log. It runs only when the request
	// ends without a later log to carry the details.
	write func()
	done  bool
}

// holdErrorLog keeps an attempt's error log instead of writing it when an error
// keyword retry rule redirected that attempt's error. It reports whether the
// log was kept.
func (s *RequestPolicyState) holdErrorLog(err *types.NewAPIError, write func()) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	held := s.keywordRetryLog
	if held == nil || held.err != err || held.write != nil {
		return false
	}
	held.write = write
	return true
}

// attachHeldErrorLog adds the held error of a redirected attempt to the admin
// details of the log written after it.
func (s *RequestPolicyState) attachHeldErrorLog(other *model.LogOther) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held := s.keywordRetryLog
	if other == nil || held == nil || held.write == nil || held.done {
		return
	}
	held.done = true
	other.SetAdmin("error_keyword_retry", held.details)
}

// WriteHeldErrorLog writes the held error log of a redirected attempt when the
// request ended without a later log to carry it, so the failure is still
// logged. It runs once at the end of each request.
func WriteHeldErrorLog(c *gin.Context) {
	state := RequestPolicy(c)
	state.mu.Lock()
	held := state.keywordRetryLog
	if held == nil || held.write == nil || held.done {
		state.mu.Unlock()
		return
	}
	held.done = true
	state.mu.Unlock()
	held.write()
}

// CanAttempt reports whether a retry loop may run another attempt. After an
// error keyword retry redirect, exactly one more attempt runs, on the rule's
// retry channel, whatever the retry budget allows.
func (p *RetryParam) CanAttempt() bool {
	if p.keywordRetry != nil {
		return !p.keywordRetry.started
	}
	return p.GetRetry() <= common.RetryTimes
}

// RedirectOnErrorKeyword applies the error keyword retry rules to an attempt
// that failed on channelID in group. When it reports true, the returned
// decision replaces the retry policy's decision for that attempt.
//
// The first rule for the group and channel whose keyword appears in the error,
// and whose retry channel can serve the request, sends the next attempt to that
// channel even when the status code rules or the retry budget would stop. That
// attempt is the last one: when it fails, the request stops with its error.
// Rules never override a pinned channel, a strict session, or an error the
// gateway marked as not retryable, such as a response that already reached the
// client. A nil err matches no rule but still ends a redirected request.
//
// The redirected error's log is held for the log of the retry attempt; see
// keywordRetryLog.
func (p *RetryParam) RedirectOnErrorKeyword(group string, channelID int, err *types.NewAPIError) (PolicyDecision, bool) {
	if p.keywordRetry != nil {
		return PolicyDecision{Action: "stop", Reason: "error_keyword_retry_failed", Source: "error_keyword_rule"}, true
	}
	if err == nil || !operation_setting.GetErrorKeywordRetrySetting().Enabled {
		return PolicyDecision{}, false
	}
	if types.IsSkipRetryError(err) || ShouldSkipRetryAfterChannelAffinityFailure(p.Ctx) {
		return PolicyDecision{}, false
	}
	if _, local := gatewayLocalErrorCodes[err.GetErrorCode()]; local {
		return PolicyDecision{}, false
	}
	constraints := GetChannelConstraints(p.Ctx)
	if _, pinned, _ := constraints.ResolvedPin(); pinned {
		return PolicyDecision{}, false
	}
	// Match what the error log shows as well as the raw message and the code.
	text := strings.ToLower(err.ErrorWithStatusCode() + "\n" + err.MaskSensitiveError() + "\n" + string(err.GetErrorCode()))
	for _, rule := range operation_setting.ErrorKeywordRetryRules() {
		if rule.Group != group || rule.ChannelID != channelID {
			continue
		}
		found, words := AcSearch(text, rule.Keywords, true)
		if !found {
			continue
		}
		target, cacheErr := model.CacheGetChannel(rule.RetryChannelID)
		usable := cacheErr == nil && target.Status == common.ChannelStatusEnabled
		if usable {
			// The retry channel may sit outside the group, so check its own
			// model list the way the channel cache indexes it.
			models := target.GetModels()
			normalized := ratio_setting.RoutingMatchModelName(p.ModelName)
			usable = slices.Contains(models, p.ModelName) ||
				(normalized != "" && normalized != p.ModelName && slices.Contains(models, normalized))
		}
		if usable {
			usable, _ = model.ChannelSatisfiesFilters(target, p.ModelName, constraints.Filters)
		}
		if !usable {
			logger.LogWarn(p.Ctx, common.LogText("error keyword retry skipped: retry channel #%d is missing, disabled, or cannot serve model %s", rule.RetryChannelID, p.ModelName))
			continue
		}
		p.keywordRetry = &errorKeywordRetry{channel: target, group: group}
		state := RequestPolicy(p.Ctx)
		state.mu.Lock()
		state.keywordRetryLog = &keywordRetryLog{err: err, details: map[string]any{
			"channel_id":  channelID,
			"keyword":     words[0],
			"status_code": err.StatusCode,
			"error_type":  err.GetErrorType(),
			"error_code":  err.GetErrorCode(),
			"message":     err.MaskSensitiveError(),
		}}
		state.mu.Unlock()
		logger.LogInfo(p.Ctx, common.LogText("error keyword retry: channel #%d error matched %q, retrying on channel #%d", channelID, words[0], target.Id))
		return PolicyDecision{Action: "retry", Reason: "error_keyword_matched", Source: "error_keyword_rule"}, true
	}
	return PolicyDecision{}, false
}
