package openai

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestForwardsMultipleEditImagesAsArrayParts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := newOpenAIImageEditContext(t, map[string][]string{
		"image[]": {"source-a.png", "source-b.jpg"},
	})
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	}
	request := dto.ImageRequest{
		Model:  "gpt-image-2",
		Prompt: "combine these references",
	}

	got, err := (&Adaptor{}).ConvertImageRequest(ctx, info, request)

	require.NoError(t, err)
	body, ok := got.(*bytes.Buffer)
	require.True(t, ok)

	reader := multipart.NewReader(bytes.NewReader(body.Bytes()), boundaryFromContentType(t, ctx.Request.Header.Get("Content-Type")))
	form, err := reader.ReadForm(32 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })

	require.Len(t, form.File["image[]"], 2)
	require.Equal(t, "source-a.png", form.File["image[]"][0].Filename)
	require.Equal(t, "source-b.jpg", form.File["image[]"][1].Filename)
	require.Empty(t, form.File["image"])
	require.Equal(t, []string{"combine these references"}, form.Value["prompt"])
}

func newOpenAIImageEditContext(t *testing.T, files map[string][]string) *gin.Context {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "combine these references"))
	for field, filenames := range files {
		for _, filename := range filenames {
			part, err := writer.CreateFormFile(field, filename)
			require.NoError(t, err)
			_, err = io.WriteString(part, "fake image bytes")
			require.NoError(t, err)
		}
	}
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return ctx
}

func boundaryFromContentType(t *testing.T, contentType string) string {
	t.Helper()

	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	return params["boundary"]
}
