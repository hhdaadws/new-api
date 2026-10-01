package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelHasSensitiveChanges(t *testing.T) {
	baseURL := "https://api.example.com"
	headerOverride := `{"Authorization":"Bearer {api_key}"}`
	origin := &model.Channel{
		Type:           1,
		Key:            "old-key",
		BaseURL:        &baseURL,
		HeaderOverride: &headerOverride,
		Models:         "gpt-4o",
		Group:          "default",
	}

	t.Run("non-sensitive routing fields", func(t *testing.T) {
		updated := PatchChannel{Channel: *origin}
		updated.Models = "gpt-4o,gpt-4o-mini"
		updated.Group = "vip"

		assert.False(t, channelHasSensitiveChanges(&updated, origin, map[string]any{
			"models": updated.Models,
			"group":  updated.Group,
		}))
	})

	t.Run("key change", func(t *testing.T) {
		updated := PatchChannel{Channel: *origin}
		updated.Key = "new-key"

		assert.True(t, channelHasSensitiveChanges(&updated, origin, map[string]any{"key": updated.Key}))
	})

	t.Run("base url change", func(t *testing.T) {
		updated := PatchChannel{Channel: *origin}
		newBaseURL := "https://leak.example.com"
		updated.BaseURL = &newBaseURL

		assert.True(t, channelHasSensitiveChanges(&updated, origin, map[string]any{"base_url": newBaseURL}))
	})

	t.Run("header override change", func(t *testing.T) {
		updated := PatchChannel{Channel: *origin}
		newHeaderOverride := `{"X-Key":"{api_key}"}`
		updated.HeaderOverride = &newHeaderOverride

		assert.True(t, channelHasSensitiveChanges(&updated, origin, map[string]any{"header_override": newHeaderOverride}))
	})

	t.Run("omitted sensitive fields do not use zero values", func(t *testing.T) {
		updated := PatchChannel{}
		updated.Id = origin.Id
		updated.Priority = origin.Priority

		assert.False(t, channelHasSensitiveChanges(&updated, origin, map[string]any{"priority": 10}))
	})

	t.Run("unknown field fails closed", func(t *testing.T) {
		updated := PatchChannel{Channel: *origin}

		assert.True(t, channelHasSensitiveChanges(&updated, origin, map[string]any{"future_secret_field": "x"}))
	})

	t.Run("status is operational", func(t *testing.T) {
		updated := PatchChannel{Channel: *origin}
		updated.Status = common.ChannelStatusManuallyDisabled

		assert.False(t, channelHasSensitiveChanges(&updated, origin, map[string]any{"status": updated.Status}))
	})

	t.Run("read-only fields are ignored by sensitivity check", func(t *testing.T) {
		updated := PatchChannel{Channel: *origin}
		updated.Balance = 99
		updated.UsedQuota = 100
		updated.ResponseTime = 200

		assert.False(t, channelHasSensitiveChanges(&updated, origin, map[string]any{
			"balance":       updated.Balance,
			"used_quota":    updated.UsedQuota,
			"response_time": updated.ResponseTime,
		}))
	})
}

func TestClearChannelReadOnlyFields(t *testing.T) {
	channel := PatchChannel{Channel: model.Channel{
		CreatedTime:        11,
		TestTime:           22,
		ResponseTime:       33,
		Balance:            44.5,
		BalanceUpdatedTime: 55,
		UsedQuota:          66,
		Models:             "gpt-4o",
		Group:              "default",
	}}

	clearChannelReadOnlyFields(&channel, map[string]any{
		"created_time":         channel.CreatedTime,
		"test_time":            channel.TestTime,
		"response_time":        channel.ResponseTime,
		"balance":              channel.Balance,
		"balance_updated_time": channel.BalanceUpdatedTime,
		"used_quota":           channel.UsedQuota,
		"models":               channel.Models,
		"group":                channel.Group,
	})

	assert.Zero(t, channel.CreatedTime)
	assert.Zero(t, channel.TestTime)
	assert.Zero(t, channel.ResponseTime)
	assert.Zero(t, channel.Balance)
	assert.Zero(t, channel.BalanceUpdatedTime)
	assert.Zero(t, channel.UsedQuota)
	assert.Equal(t, "gpt-4o", channel.Models)
	assert.Equal(t, "default", channel.Group)
}

func TestUpdateChannelRejectsStatusField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/channel/",
		bytes.NewBufferString(`{"id":1,"status":2}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}

func TestChannelStatusValidation(t *testing.T) {
	assert.True(t, isManageableChannelStatus(common.ChannelStatusEnabled))
	assert.True(t, isManageableChannelStatus(common.ChannelStatusManuallyDisabled))
	assert.False(t, isManageableChannelStatus(common.ChannelStatusAutoDisabled))
	assert.False(t, isManageableChannelStatus(0))
}

// TestChannelFieldsAreClassified guards the fail-closed sensitivity check: every
// JSON field of PatchChannel (including the embedded model.Channel) must be listed
// in channelSensitiveFields, channelNonSensitiveFields, or
// channelOperationalFields. A newly added field that is left unclassified will
// fail this test, forcing a conscious permission decision instead of silently
// defaulting either way.
func TestChannelFieldsAreClassified(t *testing.T) {
	classified := func(name string) bool {
		if _, ok := channelSensitiveFields[name]; ok {
			return true
		}
		if _, ok := channelNonSensitiveFields[name]; ok {
			return true
		}
		if _, ok := channelOperationalFields[name]; ok {
			return true
		}
		if _, ok := channelReadOnlyFields[name]; ok {
			return true
		}
		_, ok := channelRootOnlyFields[name]
		return ok
	}

	var collect func(rt reflect.Type) []string
	collect = func(rt reflect.Type) []string {
		var names []string
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				names = append(names, collect(field.Type)...)
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			names = append(names, name)
		}
		return names
	}

	for _, name := range collect(reflect.TypeFor[PatchChannel]()) {
		assert.Truef(t, classified(name),
			"channel field %q is not classified; add it to channelSensitiveFields, channelNonSensitiveFields, channelOperationalFields, or channelReadOnlyFields in channel_authz.go", name)
	}
}

func TestChannelViewPolicyHidesRealNameAndBaseURL(t *testing.T) {
	setupTaskPluginBindChannelTest(t)
	require.NoError(t, i18n.Init())
	const restrictedAdmin, plainAdmin, root = 2, 3, 1
	require.NoError(t, authz.SetUserPermissions(restrictedAdmin, authz.PermissionsMap{authz.ResourceChannel: {
		authz.ActionNameView: false, authz.ActionBaseURLView: false, authz.ActionSensitiveWrite: true,
	}}))

	baseURL, alias := "https://secret-upstream.example", "Line A"
	aliased := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "real-upstream", Alias: &alias, Models: "gpt-4", Group: "default", Key: "sk-1", BaseURL: &baseURL}
	require.NoError(t, aliased.Insert())
	unaliased := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "real-backup", Models: "gpt-4", Group: "default", Key: "sk-2", BaseURL: &baseURL}
	require.NoError(t, unaliased.Insert())

	call := func(handler gin.HandlerFunc, userID, role int, method, target, body string, params gin.Params) string {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Set("id", userID)
		context.Set("role", role)
		context.Params = params
		context.Request = httptest.NewRequest(method, target, strings.NewReader(body))
		context.Request.Header.Set("Content-Type", "application/json")
		context.Request.Header.Set("Accept-Language", "en")
		handler(context)
		return recorder.Body.String()
	}
	idParam := func(id int) gin.Params { return gin.Params{{Key: "id", Value: strconv.Itoa(id)}} }

	t.Run("restricted admin sees alias or channel id and no base URL", func(t *testing.T) {
		list := call(GetAllChannels, restrictedAdmin, common.RoleAdminUser, http.MethodGet, "/api/channel/?p=1&page_size=10&sort_by=name", "", nil)
		assert.Contains(t, list, `"name":"Line A"`)
		assert.Contains(t, list, fmt.Sprintf(`"name":"Channel #%d"`, unaliased.Id))
		assert.NotContains(t, list, "real-")
		assert.NotContains(t, list, "secret-upstream")

		detail := call(GetChannel, restrictedAdmin, common.RoleAdminUser, http.MethodGet, "/api/channel/1", "", idParam(aliased.Id))
		assert.Contains(t, detail, `"name":"Line A"`)
		assert.Contains(t, detail, `"alias":null`)
		assert.Contains(t, detail, `"base_url":null`)
	})

	t.Run("saving redacted values keeps the stored name and base URL", func(t *testing.T) {
		body := fmt.Sprintf(`{"id":%d,"type":1,"name":"Line A","base_url":"","models":"gpt-4,gpt-4o","group":"default"}`, aliased.Id)
		result := call(UpdateChannel, restrictedAdmin, common.RoleAdminUser, http.MethodPut, "/api/channel", body, nil)
		assert.Contains(t, result, `"success":true`)
		assert.NotContains(t, result, "real-upstream")
		stored, err := model.GetChannelById(aliased.Id, true)
		require.NoError(t, err)
		assert.Equal(t, "real-upstream", stored.Name)
		assert.Equal(t, baseURL, lo.FromPtr(stored.BaseURL))
		assert.Equal(t, "gpt-4,gpt-4o", stored.Models)
		assert.Equal(t, "Line A", lo.FromPtr(stored.Alias))
	})

	t.Run("only root changes the alias", func(t *testing.T) {
		body := fmt.Sprintf(`{"id":%d,"type":1,"name":"real-upstream","alias":"Line B","models":"gpt-4,gpt-4o","group":"default"}`, aliased.Id)
		denied := call(UpdateChannel, plainAdmin, common.RoleAdminUser, http.MethodPut, "/api/channel", body, nil)
		assert.Contains(t, denied, `"success":false`)
		allowed := call(UpdateChannel, root, common.RoleRootUser, http.MethodPut, "/api/channel", body, nil)
		assert.Contains(t, allowed, `"success":true`)
		stored, err := model.GetChannelById(aliased.Id, true)
		require.NoError(t, err)
		assert.Equal(t, "Line B", lo.FromPtr(stored.Alias))
	})
}
