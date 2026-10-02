package channel

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
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

func TestChannelRequestTimeoutDisabledByDefault(t *testing.T) {
	c, _ := newChannelTimeoutContext(t, dto.ChannelSettings{})

	timeout := helper.StartChannelRequestTimeout(c)
	assert.Nil(t, timeout)

	original := types.NewError(assert.AnError, types.ErrorCodeBadResponse)
	assert.Same(t, original, timeout.Finish(c, &relaycommon.RelayInfo{}, original))
}

func TestChannelRequestTimeoutCutsUpstreamWithConfiguredError(t *testing.T) {
	t.Parallel()
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

	timeout := helper.StartChannelRequestTimeout(c)
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
	assert.True(t, types.IsSkipRetryError(apiErr))
	assert.False(t, service.MatchUpstreamErrorKeyword(apiErr))
	assert.Equal(t, clientCtx, c.Request.Context())
	assert.NoError(t, c.Request.Context().Err())
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
			info := &relaycommon.RelayInfo{IsStream: true, RelayFormat: tc.format, StreamStatus: relaycommon.NewStreamStatus()}

			timeout := helper.StartChannelRequestTimeout(c)
			require.NotNil(t, timeout)
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
