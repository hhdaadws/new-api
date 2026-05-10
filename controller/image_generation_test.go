package controller

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndValidateImageGenerationRequestAcceptsPresetSizes(t *testing.T) {
	for _, size := range imageGenerationSizes {
		t.Run(size, func(t *testing.T) {
			req := newValidImageGenerationPageRequest()
			req.Size = size

			err := normalizeAndValidateImageGenerationRequest(req)

			require.NoError(t, err)
			require.Equal(t, size, req.Size)
		})
	}
}

func TestNormalizeAndValidateImageGenerationRequestAcceptsCustomSizes(t *testing.T) {
	tests := []string{
		"2048x1152",
		"1152x2048",
		"3000x1000",
		"3840x1280",
	}

	for _, size := range tests {
		t.Run(size, func(t *testing.T) {
			req := newValidImageGenerationPageRequest()
			req.Size = size

			err := normalizeAndValidateImageGenerationRequest(req)

			require.NoError(t, err)
			require.Equal(t, size, req.Size)
		})
	}
}

func TestNormalizeAndValidateImageGenerationRequestRejectsInvalidSizes(t *testing.T) {
	tests := []string{
		"4096x1024",
		"3840x1279",
		"1024×1024",
		"abc",
		"0x1024",
		"1024x0",
		"1024X1024",
	}

	for _, size := range tests {
		t.Run(size, func(t *testing.T) {
			req := newValidImageGenerationPageRequest()
			req.Size = size

			err := normalizeAndValidateImageGenerationRequest(req)

			require.Error(t, err)
		})
	}
}

func TestReplaceImageGenerationRequestBodyForwardsCompressionOnlyWhenNeeded(t *testing.T) {
	tests := []struct {
		name            string
		outputFormat    string
		compression     int
		wantCompression bool
	}{
		{
			name:            "jpeg zero is preserved",
			outputFormat:    "jpeg",
			compression:     0,
			wantCompression: true,
		},
		{
			name:            "webp zero is preserved",
			outputFormat:    "webp",
			compression:     0,
			wantCompression: true,
		},
		{
			name:         "jpeg 100 is omitted",
			outputFormat: "jpeg",
			compression:  100,
		},
		{
			name:         "png compression is omitted",
			outputFormat: "png",
			compression:  50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/images/generations", nil)
			req := newValidImageGenerationPageRequest()
			req.OutputFormat = tt.outputFormat
			req.OutputCompression = &tt.compression

			err := replaceImageGenerationRequestBody(ctx, req)

			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.UnmarshalBodyReusable(ctx, &payload))
			value, ok := payload["output_compression"]
			require.Equal(t, tt.wantCompression, ok)
			if tt.wantCompression {
				require.Equal(t, float64(tt.compression), value)
			}
		})
	}
}

func TestSetupImageGenerationRelayContextUsesSelectedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 123)
	req := newValidImageGenerationPageRequest()
	req.Group = "image-group"

	err := setupImageGenerationRelayContext(ctx, req)

	require.NoError(t, err)
	require.Equal(t, "image-group", common.GetContextKeyString(ctx, constant.ContextKeyUsingGroup))
	require.Equal(t, "image-group", common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))
	require.Equal(t, "image-generation-image-group", ctx.GetString("token_name"))
}

func TestFillImageEditRequestFromMultipartAcceptsSupportedImageFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name  string
		files map[string][]string
	}{
		{
			name: "single image array field",
			files: map[string][]string{
				"image[]": {"source.png"},
			},
		},
		{
			name: "multiple image array fields",
			files: map[string][]string{
				"image[]": {"source-a.png", "source-b.jpg"},
			},
		},
		{
			name: "repeated standard image fields",
			files: map[string][]string{
				"image": {"source-a.png", "source-b.webp"},
			},
		},
		{
			name: "indexed image fields",
			files: map[string][]string{
				"image[0]": {"source-a.png"},
				"image[1]": {"source-b.png"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newImageEditMultipartContext(t, tt.files)
			req := &imageGenerationPageRequest{}

			err := fillImageEditRequestFromMultipart(ctx, req)

			require.NoError(t, err)
			require.Equal(t, "gpt-image-2", req.Model)
			require.Equal(t, "default", req.Group)
			require.Equal(t, "make it cinematic", req.Prompt)
			require.Equal(t, uint(1), *req.N)
		})
	}
}

func TestFillImageEditRequestFromMultipartRejectsInvalidImageCountsAndTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		files   map[string][]string
		wantErr string
	}{
		{
			name:    "requires at least one image",
			files:   map[string][]string{},
			wantErr: "请至少上传一张图片",
		},
		{
			name: "rejects more than sixteen images",
			files: map[string][]string{
				"image[]": makeImageFilenames(17),
			},
			wantErr: "最多支持上传 16 张图片",
		},
		{
			name: "rejects unsupported file type",
			files: map[string][]string{
				"image[]": {"source.gif"},
			},
			wantErr: "仅支持 PNG、JPEG、WebP 图片",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newImageEditMultipartContext(t, tt.files)
			req := &imageGenerationPageRequest{}

			err := fillImageEditRequestFromMultipart(ctx, req)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func newImageEditMultipartContext(t *testing.T, files map[string][]string) *gin.Context {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("group", "default"))
	require.NoError(t, writer.WriteField("prompt", "make it cinematic"))
	require.NoError(t, writer.WriteField("n", "1"))
	require.NoError(t, writer.WriteField("size", "1024x1024"))
	require.NoError(t, writer.WriteField("quality", "auto"))
	require.NoError(t, writer.WriteField("output_format", "png"))

	for field, filenames := range files {
		for _, filename := range filenames {
			part, err := writer.CreateFormFile(field, filename)
			require.NoError(t, err)
			_, err = part.Write([]byte("fake image bytes"))
			require.NoError(t, err)
		}
	}
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/images/edits", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return ctx
}

func makeImageFilenames(count int) []string {
	filenames := make([]string, 0, count)
	for i := 0; i < count; i++ {
		filenames = append(filenames, fmt.Sprintf("source-%02d.png", i))
	}
	return filenames
}

func newValidImageGenerationPageRequest() *imageGenerationPageRequest {
	return &imageGenerationPageRequest{
		Model:        "gpt-image-2",
		Group:        "default",
		Prompt:       "make it cinematic",
		N:            common.GetPointer(uint(1)),
		Size:         "1024x1024",
		Quality:      "auto",
		OutputFormat: "png",
	}
}
