package helper

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidOpenAIImageRequestMultipartCompressionForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		outputFormat      string
		outputCompression string
		wantCompression   *int
	}{
		{
			name:              "jpeg zero is forwarded",
			outputFormat:      "jpeg",
			outputCompression: "0",
			wantCompression:   intPtr(0),
		},
		{
			name:              "webp below 100 is forwarded",
			outputFormat:      "webp",
			outputCompression: "42",
			wantCompression:   intPtr(42),
		},
		{
			name:              "jpeg 100 is omitted",
			outputFormat:      "jpeg",
			outputCompression: "100",
		},
		{
			name:              "png is omitted",
			outputFormat:      "png",
			outputCompression: "50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMultipartImageEditContext(t, tt.outputFormat, tt.outputCompression)

			request, err := GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesEdits)

			require.NoError(t, err)
			var outputFormat string
			require.NoError(t, common.Unmarshal(request.OutputFormat, &outputFormat))
			require.Equal(t, tt.outputFormat, outputFormat)
			if tt.wantCompression == nil {
				require.Nil(t, request.OutputCompression)
				return
			}
			require.NotNil(t, request.OutputCompression)
			var compression int
			require.NoError(t, common.Unmarshal(request.OutputCompression, &compression))
			require.Equal(t, *tt.wantCompression, compression)
		})
	}
}

func TestGetAndValidOpenAIImageRequestJSONCompressionForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		outputFormat      string
		outputCompression int
		wantCompression   *int
	}{
		{
			name:              "jpeg zero is forwarded",
			outputFormat:      "jpeg",
			outputCompression: 0,
			wantCompression:   intPtr(0),
		},
		{
			name:              "webp below 100 is forwarded",
			outputFormat:      "webp",
			outputCompression: 42,
			wantCompression:   intPtr(42),
		},
		{
			name:              "jpeg 100 is omitted",
			outputFormat:      "jpeg",
			outputCompression: 100,
		},
		{
			name:              "png is omitted",
			outputFormat:      "png",
			outputCompression: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newJSONImageGenerationContext(t, map[string]any{
				"model":              "gpt-image-2",
				"prompt":             "make it cinematic",
				"size":               "1024x1024",
				"quality":            "auto",
				"output_format":      tt.outputFormat,
				"output_compression": tt.outputCompression,
			})

			request, err := GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesGenerations)

			require.NoError(t, err)
			if tt.wantCompression == nil {
				require.Nil(t, request.OutputCompression)
				return
			}
			require.NotNil(t, request.OutputCompression)
			var compression int
			require.NoError(t, common.Unmarshal(request.OutputCompression, &compression))
			require.Equal(t, *tt.wantCompression, compression)
		})
	}
}

func newMultipartImageEditContext(t *testing.T, outputFormat, outputCompression string) *gin.Context {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "make it cinematic"))
	require.NoError(t, writer.WriteField("size", "1024x1024"))
	require.NoError(t, writer.WriteField("quality", "auto"))
	require.NoError(t, writer.WriteField("output_format", outputFormat))
	require.NoError(t, writer.WriteField("output_compression", outputCompression))
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return ctx
}

func newJSONImageGenerationContext(t *testing.T, payload map[string]any) *gin.Context {
	t.Helper()

	body, err := common.Marshal(payload)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func intPtr(v int) *int {
	return &v
}

func TestGetAndValidOpenAIImageRequestMultipartRejectsInvalidCompression(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newMultipartImageEditContext(t, "jpeg", "bad")

	_, err := GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesEdits)

	require.Error(t, err)
}

func TestGetAndValidOpenAIImageRequestJSONRejectsInvalidCompression(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []any{-1, 101, "42", true}
	for _, compression := range tests {
		t.Run(fmt.Sprintf("%v", compression), func(t *testing.T) {
			ctx := newJSONImageGenerationContext(t, map[string]any{
				"model":              "gpt-image-2",
				"prompt":             "make it cinematic",
				"output_format":      "jpeg",
				"output_compression": compression,
			})

			_, err := GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesGenerations)

			require.Error(t, err)
		})
	}
}
