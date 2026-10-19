package helper

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// States of the first stream data event a stream attempt waits for.
const (
	firstResponsePending int32 = iota
	firstResponseReceived
	firstResponseTimedOut
)

// ChannelRequestTimeout bounds one relay attempt on a channel that configures
// a request timeout.
//
// A non-stream attempt must finish within request_timeout_seconds. The request
// context seen by relay handlers and the upstream request share one deadline,
// so the upstream call and the response relay stop together when it fires.
//
// A stream attempt must receive its first stream data event from upstream
// within stream_first_token_timeout_seconds; after that the stream runs on
// without this limit. Until then the client receives nothing, keep-alive pings
// included, so a timed-out stream can still move to another channel.
//
// A timeout that fires before the response reached the client is retryable.
type ChannelRequestTimeout struct {
	setting        dto.ChannelSettings
	stream         bool
	seconds        int
	deadline       time.Time
	clientCtx      context.Context
	upstreamCtx    context.Context
	cancelRequest  context.CancelFunc
	cancelUpstream context.CancelFunc
	// watchdog cancels the upstream request of a stream attempt whose first
	// stream data event is late. It starts with WatchFirstResponse.
	watchdog      *time.Timer
	firstResponse atomic.Int32
}

// StartChannelRequestTimeout arms the request timeout of the channel selected
// for the current attempt. It returns nil when the channel has none for the
// request's stream mode.
func StartChannelRequestTimeout(c *gin.Context, info *relaycommon.RelayInfo) *ChannelRequestTimeout {
	if c.Request == nil {
		return nil
	}
	setting, _ := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	seconds := setting.RequestTimeoutSeconds
	if info.IsStream {
		seconds = setting.StreamFirstTokenTimeoutSeconds
	}
	if seconds <= 0 {
		return nil
	}
	t := &ChannelRequestTimeout{
		setting:   setting,
		stream:    info.IsStream,
		seconds:   seconds,
		deadline:  time.Now().Add(time.Duration(seconds) * time.Second),
		clientCtx: c.Request.Context(),
	}
	// Upstream requests do not follow client cancellation, so they only take
	// the timeout.
	if t.stream {
		t.upstreamCtx, t.cancelUpstream = context.WithCancel(context.Background())
	} else {
		requestCtx, cancelRequest := context.WithDeadline(t.clientCtx, t.deadline)
		t.cancelRequest = cancelRequest
		c.Request = c.Request.WithContext(requestCtx)
		t.upstreamCtx, t.cancelUpstream = context.WithDeadline(context.Background(), t.deadline)
	}
	common.SetContextKey(c, constant.ContextKeyChannelRequestTimeout, t)
	return t
}

// UpstreamContext is the context of the attempt's upstream HTTP requests.
func (t *ChannelRequestTimeout) UpstreamContext() context.Context {
	return t.upstreamCtx
}

// WatchFirstResponse starts waiting for the first stream data event of the
// upstream request about to be sent. It reports whether the caller must pass
// that request's response to AwaitFirstResponse and keep the client response
// untouched, pings included, until then.
func (t *ChannelRequestTimeout) WatchFirstResponse() bool {
	if t == nil || !t.stream || t.watchdog != nil {
		return false
	}
	t.watchdog = time.AfterFunc(time.Until(t.deadline), t.expireFirstResponse)
	return true
}

func (t *ChannelRequestTimeout) expireFirstResponse() {
	if t.firstResponse.CompareAndSwap(firstResponsePending, firstResponseTimedOut) {
		t.cancelUpstream()
	}
}

// AwaitFirstResponse reads resp up to its first stream data event, the event
// the first token time of logs is measured to, and leaves everything it read
// in resp.Body for the stream handler. SSE comments and field lines such as
// event: do not count; a body that is not SSE counts from its first line. It
// returns an error when the timeout fired first.
func (t *ChannelRequestTimeout) AwaitFirstResponse(resp *http.Response) error {
	reader := bufio.NewReaderSize(resp.Body, InitialScannerBufferSize)
	var peeked []byte
	for {
		line, err := reader.ReadSlice('\n')
		peeked = append(peeked, line...)
		if errors.Is(err, bufio.ErrBufferFull) {
			// A line longer than the buffer carries data.
			break
		}
		content := bytes.TrimSpace(line)
		if data, ok := bytes.CutPrefix(content, []byte("data:")); ok {
			content = bytes.TrimSpace(data)
		} else if bytes.HasPrefix(content, []byte(":")) || bytes.HasPrefix(content, []byte("event:")) ||
			bytes.HasPrefix(content, []byte("id:")) || bytes.HasPrefix(content, []byte("retry:")) {
			content = nil
		}
		// A read error, end of body included, is left for the stream handler.
		if len(content) > 0 || err != nil {
			break
		}
	}
	t.watchdog.Stop()
	if !t.firstResponse.CompareAndSwap(firstResponsePending, firstResponseReceived) {
		_ = resp.Body.Close()
		return fmt.Errorf("no stream data within %ds", t.seconds)
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(peeked), reader), resp.Body}
	return nil
}

// Finish disarms the timeout and, when it fired, replaces the attempt result
// with the channel's timeout error. The error is retryable unless part of the
// response already reached the client. A non-stream request whose upstream
// streamed anyway keeps the status code it already sent, so it ends with an
// error event instead.
func (t *ChannelRequestTimeout) Finish(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) *types.NewAPIError {
	if t == nil {
		return apiErr
	}
	var timedOut bool
	if t.stream {
		if t.watchdog != nil {
			t.watchdog.Stop()
		}
		timedOut = t.firstResponse.Load() == firstResponseTimedOut
	} else {
		// Both contexts expire on their own timers, so judge by the deadline
		// rather than by whichever context happened to fire first.
		timedOut = !time.Now().Before(t.deadline)
		t.cancelRequest()
		c.Request = c.Request.WithContext(t.clientCtx)
	}
	t.cancelUpstream()
	common.SetContextKey(c, constant.ContextKeyChannelRequestTimeout, nil)
	if !timedOut {
		return apiErr
	}

	streamCut := info.IsStream && c.Writer.Written() && (info.StreamStatus == nil || !info.StreamStatus.IsNormalEnd())
	if apiErr == nil && !streamCut {
		// The attempt completed right at the deadline.
		return nil
	}

	if t.stream {
		logger.LogWarn(c, common.LogText("channel request timeout: no stream data within %ds, attempt error: %v", t.seconds, apiErr))
	} else {
		logger.LogWarn(c, common.LogText("channel request timeout: not finished within %ds, attempt error: %v", t.seconds, apiErr))
	}
	statusCode, message := t.setting.RequestTimeoutResponse()
	if apiErr != nil {
		var options []types.NewAPIErrorOptions
		if c.Writer.Written() {
			// The client already has part of the response, so another channel
			// cannot answer it.
			options = append(options, types.ErrOptionWithSkipRetry())
		}
		return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeUpstreamTimeout, statusCode, options...)
	}

	// The partial stream was already settled by its handler; only tell the
	// client why it stopped.
	timeoutErr := types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeUpstreamTimeout, statusCode, types.ErrOptionWithSkipRetry())
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
