package channel

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newChannelTimeoutContext(t *testing.T, setting dto.ChannelSettings) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "req-1")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)
	return c, recorder
}

func TestChannelRequestTimeoutAppliesOnlyToItsStreamMode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setting dto.ChannelSettings
		stream  bool
	}{
		{name: "no timeout", setting: dto.ChannelSettings{}},
		{name: "stream ignores the total timeout", setting: dto.ChannelSettings{RequestTimeoutSeconds: 1}, stream: true},
		{name: "non-stream ignores the first token timeout", setting: dto.ChannelSettings{StreamFirstTokenTimeoutSeconds: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newChannelTimeoutContext(t, tc.setting)
			info := &relaycommon.RelayInfo{IsStream: tc.stream}

			timeout := helper.StartChannelRequestTimeout(c, info)
			assert.Nil(t, timeout)

			original := types.NewError(assert.AnError, types.ErrorCodeBadResponse)
			assert.Same(t, original, timeout.Finish(c, info, original))
		})
	}
}

func TestChannelRequestTimeoutCutsUpstreamWithRetryableConfiguredError(t *testing.T) {
	previousAutoDisable, previousKeywords := common.AutomaticDisableChannelEnabled, operation_setting.AutomaticDisableKeywords
	common.AutomaticDisableChannelEnabled, operation_setting.AutomaticDisableKeywords = true, []string{"too slow"}
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled, operation_setting.AutomaticDisableKeywords = previousAutoDisable, previousKeywords
	})
	service.InitHttpClient()
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer upstream.Close()
	defer close(release)

	setting := dto.ChannelSettings{RequestTimeoutSeconds: 1, RequestTimeoutStatusCode: http.StatusGatewayTimeout, RequestTimeoutMessage: "upstream too slow"}
	c, _ := newChannelTimeoutContext(t, setting)
	clientCtx := c.Request.Context()
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: setting}}

	timeout := helper.StartChannelRequestTimeout(c, info)
	require.NotNil(t, timeout)
	req, err := http.NewRequest(http.MethodPost, upstream.URL, bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	resp, err := doRequest(c, req, info)
	require.Error(t, err)
	assert.Nil(t, resp)

	apiErr := timeout.Finish(c, info, types.NewError(err, types.ErrorCodeDoRequestFailed))
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusGatewayTimeout, apiErr.StatusCode)
	assert.Equal(t, "upstream too slow", apiErr.Error())
	assert.Equal(t, types.ErrorCodeUpstreamTimeout, apiErr.GetErrorCode())
	assert.False(t, types.IsSkipRetryError(apiErr))
	assert.False(t, service.MatchUpstreamErrorKeyword(apiErr))
	assert.False(t, service.ShouldDisableChannel(apiErr), "a slow channel is never disabled")
	assert.Equal(t, clientCtx, c.Request.Context())
	assert.NoError(t, c.Request.Context().Err())
}

func TestChannelRequestTimeoutWaitsForFirstStreamData(t *testing.T) {
	general := operation_setting.GetGeneralSetting()
	previousPing, previousInterval := general.PingIntervalEnabled, general.PingIntervalSeconds
	general.PingIntervalEnabled, general.PingIntervalSeconds = true, 1
	t.Cleanup(func() { general.PingIntervalEnabled, general.PingIntervalSeconds = previousPing, previousInterval })
	service.InitHttpClient()

	t.Run("an upstream without stream data in time is retryable and the client got nothing", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, ": processing\n\nevent: message_start\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		defer upstream.Close()
		// The timeout outlasts a ping interval, so a ping would have reached the client.
		setting := dto.ChannelSettings{StreamFirstTokenTimeoutSeconds: 2}
		c, recorder := newChannelTimeoutContext(t, setting)
		info := &relaycommon.RelayInfo{IsStream: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: setting}}

		timeout := helper.StartChannelRequestTimeout(c, info)
		require.NotNil(t, timeout)
		req, err := http.NewRequest(http.MethodPost, upstream.URL, bytes.NewReader([]byte("{}")))
		require.NoError(t, err)
		resp, err := doRequest(c, req, info)
		require.Error(t, err)
		assert.Nil(t, resp)

		apiErr := timeout.Finish(c, info, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError))
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		assert.Equal(t, "bad response", apiErr.Error())
		assert.Equal(t, types.ErrorCodeUpstreamTimeout, apiErr.GetErrorCode())
		assert.False(t, types.IsSkipRetryError(apiErr))
		assert.False(t, c.Writer.Written())
		assert.Zero(t, recorder.Body.Len())
	})

	t.Run("a stream with data in time runs past the timeout intact", func(t *testing.T) {
		const head = ": processing\n\ndata: {\"n\":1}\n\n"
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, head)
			w.(http.Flusher).Flush()
			select {
			case <-time.After(1500 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		}))
		defer upstream.Close()
		setting := dto.ChannelSettings{StreamFirstTokenTimeoutSeconds: 1}
		c, _ := newChannelTimeoutContext(t, setting)
		info := &relaycommon.RelayInfo{IsStream: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: setting}}

		timeout := helper.StartChannelRequestTimeout(c, info)
		require.NotNil(t, timeout)
		req, err := http.NewRequest(http.MethodPost, upstream.URL, bytes.NewReader([]byte("{}")))
		require.NoError(t, err)
		resp, err := doRequest(c, req, info)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, head+"data: [DONE]\n\n", string(body))
		assert.Nil(t, timeout.Finish(c, info, nil))
	})
}

func TestChannelRequestTimeoutEndsStartedStreamWithErrorEvent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		format types.RelayFormat
		event  string
	}{
		{name: "openai", format: types.RelayFormatOpenAI, event: `data: {"error":{"message":"bad response (request id: req-1)","type":"new_api_error","param":"","code":"upstream_timeout"}}`},
		{name: "claude", format: types.RelayFormatClaude, event: "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"new_api_error\",\"message\":\"bad response (request id: req-1)\"}}"},
		{name: "responses", format: types.RelayFormatOpenAIResponses, event: "event: error\ndata: {\"type\":\"error\",\"code\":\"upstream_timeout\",\"message\":\"bad response (request id: req-1)\"}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, recorder := newChannelTimeoutContext(t, dto.ChannelSettings{RequestTimeoutSeconds: 1})
			info := &relaycommon.RelayInfo{RelayFormat: tc.format, StreamStatus: relaycommon.NewStreamStatus()}

			timeout := helper.StartChannelRequestTimeout(c, info)
			require.NotNil(t, timeout)
			// The upstream answered the non-stream request with a stream.
			info.IsStream = true
			require.NoError(t, helper.StringData(c, `{"partial":true}`))
			<-c.Request.Context().Done()
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())

			assert.Nil(t, timeout.Finish(c, info, nil))
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `{"partial":true}`)
			assert.Contains(t, recorder.Body.String(), tc.event)
		})
	}
}
