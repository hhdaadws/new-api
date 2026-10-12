package service

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type pushPlusRequest struct {
	path string
	body map[string]string
}

// usePushPlusServer points PushPlus sends at a local server that accepts only
// the given tokens, answering HTTP 200 with a rejection code otherwise as the
// real service does.
func usePushPlusServer(t *testing.T, acceptedTokens ...string) <-chan pushPlusRequest {
	t.Helper()
	requests := make(chan pushPlusRequest, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]string
		_ = common.Unmarshal(raw, &body)
		requests <- pushPlusRequest{path: r.URL.RequestURI(), body: body}
		w.Header().Set("Content-Type", "application/json")
		for _, token := range acceptedTokens {
			if body["token"] == token {
				_, _ = w.Write([]byte(`{"code":200,"msg":"请求成功","data":"3cbc5eab19fe512e80677540fbde332a"}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"code":999,"msg":"服务端验证错误","data":"用户token不存在"}`))
	}))
	t.Cleanup(server.Close)

	previousURL, previousClient, previousWorker := pushPlusSendURL, httpClient, system_setting.WorkerUrl
	t.Cleanup(func() {
		pushPlusSendURL, httpClient, system_setting.WorkerUrl = previousURL, previousClient, previousWorker
	})
	pushPlusSendURL, httpClient, system_setting.WorkerUrl = server.URL+"/send", server.Client(), ""
	return requests
}

func TestChannelErrorAlertCounterFiresOncePerMinuteAfterThreshold(t *testing.T) {
	start := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name     string
		cooldown time.Duration
		// offsets of each recorded error from start, and whether it alerts
		errors []time.Duration
		want   []bool
	}{
		{
			name:     "alerts on the error that exceeds the threshold and not after it",
			cooldown: 0,
			errors:   []time.Duration{0, time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second},
			want:     []bool{false, false, false, true, false},
		},
		{
			name:     "a new minute counts from zero and alerts again without cooldown",
			cooldown: 0,
			errors:   []time.Duration{50 * time.Second, 51 * time.Second, 52 * time.Second, 53 * time.Second, 61 * time.Second, 62 * time.Second, 63 * time.Second, 64 * time.Second},
			want:     []bool{false, false, false, true, false, false, false, true},
		},
		{
			name:     "cooldown suppresses the next minute until it expires",
			cooldown: 2 * time.Minute,
			errors: []time.Duration{
				0, 1 * time.Second, 2 * time.Second, 3 * time.Second,
				60 * time.Second, 61 * time.Second, 62 * time.Second, 63 * time.Second,
				180 * time.Second, 181 * time.Second, 182 * time.Second, 183 * time.Second,
			},
			want: []bool{false, false, false, true, false, false, false, false, false, false, false, true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			counter := &channelErrorAlertCounter{windows: map[int]channelErrorAlertWindow{}, lastAlerts: map[int]time.Time{}}
			got := make([]bool, 0, len(test.errors))
			for _, offset := range test.errors {
				got = append(got, counter.record(7, start.Add(offset), 3, test.cooldown))
			}
			assert.Equal(t, test.want, got)
			assert.False(t, counter.record(8, start, 3, test.cooldown), "another channel keeps its own count")
		})
	}
}

func TestChannelErrorAlertSharesCountsAndCooldownThroughRedis(t *testing.T) {
	previousRedis, previousRDB, previousCounter := common.RedisEnabled, common.RDB, memoryChannelErrorAlertCounter
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		common.RedisEnabled, common.RDB, memoryChannelErrorAlertCounter = previousRedis, previousRDB, previousCounter
	})
	common.RedisEnabled, common.RDB = true, client
	memoryChannelErrorAlertCounter = &channelErrorAlertCounter{windows: map[int]channelErrorAlertWindow{}, lastAlerts: map[int]time.Time{}}

	start := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	recordMinute := func(minute int) []bool {
		got := make([]bool, 0, 4)
		for second := range 4 {
			got = append(got, shouldSendChannelErrorAlert(7, start.Add(time.Duration(minute)*time.Minute+time.Duration(second)*time.Second), 3, 2*time.Minute))
		}
		return got
	}

	assert.Equal(t, []bool{false, false, false, true}, recordMinute(0))
	assert.Equal(t, []bool{false, false, false, false}, recordMinute(1), "the shared cooldown suppresses the next minute")
	server.FastForward(3 * time.Minute)
	assert.Equal(t, []bool{false, false, false, true}, recordMinute(3), "an expired cooldown allows the next alert")
	assert.Empty(t, memoryChannelErrorAlertCounter.windows, "Redis counts must not fall back to this instance")
}

func TestSendPushPlusToTokensReportsEachRecipient(t *testing.T) {
	requests := usePushPlusServer(t, "accepted-token-0001")

	results := SendPushPlusToTokens([]string{"accepted-token-0001", "rejected-token-0002"}, "title", "content")

	require.Len(t, results, 2)
	assert.Equal(t, PushPlusSendResult{Token: "acce****0001", Success: true}, results[0])
	assert.Equal(t, "reje****0002", results[1].Token)
	assert.False(t, results[1].Success)
	assert.Contains(t, results[1].Error, "code=999")
	for _, token := range []string{"accepted-token-0001", "rejected-token-0002"} {
		request := <-requests
		assert.Equal(t, "/send", request.path, "the token must stay out of the URL")
		assert.Equal(t, map[string]string{"token": token, "title": "title", "content": "content", "template": "txt"}, request.body)
	}
}

func TestProcessChannelErrorSendsPushPlusAlertAfterThreshold(t *testing.T) {
	previousDB, previousType := model.DB, common.MainDatabaseType()
	previousRedis, previousAutoDisable, previousErrorLog := common.RedisEnabled, common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled
	previousCounter := memoryChannelErrorAlertCounter
	alertSetting := operation_setting.GetChannelErrorAlertSetting()
	previousSetting := *alertSetting
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled, common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled = previousRedis, previousAutoDisable, previousErrorLog
		memoryChannelErrorAlertCounter = previousCounter
		*alertSetting = previousSetting
	})
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, database.AutoMigrate(&model.User{}))
	settings, err := common.Marshal(map[string]string{"language": "en"})
	require.NoError(t, err)
	require.NoError(t, database.Create(&model.User{Username: "alert-test-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Setting: string(settings)}).Error)
	model.DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled, common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled = false, false, false
	memoryChannelErrorAlertCounter = &channelErrorAlertCounter{windows: map[int]channelErrorAlertWindow{}, lastAlerts: map[int]time.Time{}}
	*alertSetting = operation_setting.ChannelErrorAlertSetting{
		Enabled:         true,
		Threshold:       1,
		CooldownMinutes: 10,
		PushPlusTokens:  []string{"first-person-token", "second-person-token"},
	}
	requests := usePushPlusServer(t, "first-person-token", "second-person-token")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	channelError := types.ChannelError{ChannelId: 42, ChannelName: "primary-openai"}
	apiErr := types.NewErrorWithStatusCode(errors.New("upstream failed with api_key:alert-secret"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	ProcessChannelError(c, channelError, apiErr, nil)
	ProcessChannelError(c, channelError, apiErr, nil)

	recipients := make([]string, 0, 2)
	for range 2 {
		select {
		case request := <-requests:
			recipients = append(recipients, request.body["token"])
			assert.Equal(t, `Channel "primary-openai" (#42) is failing frequently`, request.body["title"])
			assert.True(t, strings.HasPrefix(request.body["content"], `Channel "primary-openai" (#42) exceeded its error threshold (1 per minute).`))
			assert.Contains(t, request.body["content"], "status_code=502")
			assert.NotContains(t, request.body["content"], "alert-secret")
		case <-time.After(5 * time.Second):
			t.Fatal("PushPlus alert was not delivered to every token")
		}
	}
	assert.ElementsMatch(t, []string{"first-person-token", "second-person-token"}, recipients)
}
