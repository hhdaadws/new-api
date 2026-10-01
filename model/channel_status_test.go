package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelStatusTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)

	memoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = memoryCacheEnabled
	})
}

func TestUpdateChannelStatusPersistsMultiKeyState(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "multi-key-status",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:           true,
			MultiKeySize:         2,
			MultiKeyMode:         constant.MultiKeyModePolling,
			MultiKeyPollingIndex: 1,
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	changed := UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "provider rejected key")
	require.True(t, changed)

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.ChannelInfo.MultiKeyStatusList[0])
	assert.Equal(t, "provider rejected key", stored.ChannelInfo.MultiKeyDisabledReason[0])
	assert.NotZero(t, stored.ChannelInfo.MultiKeyDisabledTime[0])
	assert.Equal(t, 1, stored.ChannelInfo.MultiKeyPollingIndex)
}

func TestSaveStatusStateFromSingleKeySnapshotPreservesUnownedColumns(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:        "single-key-status",
		Key:         "original-key",
		Status:      common.ChannelStatusEnabled,
		Models:      "original-model",
		Group:       "default",
		UsedQuota:   100,
		ChannelInfo: ChannelInfo{},
	}
	require.NoError(t, DB.Create(&channel).Error)

	stale, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)

	concurrentChannelInfo := ChannelInfo{
		IsMultiKey:           true,
		MultiKeySize:         2,
		MultiKeyMode:         constant.MultiKeyModePolling,
		MultiKeyPollingIndex: 1,
	}
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"key":          "rotated-key",
		"used_quota":   gorm.Expr("used_quota + ?", 250),
		"models":       "concurrent-model",
		"channel_info": concurrentChannelInfo,
	}).Error)

	stale.Status = common.ChannelStatusManuallyDisabled
	stale.SetOtherInfo(map[string]any{
		"status_reason": "manual operation",
		"status_time":   int64(1234),
	})
	require.NoError(t, stale.saveStatusState())

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, stored.Status)
	assert.Equal(t, "rotated-key", stored.Key)
	assert.Equal(t, int64(350), stored.UsedQuota)
	assert.Equal(t, "concurrent-model", stored.Models)
	assert.Equal(t, concurrentChannelInfo, stored.ChannelInfo)

	otherInfo := stored.GetOtherInfo()
	assert.Equal(t, "manual operation", otherInfo["status_reason"])
	assert.Equal(t, float64(1234), otherInfo["status_time"])
}

func TestSearchChannelsDoesNotMatchHiddenFields(t *testing.T) {
	setupChannelStatusTest(t)

	baseURL, alias := "https://secret-upstream.example", "Line A"
	require.NoError(t, DB.Create(&Channel{Name: "real-upstream", Alias: &alias, Key: "sk-1", Models: "gpt-4", Group: "default", Tag: common.GetPointer("pool"), BaseURL: &baseURL, Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "real-backup", Key: "sk-2", Models: "gpt-4", Group: "default", BaseURL: &baseURL, Status: common.ChannelStatusEnabled}).Error)

	for _, tc := range []struct {
		name     string
		keyword  string
		hidden   ChannelHiddenFields
		channels int
		tags     int
	}{
		{name: "real name matches when visible", keyword: "real", channels: 2, tags: 1},
		{name: "base URL matches when visible", keyword: "secret-upstream", channels: 2, tags: 1},
		{name: "hidden name is not searchable", keyword: "real", hidden: ChannelHiddenFields{Name: true}, channels: 0, tags: 0},
		{name: "hidden name is searched by alias", keyword: "Line", hidden: ChannelHiddenFields{Name: true}, channels: 1, tags: 1},
		{name: "hidden base URL is not searchable", keyword: "secret-upstream", hidden: ChannelHiddenFields{BaseURL: true}, channels: 0, tags: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channels, err := SearchChannels(tc.keyword, "", "", true, tc.hidden)
			require.NoError(t, err)
			assert.Len(t, channels, tc.channels)
			tags, err := SearchTags(tc.keyword, "", "", true, tc.hidden)
			require.NoError(t, err)
			assert.Len(t, tags, tc.tags)
		})
	}
}
