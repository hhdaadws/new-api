package controller

import (
	"cmp"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func channelHasSensitiveChanges(channel *PatchChannel, origin *model.Channel, requestData map[string]any) bool {
	if _, ok := requestData["type"]; ok && channel.Type != origin.Type {
		return true
	}
	if _, ok := requestData["key"]; ok && channel.Key != "" && channel.Key != origin.Key {
		return true
	}
	if _, ok := requestData["base_url"]; ok && !equalStringPtr(channel.BaseURL, origin.BaseURL) {
		return true
	}
	if _, ok := requestData["openai_organization"]; ok && !equalStringPtr(channel.OpenAIOrganization, origin.OpenAIOrganization) {
		return true
	}
	if _, ok := requestData["header_override"]; ok && !equalStringPtr(channel.HeaderOverride, origin.HeaderOverride) {
		return true
	}
	if _, ok := requestData["param_override"]; ok && !equalStringPtr(channel.ParamOverride, origin.ParamOverride) {
		return true
	}
	if _, ok := requestData["setting"]; ok && !equalStringPtr(channel.Setting, origin.Setting) {
		return true
	}
	if _, ok := requestData["other"]; ok && channel.Other != origin.Other {
		return true
	}
	if _, ok := requestData["settings"]; ok && channel.OtherSettings != origin.OtherSettings {
		return true
	}
	if _, ok := requestData["key_mode"]; ok && channel.KeyMode != nil {
		return true
	}
	// Fail closed: any field present in the request that is neither a known
	// sensitive field (gated above) nor an explicitly classified non-sensitive
	// field must be treated as sensitive. This keeps a newly added channel field
	// from silently becoming editable by ChannelWrite-only admins until it is
	// consciously classified in channelNonSensitiveFields.
	for field := range requestData {
		if _, ok := channelSensitiveFields[field]; ok {
			continue
		}
		if _, ok := channelNonSensitiveFields[field]; ok {
			continue
		}
		if _, ok := channelOperationalFields[field]; ok {
			continue
		}
		if _, ok := channelReadOnlyFields[field]; ok {
			continue
		}
		if _, ok := channelRootOnlyFields[field]; ok {
			continue
		}
		return true
	}
	return false
}

// channelSensitiveFields lists the channel fields whose modification requires
// ChannelSensitiveWrite. They are each checked individually in
// channelHasSensitiveChanges with a precise old-vs-new comparison; this set is
// used to exclude them from the fail-closed scan for unknown fields.
var channelSensitiveFields = map[string]struct{}{
	"type":                {},
	"key":                 {},
	"base_url":            {},
	"openai_organization": {},
	"header_override":     {},
	"param_override":      {},
	"setting":             {},
	"other":               {},
	"settings":            {},
	"key_mode":            {},
}

// channelOperationalFields lists fields managed by operation endpoints instead
// of the general channel edit endpoint.
var channelOperationalFields = map[string]struct{}{
	"status": {},
}

// channelReadOnlyFields lists server-managed/accounting fields that the general
// channel edit endpoint must ignore even if a client sends them.
var channelReadOnlyFields = map[string]struct{}{
	"created_time":         {},
	"test_time":            {},
	"response_time":        {},
	"balance":              {},
	"balance_updated_time": {},
	"used_quota":           {},
}

func clearChannelReadOnlyFields(channel *PatchChannel, requestData map[string]any) {
	if _, ok := requestData["created_time"]; ok {
		channel.CreatedTime = 0
	}
	if _, ok := requestData["test_time"]; ok {
		channel.TestTime = 0
	}
	if _, ok := requestData["response_time"]; ok {
		channel.ResponseTime = 0
	}
	if _, ok := requestData["balance"]; ok {
		channel.Balance = 0
	}
	if _, ok := requestData["balance_updated_time"]; ok {
		channel.BalanceUpdatedTime = 0
	}
	if _, ok := requestData["used_quota"]; ok {
		channel.UsedQuota = 0
	}
}

// channelNonSensitiveFields lists routing / server-managed channel
// fields a ChannelWrite admin may edit without ChannelSensitiveWrite. When a new
// field is added to model.Channel it must be added to either this set or
// channelSensitiveFields or channelOperationalFields; otherwise it falls through
// to the fail-closed branch and is treated as sensitive. The
// TestChannelFieldsAreClassified guard test enforces this.
var channelNonSensitiveFields = map[string]struct{}{
	"id":                  {},
	"test_model":          {},
	"name":                {},
	"weight":              {},
	"models":              {},
	"group":               {},
	"model_mapping":       {},
	"status_code_mapping": {},
	"priority":            {},
	"auto_ban":            {},
	"other_info":          {},
	"tag":                 {},
	"remark":              {},
	"channel_info":        {},
	"multi_key_mode":      {},
}

// channelRootOnlyFields lists channel fields only root may change. They are
// enforced separately in UpdateChannel, so the fail-closed scan skips them.
var channelRootOnlyFields = map[string]struct{}{
	"alias": {},
}

// channelViewPolicy describes which channel identity fields the current
// administrator may see. Root always sees everything.
type channelViewPolicy struct {
	RealName bool
	BaseURL  bool
}

func channelViewPolicyOf(c *gin.Context) channelViewPolicy {
	userID, role := c.GetInt("id"), c.GetInt("role")
	return channelViewPolicy{
		RealName: authz.Can(userID, role, authz.ChannelNameView),
		BaseURL:  authz.Can(userID, role, authz.ChannelBaseURLView),
	}
}

func (p channelViewPolicy) hiddenFields() model.ChannelHiddenFields {
	return model.ChannelHiddenFields{Name: !p.RealName, BaseURL: !p.BaseURL}
}

// sortBy drops a requested sort column whose order would reveal hidden values.
func (p channelViewPolicy) sortBy(requested string) string {
	if !p.RealName && strings.EqualFold(strings.TrimSpace(requested), "name") {
		return ""
	}
	return requested
}

// channelDisplayName is the name shown in place of a hidden real channel name:
// the alias set by root, or a localized "Channel #id" when there is none.
func channelDisplayName(c *gin.Context, id int, alias string) string {
	if alias != "" {
		return alias
	}
	return i18n.T(c, i18n.MsgChannelAliasFallback, map[string]any{"Id": id})
}

// nameOf is the channel name the administrator may see, without changing the
// channel itself.
func (p channelViewPolicy) nameOf(c *gin.Context, channel *model.Channel) string {
	if p.RealName {
		return channel.Name
	}
	return channelDisplayName(c, channel.Id, lo.FromPtr(channel.Alias))
}

// redact removes the fields the administrator may not see from a channel that
// is about to be returned.
func (p channelViewPolicy) redact(c *gin.Context, channel *model.Channel) {
	if channel == nil {
		return
	}
	if !p.RealName {
		channel.Name = p.nameOf(c, channel)
		channel.Alias = nil
	}
	if !p.BaseURL {
		channel.BaseURL = nil
	}
}

// displayNames maps channel ids to the names the administrator may see, or
// returns nil when real names are visible and nothing needs replacing.
func (p channelViewPolicy) displayNames(c *gin.Context, ids []int) (map[int]string, error) {
	if p.RealName {
		return nil, nil
	}
	aliases, err := model.GetChannelAliases(ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(ids))
	for _, id := range ids {
		names[id] = channelDisplayName(c, id, aliases[id])
	}
	return names, nil
}

// redactBoundChannels replaces the channel names listed on model metadata when
// the administrator may not see real channel names.
func (p channelViewPolicy) redactBoundChannels(c *gin.Context, models []*model.Model) error {
	if p.RealName {
		return nil
	}
	var ids []int
	for _, metadata := range models {
		if metadata == nil {
			continue
		}
		for _, channel := range metadata.BoundChannels {
			ids = append(ids, channel.Id)
		}
	}
	names, err := p.displayNames(c, ids)
	if err != nil {
		return err
	}
	for _, metadata := range models {
		if metadata == nil {
			continue
		}
		for i := range metadata.BoundChannels {
			metadata.BoundChannels[i].Name = names[metadata.BoundChannels[i].Id]
		}
		slices.SortFunc(metadata.BoundChannels, func(a, b model.BoundChannel) int {
			return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Type, b.Type))
		})
	}
	return nil
}
