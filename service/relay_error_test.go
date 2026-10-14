package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestShouldRetryRelayErrorHonorsChannelPinOnChannelError(t *testing.T) {
	err := types.NewError(errors.New("channel failed"), types.ErrorCodeChannelNoAvailableKey)
	for _, test := range []struct {
		name      string
		pin       *dto.ChannelPin
		wantRetry bool
	}{
		{name: "unrestricted channel error", wantRetry: true},
		{
			name: "single attempt pin suppresses channel error retry",
			pin: &dto.ChannelPin{
				ChannelId: 1, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt,
			},
			wantRetry: false,
		},
		{
			name: "origin task pin permits retry on the same channel",
			pin: &dto.ChannelPin{
				ChannelId: 1, Source: dto.PinSourceOriginTask, Rank: dto.PinRankOriginTask, RetryMode: dto.PinRetrySameChannel,
			},
			wantRetry: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if test.pin != nil {
				GetChannelConstraints(c).AddPin(*test.pin)
			}
			assert.Equal(t, test.wantRetry, ShouldRetryRelayError(c, err, 1))
		})
	}
}

func TestProcessChannelErrorMasksDisableReasonAndNotification(t *testing.T) {
	previousDB, previousType := model.DB, common.MainDatabaseType()
	previousCache, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
	previousAutoDisable, previousErrorLog := common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled
	previousNotifyLimit := constant.NotifyLimitCount
	previousClient, previousWorker := httpClient, system_setting.WorkerUrl
	fetch := system_setting.GetFetchSetting()
	previousFetch := *fetch
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.MemoryCacheEnabled, common.RedisEnabled = previousCache, previousRedis
		common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled = previousAutoDisable, previousErrorLog
		constant.NotifyLimitCount = previousNotifyLimit
		httpClient, system_setting.WorkerUrl = previousClient, previousWorker
		*fetch = previousFetch
	})
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}))
	model.DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled, common.RedisEnabled = false, false
	common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled = true, false
	constant.NotifyLimitCount = 10
	channel := &model.Channel{Name: "relay-review", Key: "fixture-key", Type: 1, Status: common.ChannelStatusEnabled, Group: "default", Models: "test-model"}
	require.NoError(t, channel.Insert())
	notifications := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		notifications <- body
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	httpClient, system_setting.WorkerUrl = server.Client(), ""
	fetch.EnableSSRFProtection = false
	settings, err := common.Marshal(kitdto.UserSetting{NotifyType: kitdto.NotifyTypeWebhook, WebhookUrl: server.URL})
	require.NoError(t, err)
	root := &model.User{Username: "notification-test-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Setting: string(settings)}
	require.NoError(t, database.Create(root).Error)
	notifyKey := fmt.Sprintf("%d:%s:%s", root.Id, formatNotifyType(channel.Id, common.ChannelStatusAutoDisabled), time.Now().Format("2006010215"))
	notifyLimitStore.Delete(notifyKey)
	t.Cleanup(func() { notifyLimitStore.Delete(notifyKey) })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	apiErr := types.NewErrorWithStatusCode(errors.New("upstream https://private.example.com/path?token=review-token api_key:review-secret"), types.ErrorCodeChannelNoAvailableKey, http.StatusUnauthorized)
	ProcessChannelError(c, types.ChannelError{ChannelId: channel.Id, ChannelName: channel.Name, AutoBan: true}, apiErr, nil)
	var notification WebhookPayload
	select {
	case payload := <-notifications:
		require.NoError(t, common.Unmarshal(payload, &notification))
	case <-time.After(5 * time.Second):
		t.Fatal("automatic channel-disable notification was not delivered")
	}
	loaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, loaded.Status)
	wantReason := "status_code=401, upstream https://***.com/***?token=*** api_key:***"
	assert.Equal(t, wantReason, loaded.GetOtherInfo()["status_reason"])
	assert.Contains(t, notification.Content, wantReason)
	assert.NotContains(t, notification.Content, "review-token")
	assert.NotContains(t, notification.Content, "review-secret")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
}

func TestDecideRelayRetryReasons(t *testing.T) {
	upstream := func(status int) *types.NewAPIError {
		return types.NewOpenAIError(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, status)
	}
	for _, tc := range []struct {
		name    string
		err     *types.NewAPIError
		retries int
		setup   func(*gin.Context)
		want    PolicyDecision
	}{
		{name: "retry status matched", err: upstream(http.StatusTooManyRequests), retries: 1, want: PolicyDecision{Action: "retry", Reason: "retry_status_matched", Source: "global"}},
		{name: "status outside retry rules", err: upstream(http.StatusBadRequest), retries: 1, want: PolicyDecision{Action: "stop", Reason: "status_not_retryable", Source: "global"}},
		{name: "attempt budget exhausted", err: upstream(http.StatusTooManyRequests), retries: 0, want: PolicyDecision{Action: "stop", Reason: "attempt_budget_exhausted", Source: "global"}},
		{name: "always skipped status", err: upstream(http.StatusGatewayTimeout), retries: 1, want: PolicyDecision{Action: "stop", Reason: "system_retry_exclusion", Source: "system"}},
		{name: "success status never retries", err: upstream(http.StatusOK), retries: 1, want: PolicyDecision{Action: "stop", Reason: "system_retry_exclusion", Source: "system"}},
		{name: "skip retry error", err: types.NewErrorWithStatusCode(errors.New("local"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry()), retries: 1, want: PolicyDecision{Action: "stop", Reason: "non_retryable_error", Source: "system"}},
		{name: "channel error retries without budget", err: types.NewError(errors.New("no key"), types.ErrorCodeChannelNoAvailableKey), retries: 0, want: PolicyDecision{Action: "retry", Reason: "channel_error", Source: "system"}},
		{name: "single attempt pin", err: upstream(http.StatusTooManyRequests), retries: 1, setup: func(c *gin.Context) {
			GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: 1, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt})
		}, want: PolicyDecision{Action: "stop", Reason: "pinned_channel", Source: "channel_constraint"}},
		{name: "strict session", err: upstream(http.StatusTooManyRequests), retries: 1, setup: func(c *gin.Context) {
			c.Set(ginKeyChannelAffinitySkipRetry, true)
			RequestPolicy(c).SessionModeSource = "global"
		}, want: PolicyDecision{Action: "stop", Reason: "strict_session", Source: "global"}},
		{name: "nil error", retries: 1, want: PolicyDecision{Action: "stop", Reason: "request_completed", Source: "system"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tc.setup != nil {
				tc.setup(c)
			}
			decision := DecideRelayRetry(c, tc.err, tc.retries)
			assert.Equal(t, tc.want, decision)
			assert.Equal(t, tc.want.Action == "retry", ShouldRetryRelayError(c, tc.err, tc.retries))
		})
	}
}

func TestRequestPolicyEventsReachLogAdminInfo(t *testing.T) {
	previousAutoDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = previousAutoDisable })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("auto_ban", true)
	c.Set("channel_id", 7)
	state := RequestPolicy(c)
	state.BeginAttempt(&model.Channel{Id: 7}, "default")
	apiErr := types.NewOpenAIError(errors.New("invalid credential"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized)
	RecordPolicyFailure(c, 7, apiErr, DecideRelayRetry(c, apiErr, 0))

	failed := model.NewLogOther()
	AppendRelayLogAdminInfo(c, nil, failed)
	events, ok := failed.Snapshot()["admin_info"].(map[string]any)["request_policy"].([]PolicyEvent)
	require.True(t, ok, "a failed relay exposes its decision events to administrators")
	require.Len(t, events, 3)
	assert.Equal(t, PolicyDecision{Action: "attempt", Reason: "channel_selected", Source: "routing"}, events[0].Decision)
	assert.Equal(t, "default", events[0].Group)
	assert.Equal(t, PolicyDecision{Action: "failure", Reason: "upstream_failure", Source: "upstream"}, events[1].Decision)
	assert.Equal(t, http.StatusUnauthorized, events[1].Status)
	assert.Equal(t, PolicyDecision{Action: "stop", Reason: "attempt_budget_exhausted", Source: "global"}, events[2].Decision)
	assert.Equal(t, "channel_disable_requested", events[2].Health, "the health entry follows the automatic disable rules")
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	RecordPolicyFailure(c, 7, apiErr, DecideRelayRetry(c, apiErr, 0))
	assert.Equal(t, "key_disable_requested", state.Events()[4].Health)

	state.BeginAttempt(&model.Channel{Id: 8}, "default")
	c.Set("channel_id", 8)
	MarkRequestPolicySuccess(c, nil)
	MarkRequestPolicySuccess(c, nil)
	succeeded := model.NewLogOther()
	AppendRelayLogAdminInfo(c, nil, succeeded)
	events, ok = succeeded.Snapshot()["admin_info"].(map[string]any)["request_policy"].([]PolicyEvent)
	require.True(t, ok, "a successful relay exposes its decision events to administrators")
	require.Len(t, events, 7, "the outcome is recorded once")
	assert.Equal(t, PolicyDecision{Action: "success", Reason: "request_completed", Source: "upstream"}, events[6].Decision)
	assert.Equal(t, 8, events[6].ChannelID)
	assert.Equal(t, 2, events[6].Attempt)
	assert.True(t, state.Successful)

	untouched, _ := gin.CreateTestContext(httptest.NewRecorder())
	other := model.NewLogOther()
	AppendRelayLogAdminInfo(untouched, nil, other)
	assert.NotContains(t, other.Snapshot()["admin_info"], "request_policy", "requests without decisions do not carry an empty record")
}

func TestUpstreamErrorInterception(t *testing.T) {
	setting := operation_setting.GetUpstreamErrorInterceptionSetting()
	saved := *setting
	t.Cleanup(func() { *setting = saved })
	configure := func(enabled, retryOnMatch bool) {
		*setting = operation_setting.UpstreamErrorInterceptionSetting{
			Enabled:      enabled,
			Keywords:     []string{"Upstream Vendor", "quota exhausted"},
			StatusCode:   http.StatusServiceUnavailable,
			Message:      "service busy",
			RetryOnMatch: retryOnMatch,
		}
	}
	upstreamJSON := func(message string, status int) *types.NewAPIError {
		return types.WithOpenAIError(types.OpenAIError{Message: message, Type: "upstream_error", Code: "x"}, status)
	}

	for _, tc := range []struct {
		name    string
		enabled bool
		err     *types.NewAPIError
		want    bool
	}{
		{name: "upstream JSON error matches case-insensitively", enabled: true, err: upstreamJSON("powered by upstream vendor", http.StatusTooManyRequests), want: true},
		{name: "upstream status error matches", enabled: true, err: types.NewOpenAIError(errors.New("Quota Exhausted for key"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden), want: true},
		{name: "no keyword", enabled: true, err: upstreamJSON("rate limited", http.StatusTooManyRequests), want: false},
		{name: "disabled", enabled: false, err: upstreamJSON("upstream vendor", http.StatusTooManyRequests), want: false},
		{name: "gateway local error never matches", enabled: true, err: types.NewError(errors.New("upstream vendor"), types.ErrorCodeInvalidRequest), want: false},
		{name: "channel error never matches", enabled: true, err: types.NewError(errors.New("upstream vendor"), types.ErrorCodeChannelNoAvailableKey), want: false},
		{name: "nil error", enabled: true, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configure(tc.enabled, false)
			assert.Equal(t, tc.want, MatchUpstreamErrorKeyword(tc.err))
		})
	}

	t.Run("replacement carries configured status and message in every client format", func(t *testing.T) {
		configure(true, false)
		intercepted := InterceptUpstreamError(upstreamJSON("upstream vendor overloaded", http.StatusTooManyRequests))
		require.NotNil(t, intercepted)
		assert.Equal(t, http.StatusServiceUnavailable, intercepted.StatusCode)
		assert.Equal(t, types.ErrorCodeBadResponse, intercepted.GetErrorCode())
		assert.Equal(t, "service busy", intercepted.ToOpenAIError().Message)
		assert.Equal(t, "service busy", intercepted.ToClaudeError().Message)
		assert.Nil(t, InterceptUpstreamError(upstreamJSON("rate limited", http.StatusTooManyRequests)))
	})

	t.Run("non-JSON error body surfaces the matched keyword without the body", func(t *testing.T) {
		configure(true, false)
		resp := &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("<html>Upstream Vendor gateway error secret-host</html>"))}
		apiErr := RelayErrorHandler(t.Context(), resp, false)
		assert.Equal(t, "bad response status code 502 (matched keyword: upstream vendor)", apiErr.Error())
		assert.True(t, MatchUpstreamErrorKeyword(apiErr))
	})

	t.Run("error log shows users the replacement and keeps the original for admins", func(t *testing.T) {
		configure(true, false)
		previousLogDB, previousDB, previousErrorLog := model.LOG_DB, model.DB, constant.ErrorLogEnabled
		t.Cleanup(func() {
			model.LOG_DB, model.DB, constant.ErrorLogEnabled = previousLogDB, previousDB, previousErrorLog
		})
		database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := database.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		require.NoError(t, database.AutoMigrate(&model.Log{}, &model.User{}))
		model.LOG_DB, model.DB, constant.ErrorLogEnabled = database, database, true

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		ProcessChannelError(c, types.ChannelError{ChannelId: 7}, upstreamJSON("upstream vendor overloaded", http.StatusTooManyRequests), nil)

		var log model.Log
		require.NoError(t, database.First(&log).Error)
		assert.Equal(t, "status_code=503, service busy", log.Content)
		var other struct {
			StatusCode int `json:"status_code"`
			AdminInfo  struct {
				UpstreamError struct {
					StatusCode int    `json:"status_code"`
					Message    string `json:"message"`
				} `json:"upstream_error"`
			} `json:"admin_info"`
		}
		require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
		assert.Equal(t, http.StatusServiceUnavailable, other.StatusCode)
		assert.Equal(t, http.StatusTooManyRequests, other.AdminInfo.UpstreamError.StatusCode)
		assert.Equal(t, "upstream vendor overloaded", other.AdminInfo.UpstreamError.Message)
	})

	for _, tc := range []struct {
		name         string
		retryOnMatch bool
		want         PolicyDecision
	}{
		{name: "matched error stops retry by default", want: PolicyDecision{Action: "stop", Reason: "upstream_error_keyword", Source: "global"}},
		{name: "matched error follows retry rules when allowed", retryOnMatch: true, want: PolicyDecision{Action: "retry", Reason: "retry_status_matched", Source: "global"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configure(true, tc.retryOnMatch)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			assert.Equal(t, tc.want, DecideRelayRetry(c, upstreamJSON("upstream vendor overloaded", http.StatusTooManyRequests), 1))
		})
	}
}

func TestErrorKeywordRetryRules(t *testing.T) {
	previousDB, previousType, previousOptions := model.DB, common.MainDatabaseType(), common.OptionMap
	previousLogDB, previousErrorLog := model.LOG_DB, constant.ErrorLogEnabled
	previousCache, previousRetries := common.MemoryCacheEnabled, common.RetryTimes
	retrySetting := operation_setting.GetErrorKeywordRetrySetting()
	previousEnabled := retrySetting.Enabled
	previousRules, err := common.Marshal(operation_setting.ErrorKeywordRetryRules())
	require.NoError(t, err)
	t.Cleanup(func() {
		model.DB, common.OptionMap = previousDB, previousOptions
		model.LOG_DB, constant.ErrorLogEnabled = previousLogDB, previousErrorLog
		common.SetMainDatabaseType(previousType)
		common.MemoryCacheEnabled, common.RetryTimes = previousCache, previousRetries
		retrySetting.Enabled = previousEnabled
		operation_setting.LoadErrorKeywordRetryRules(string(previousRules))
	})
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Option{}, &model.Log{}, &model.User{}))
	model.DB, common.OptionMap = database, map[string]string{}
	model.LOG_DB, constant.ErrorLogEnabled = database, true
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled, common.RetryTimes = false, 0
	for _, channel := range []*model.Channel{
		{Id: 1, Name: "primary", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-test"},
		{Id: 2, Name: "backup outside the group", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: "vip", Models: "gpt-test"},
		{Id: 3, Name: "disabled backup", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusManuallyDisabled, Group: "default", Models: "gpt-test"},
		{Id: 4, Name: "backup without the model", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: "default", Models: "other-model"},
	} {
		require.NoError(t, channel.Insert())
	}

	t.Run("saving validates the rules before they reach the relay", func(t *testing.T) {
		for _, tc := range []struct {
			name, value, key string
			index            int
		}{
			{name: "not an array", value: `{}`, key: "Error keyword retry rules must be a JSON array of rules"},
			{name: "blank group", value: `[{"group":" ","channel_id":1,"keywords":["x"],"retry_channel_id":2}]`, key: "Rule {{index}}: choose a group", index: 1},
			{name: "missing retry channel", value: `[{"group":"default","channel_id":1,"keywords":["x"],"retry_channel_id":2},{"group":"default","channel_id":1,"keywords":["x"]}]`, key: "Rule {{index}}: choose the channel and the retry channel", index: 2},
			{name: "retry on the same channel", value: `[{"group":"default","channel_id":1,"keywords":["x"],"retry_channel_id":1}]`, key: "Rule {{index}}: the retry channel must differ from the channel", index: 1},
			{name: "only blank keywords", value: `[{"group":"default","channel_id":1,"keywords":[" ",""],"retry_channel_id":2}]`, key: "Rule {{index}}: enter at least one keyword", index: 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := model.UpdateOption(operation_setting.ErrorKeywordRetryRulesOptionKey, tc.value)
				var message *common.Message
				require.ErrorAs(t, err, &message)
				assert.Equal(t, tc.key, message.Key)
				if tc.index > 0 {
					assert.Equal(t, tc.index, message.Params["index"])
				}
			})
		}
		require.NoError(t, model.UpdateOption("error_keyword_retry.enabled", "true"))
		require.NoError(t, model.UpdateOption(operation_setting.ErrorKeywordRetryRulesOptionKey, `[
			{"group":"default","channel_id":1,"keywords":["Context Length"],"retry_channel_id":3},
			{"group":" default ","channel_id":1,"keywords":[" context length ","","quota"],"retry_channel_id":2},
			{"group":"default","channel_id":1,"keywords":["model busy"],"retry_channel_id":4}
		]`))
		assert.True(t, retrySetting.Enabled)
		rules := operation_setting.ErrorKeywordRetryRules()
		require.Len(t, rules, 3)
		assert.Equal(t, operation_setting.ErrorKeywordRetryRule{Group: "default", ChannelID: 1, Keywords: []string{"context length", "quota"}, RetryChannelID: 2}, rules[1])
	})

	upstream := func(message string, status int, options ...types.NewAPIErrorOptions) *types.NewAPIError {
		return types.WithOpenAIError(types.OpenAIError{Message: message, Type: "invalid_request_error", Code: "upstream_code"}, status, options...)
	}
	for _, tc := range []struct {
		name      string
		group     string
		channelID int
		err       *types.NewAPIError
		setup     func(*gin.Context)
		redirect  bool
	}{
		{name: "keyword in a status outside the retry rules redirects to the first usable retry channel", group: "default", channelID: 1, err: upstream("This model's maximum CONTEXT LENGTH is 8192 tokens", http.StatusBadRequest), redirect: true},
		{name: "keyword in the error code redirects", group: "default", channelID: 1, err: types.WithOpenAIError(types.OpenAIError{Message: "slow down", Type: "requests", Code: "insufficient_quota"}, http.StatusTooManyRequests), redirect: true},
		{name: "another group keeps the retry policy", group: "vip", channelID: 1, err: upstream("context length", http.StatusBadRequest)},
		{name: "another channel keeps the retry policy", group: "default", channelID: 2, err: upstream("context length", http.StatusBadRequest)},
		{name: "error without a keyword keeps the retry policy", group: "default", channelID: 1, err: upstream("bad gateway", http.StatusBadGateway)},
		{name: "retry channel without the model is skipped", group: "default", channelID: 1, err: upstream("model busy", http.StatusServiceUnavailable)},
		{name: "error marked not retryable is never redirected", group: "default", channelID: 1, err: upstream("context length", http.StatusInternalServerError, types.ErrOptionWithSkipRetry())},
		{name: "gateway local error is never redirected", group: "default", channelID: 1, err: types.NewError(errors.New("context length"), types.ErrorCodeInvalidRequest)},
		{name: "pinned channel is never redirected", group: "default", channelID: 1, err: upstream("context length", http.StatusBadRequest), setup: func(c *gin.Context) {
			GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: 1, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt})
		}},
		{name: "strict session is never redirected", group: "default", channelID: 1, err: upstream("context length", http.StatusBadRequest), setup: func(c *gin.Context) {
			c.Set(ginKeyChannelAffinitySkipRetry, true)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tc.setup != nil {
				tc.setup(c)
			}
			param := &RetryParam{Ctx: c, TokenGroup: tc.group, ModelName: "gpt-test", Retry: common.GetPointer(0)}
			decision, redirected := param.RedirectOnErrorKeyword(tc.group, tc.channelID, tc.err)
			param.IncreaseRetry()
			if !tc.redirect {
				assert.False(t, redirected)
				assert.False(t, param.CanAttempt(), "without a redirect the retry budget decides")
				return
			}
			require.True(t, redirected)
			assert.Equal(t, PolicyDecision{Action: "retry", Reason: "error_keyword_matched", Source: "error_keyword_rule"}, decision)
			assert.True(t, param.CanAttempt(), "the redirect runs even when the retry budget is used up")
			channel, group, err := CacheGetRandomSatisfiedChannel(param)
			require.NoError(t, err)
			assert.Equal(t, 2, channel.Id)
			assert.Equal(t, "default", group, "the redirect stays in the group of the failed attempt")

			decision, redirected = param.RedirectOnErrorKeyword(group, channel.Id, upstream("context length", http.StatusTooManyRequests))
			param.IncreaseRetry()
			assert.True(t, redirected)
			assert.Equal(t, PolicyDecision{Action: "stop", Reason: "error_keyword_retry_failed", Source: "error_keyword_rule"}, decision)
			assert.False(t, param.CanAttempt(), "the retry channel's failure ends the request")
		})
	}

	t.Run("users read only the outcome of a redirect and admins read the redirected error in it", func(t *testing.T) {
		original := upstream("This model's maximum context length is 8192 tokens", http.StatusBadRequest)
		wantDetails := map[string]any{
			"channel_id": float64(1), "keyword": "context length", "status_code": float64(http.StatusBadRequest),
			"error_type": "openai_error", "error_code": "upstream_code", "message": "This model's maximum context length is 8192 tokens",
		}
		redirectedRequest := func(t *testing.T) *gin.Context {
			require.NoError(t, database.Where("1 = 1").Delete(&model.Log{}).Error)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			param := &RetryParam{Ctx: c, TokenGroup: "default", ModelName: "gpt-test", Retry: common.GetPointer(0)}
			_, redirected := param.RedirectOnErrorKeyword("default", 1, original)
			require.True(t, redirected)
			ProcessChannelError(c, types.ChannelError{ChannelId: 1}, original, nil)
			return c
		}
		adminDetails := func(t *testing.T, other string) any {
			var parsed struct {
				AdminInfo map[string]any `json:"admin_info"`
			}
			require.NoError(t, common.UnmarshalJsonStr(other, &parsed))
			return parsed.AdminInfo["error_keyword_retry"]
		}

		t.Run("a failed retry logs only its own error", func(t *testing.T) {
			c := redirectedRequest(t)
			ProcessChannelError(c, types.ChannelError{ChannelId: 2}, upstream("quota exceeded", http.StatusTooManyRequests), nil)
			WriteHeldErrorLog(c)

			var logs []model.Log
			require.NoError(t, database.Find(&logs).Error)
			require.Len(t, logs, 1)
			assert.Equal(t, 2, logs[0].ChannelId)
			assert.Equal(t, "status_code=429, quota exceeded", logs[0].Content)
			assert.Equal(t, wantDetails, adminDetails(t, logs[0].Other))

			userLogs, _, err := model.GetUserLogs(0, model.LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "")
			require.NoError(t, err)
			require.Len(t, userLogs, 1)
			assert.NotContains(t, userLogs[0].Other, "context length", "users never read the redirected error")
		})

		t.Run("a successful retry carries the redirected error in its log", func(t *testing.T) {
			c := redirectedRequest(t)
			other := model.NewLogOther()
			AppendRelayLogAdminInfo(c, nil, other)
			WriteHeldErrorLog(c)

			assert.Equal(t, wantDetails, adminDetails(t, other.JSONString()))
			var count int64
			require.NoError(t, database.Model(&model.Log{}).Count(&count).Error)
			assert.Zero(t, count, "no error log is written for the redirected attempt")
		})

		t.Run("a request that ends without a later log writes the held error log once", func(t *testing.T) {
			c := redirectedRequest(t)
			WriteHeldErrorLog(c)
			WriteHeldErrorLog(c)

			var logs []model.Log
			require.NoError(t, database.Find(&logs).Error)
			require.Len(t, logs, 1)
			assert.Equal(t, 1, logs[0].ChannelId)
			assert.Equal(t, "status_code=400, This model's maximum context length is 8192 tokens", logs[0].Content)
		})
	})

	t.Run("disabled rules keep the retry policy", func(t *testing.T) {
		require.NoError(t, model.UpdateOption("error_keyword_retry.enabled", "false"))
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		param := &RetryParam{Ctx: c, TokenGroup: "default", ModelName: "gpt-test", Retry: common.GetPointer(0)}
		_, redirected := param.RedirectOnErrorKeyword("default", 1, upstream("context length", http.StatusBadRequest))
		assert.False(t, redirected)
	})
}
