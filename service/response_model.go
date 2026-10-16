package service

import (
	"bytes"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// responseModelPaths are where relay payloads name a model: OpenAI, Claude and
// Responses objects and chunks, the response inside Responses stream events,
// Claude message_start, realtime session events, and Gemini's modelVersion.
var responseModelPaths = []string{"model", "response.model", "message.model", "session.model", "modelVersion"}

// RewriteResponseModel names the model the client requested in a payload sent
// to the client, so model mapping and upstream versions never reach downstream.
// Upstream declarations stay available to admins through ResponseModel, which
// handlers observe before writing.
func RewriteResponseModel(c *gin.Context, data []byte) []byte {
	return ReplaceResponseModel(data, common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
}

// ReplaceResponseModel sets every model path present in a JSON object payload
// to model. Other payloads ([DONE], audio, arrays) are returned unchanged, and
// data itself is never modified, so callers may keep using it.
func ReplaceResponseModel(data []byte, model string) []byte {
	if model == "" || !bytes.Contains(data, []byte(`"model`)) {
		return data
	}
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return data
	}
	for _, path := range responseModelPaths {
		value := gjson.GetBytes(data, path)
		if value.Type != gjson.String || value.Str == model {
			continue
		}
		if updated, err := sjson.SetBytes(data, path, model); err == nil {
			data = updated
		}
	}
	return data
}
