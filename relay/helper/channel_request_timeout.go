package helper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// ChannelRequestTimeout bounds one relay attempt on a channel that configures
// request_timeout_seconds. The request context seen by relay handlers and the
// upstream request share one deadline, so the upstream call, stream relay and
// keep-alive pings all stop together when it fires.
type ChannelRequestTimeout struct {
	setting        dto.ChannelSettings
	deadline       time.Time
	clientCtx      context.Context
	upstreamCtx    context.Context
	cancelRequest  context.CancelFunc
	cancelUpstream context.CancelFunc
}

// StartChannelRequestTimeout arms the request timeout of the channel selected
// for the current attempt. It returns nil when the channel has none.
func StartChannelRequestTimeout(c *gin.Context) *ChannelRequestTimeout {
	setting, _ := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	if setting.RequestTimeoutSeconds <= 0 || c.Request == nil {
		return nil
	}
	deadline := time.Now().Add(time.Duration(setting.RequestTimeoutSeconds) * time.Second)
	t := &ChannelRequestTimeout{setting: setting, deadline: deadline, clientCtx: c.Request.Context()}
	requestCtx, cancelRequest := context.WithDeadline(t.clientCtx, deadline)
	t.cancelRequest = cancelRequest
	// Upstream requests do not follow client cancellation, so they only take
	// the deadline.
	t.upstreamCtx, t.cancelUpstream = context.WithDeadline(context.Background(), deadline)
	c.Request = c.Request.WithContext(requestCtx)
	common.SetContextKey(c, constant.ContextKeyChannelUpstreamContext, t.upstreamCtx)
	return t
}

// Finish disarms the timeout and, when it fired, replaces the attempt result
// with the channel's configured timeout error. A stream that already reached
// the client keeps its status code, so it ends with an error event instead.
func (t *ChannelRequestTimeout) Finish(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) *types.NewAPIError {
	if t == nil {
		return apiErr
	}
	// Both contexts expire on their own timers, so judge by the deadline
	// rather than by whichever context happened to fire first.
	timedOut := !time.Now().Before(t.deadline)
	t.cancelRequest()
	t.cancelUpstream()
	c.Request = c.Request.WithContext(t.clientCtx)
	common.SetContextKey(c, constant.ContextKeyChannelUpstreamContext, nil)
	if !timedOut {
		return apiErr
	}

	streamCut := info.IsStream && c.Writer.Written() && (info.StreamStatus == nil || !info.StreamStatus.IsNormalEnd())
	if apiErr == nil && !streamCut {
		// The attempt completed right at the deadline.
		return nil
	}

	logger.LogWarn(c, fmt.Sprintf("channel request timeout after %ds, attempt error: %v", t.setting.RequestTimeoutSeconds, apiErr))
	statusCode, message := t.setting.RequestTimeoutResponse()
	timeoutErr := types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeUpstreamTimeout, statusCode, types.ErrOptionWithSkipRetry())
	if apiErr != nil {
		return timeoutErr
	}

	// The partial stream was already settled by its handler; only tell the
	// client why it stopped.
	timeoutErr.SetMessage(common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)))
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		_ = ClaudeData(c, dto.ClaudeResponse{Type: "error", Error: timeoutErr.ToClaudeError()})
	case types.RelayFormatOpenAIResponses:
		event := dto.ResponsesStreamResponse{Type: "error", Code: string(types.ErrorCodeUpstreamTimeout), Message: timeoutErr.Error()}
		data, err := common.Marshal(event)
		if err == nil {
			_ = ResponseChunkData(c, event, string(data))
		}
	default:
		_ = ObjectData(c, gin.H{"error": timeoutErr.ToOpenAIError()})
	}
	return nil
}
