package gemini

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestMapsSupportedSizesToAspectRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		size          string
		wantRatio     string
		wantImageSize string
	}{
		{name: "landscape preset", size: "2560x1440", wantRatio: "16:9", wantImageSize: "1K"},
		{name: "portrait preset", size: "2160x3840", wantRatio: "9:16", wantImageSize: "1K"},
		{name: "custom 3:1", size: "3072x1024", wantRatio: "3:1", wantImageSize: "1K"},
		{name: "custom 1:3", size: "1280x3840", wantRatio: "1:3", wantImageSize: "1K"},
		{name: "custom 16:9", size: "2048x1152", wantRatio: "16:9", wantImageSize: "1K"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: "imagen-3.0-generate-002",
				},
			}
			request := dto.ImageRequest{
				Model:          "gpt-image-2",
				Prompt:         "a red fox in snowfall",
				Size:           tt.size,
				ResponseFormat: "url",
				N:              uintPtr(2),
				Quality:        "auto",
			}

			got, err := adaptor.ConvertImageRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
			require.NoError(t, err)

			body, err := common.Marshal(got)
			require.NoError(t, err)

			var payload dto.GeminiImageRequest
			require.NoError(t, common.Unmarshal(body, &payload))
			require.Len(t, payload.Instances, 1)
			require.Equal(t, request.Prompt, payload.Instances[0].Prompt)
			require.Equal(t, tt.wantRatio, payload.Parameters.AspectRatio)
			require.Equal(t, tt.wantImageSize, payload.Parameters.ImageSize)
			require.Equal(t, 2, payload.Parameters.SampleCount)
		})
	}
}

func TestConvertImageRequestRejectsUnsupportedAspectRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "imagen-3.0-generate-002",
		},
	}
	request := dto.ImageRequest{
		Model:          "gpt-image-2",
		Prompt:         "a red fox in snowfall",
		Size:           "3850x1000",
		ResponseFormat: "url",
	}

	_, err := adaptor.ConvertImageRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
	require.Error(t, err)
}

func uintPtr(v uint) *uint {
	return &v
}
