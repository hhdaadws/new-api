package controller

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
