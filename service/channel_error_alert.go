package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

// pushPlusSendURL is a variable so tests can point it at a local server.
var pushPlusSendURL = "https://www.pushplus.plus/send"

const (
	pushPlusRequestTimeout     = 10 * time.Second
	channelErrorAlertWindowTTL = 2 * time.Minute
	channelErrorAlertReasonMax = 500
)

// channelErrorAlertWindowScript counts one error in the channel's current
// one-minute window and sets the window's expiry on its first error.
const channelErrorAlertWindowScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`

// channelErrorAlertCounter is the single-instance fallback used when Redis is
// disabled or unreachable. Windows are aligned to clock minutes, as with Redis.
type channelErrorAlertCounter struct {
	mutex      sync.Mutex
	windows    map[int]channelErrorAlertWindow
	lastAlerts map[int]time.Time
}

type channelErrorAlertWindow struct {
	start time.Time
	count int
}

var memoryChannelErrorAlertCounter = &channelErrorAlertCounter{
	windows:    make(map[int]channelErrorAlertWindow),
	lastAlerts: make(map[int]time.Time),
}

// record counts one error of the channel at now. It reports true only for the
// error that first exceeds the threshold within its minute, and only when the
// channel's previous alert is older than the cooldown.
func (counter *channelErrorAlertCounter) record(channelId int, now time.Time, threshold int, cooldown time.Duration) bool {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()

	start := now.Truncate(time.Minute)
	window := counter.windows[channelId]
	if !window.start.Equal(start) {
		window = channelErrorAlertWindow{start: start}
	}
	window.count++
	counter.windows[channelId] = window
	if window.count != threshold+1 {
		return false
	}
	if last, ok := counter.lastAlerts[channelId]; ok && now.Sub(last) < cooldown {
		return false
	}
	counter.lastAlerts[channelId] = now
	return true
}

// shouldSendChannelErrorAlert applies the per-minute threshold and cooldown
// across all instances through Redis, falling back to this instance's counts.
func shouldSendChannelErrorAlert(channelId int, now time.Time, threshold int, cooldown time.Duration) bool {
	if !common.RedisEnabled || common.RDB == nil {
		return memoryChannelErrorAlertCounter.record(channelId, now, threshold, cooldown)
	}
	ctx := context.Background()
	countKey := fmt.Sprintf("channel_error_alert:count:%d:%d", channelId, now.Unix()/60)
	count, err := common.RDB.Eval(ctx, channelErrorAlertWindowScript, []string{countKey}, int(channelErrorAlertWindowTTL.Seconds())).Int()
	if err != nil {
		common.SysError(common.LogText("failed to count channel errors in Redis for channel #%d: %s", channelId, err.Error()))
		return memoryChannelErrorAlertCounter.record(channelId, now, threshold, cooldown)
	}
	if count != threshold+1 {
		return false
	}
	if cooldown <= 0 {
		return true
	}
	acquired, err := common.RDB.SetNX(ctx, fmt.Sprintf("channel_error_alert:cooldown:%d", channelId), now.Unix(), cooldown).Result()
	if err != nil {
		common.SysError(common.LogText("failed to check channel error alert cooldown in Redis for channel #%d: %s", channelId, err.Error()))
		return false
	}
	return acquired
}

// RecordChannelErrorForAlert counts one error of the channel and, when the
// channel exceeds the per-minute threshold, sends a PushPlus alert to every
// configured token. Counting and sending run in the background so a failed
// attempt is not delayed before its retry.
func RecordChannelErrorForAlert(channelError types.ChannelError, err *types.NewAPIError) {
	alertSetting := operation_setting.GetChannelErrorAlertSetting()
	if err == nil || !alertSetting.Enabled || len(alertSetting.PushPlusTokens) == 0 {
		return
	}
	tokens := slices.Clone(alertSetting.PushPlusTokens)
	threshold := max(alertSetting.Threshold, 1)
	cooldown := time.Duration(alertSetting.CooldownMinutes) * time.Minute
	reason := err.MaskSensitiveErrorWithStatusCode()
	channelId, channelName := channelError.ChannelId, channelError.ChannelName
	gopool.Go(func() {
		now := time.Now()
		if !shouldSendChannelErrorAlert(channelId, now, threshold, cooldown) {
			return
		}
		if runes := []rune(reason); len(runes) > channelErrorAlertReasonMax {
			reason = string(runes[:channelErrorAlertReasonMax]) + "..."
		}
		lang := RootUserLanguage()
		params := map[string]any{
			"Name":      channelName,
			"Id":        channelId,
			"Threshold": threshold,
			"Reason":    reason,
			"Time":      now.Format("2006-01-02 15:04:05"),
		}
		title := i18n.Translate(lang, i18n.MsgChannelErrorAlertSubject, params)
		content := i18n.Translate(lang, i18n.MsgChannelErrorAlertContent, params)
		common.SysLog(common.LogText("channel %q (#%d) exceeded %d errors within one minute, sending PushPlus alerts to %d recipients", channelName, channelId, threshold, len(tokens)))
		for _, result := range SendPushPlusToTokens(tokens, title, content) {
			if result.Error != "" {
				common.SysError(common.LogText("failed to send channel error alert to PushPlus token %s: %s", result.Token, result.Error))
			}
		}
	})
}

// PushPlusSendResult reports the outcome for one recipient. Token is masked.
type PushPlusSendResult struct {
	Token   string `json:"token"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// SendPushPlusToTokens sends the message to each token in turn, so one
// recipient's failure does not stop the others.
func SendPushPlusToTokens(tokens []string, title string, content string) []PushPlusSendResult {
	results := make([]PushPlusSendResult, 0, len(tokens))
	for _, token := range tokens {
		masked := "****"
		if len(token) >= 12 {
			masked = token[:4] + "****" + token[len(token)-4:]
		}
		result := PushPlusSendResult{Token: masked, Success: true}
		if err := sendPushPlusMessage(token, title, content); err != nil {
			result.Success = false
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

// sendPushPlusMessage posts a plain-text message to the PushPlus account that
// owns token. PushPlus answers HTTP 200 even when it rejects a request, so
// only a body code of 200 counts as accepted.
func sendPushPlusMessage(token string, title string, content string) error {
	payload, err := common.Marshal(map[string]string{
		"token":    token,
		"title":    title,
		"content":  content,
		"template": "txt",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal PushPlus payload: %w", err)
	}

	var resp *http.Response
	if system_setting.EnableWorker() {
		resp, err = DoWorkerRequest(&WorkerRequest{
			URL:     pushPlusSendURL,
			Key:     system_setting.WorkerValidKey,
			Method:  http.MethodPost,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    payload,
		})
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), pushPlusRequestTimeout)
		defer cancel()
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, pushPlusSendURL, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("failed to create PushPlus request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		// The endpoint is fixed rather than user supplied, so it skips the
		// SSRF dialer, which rejects fake-IP DNS answers from local proxies.
		resp, err = GetHttpClient().Do(req)
	}
	if err != nil {
		return fmt.Errorf("failed to send PushPlus request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PushPlus request failed with status code: %d", resp.StatusCode)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data any    `json:"data"`
	}
	if err := common.DecodeJson(io.LimitReader(resp.Body, 64<<10), &result); err != nil {
		return fmt.Errorf("failed to decode PushPlus response: %w", err)
	}
	if result.Code != http.StatusOK {
		return fmt.Errorf("PushPlus rejected the message: code=%d, msg=%s, data=%v", result.Code, result.Msg, result.Data)
	}
	return nil
}
