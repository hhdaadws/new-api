package model

import (
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	// ChannelProbeResultKeep is how many results are kept per probe; older ones
	// are pruned after each run.
	ChannelProbeResultKeep = 200
	// channelProbeTextMaxBytes keeps stored text within a MySQL TEXT column.
	channelProbeTextMaxBytes = 60000
)

// ChannelProbe is an administrator-defined request that is sent to one channel
// on a fixed interval, with its own headers and body, to watch how the
// upstream answers (for example whether it injects a system prompt).
type ChannelProbe struct {
	Id              int    `json:"id"`
	Name            string `json:"name" gorm:"type:varchar(128);not null"`
	ChannelId       int    `json:"channel_id" gorm:"index"`
	EndpointType    string `json:"endpoint_type" gorm:"type:varchar(64)"`
	Headers         string `json:"headers" gorm:"type:text"`
	Body            string `json:"body" gorm:"type:text"`
	IntervalSeconds int    `json:"interval_seconds"`
	Enabled         bool   `json:"enabled" gorm:"index"`
	NextRunAt       int64  `json:"next_run_at" gorm:"bigint;index"`
	LastRunAt       int64  `json:"last_run_at" gorm:"bigint"`
	LastSuccess     bool   `json:"last_success"`
	LastError       string `json:"last_error" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

// ChannelProbeResult is one recorded run of a probe.
type ChannelProbeResult struct {
	Id         int    `json:"id"`
	ProbeId    int    `json:"probe_id" gorm:"index"`
	ChannelId  int    `json:"channel_id"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	Response   string `json:"response" gorm:"type:text"`
	Error      string `json:"error" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
}

// ChannelProbeText makes upstream text storable on every supported database:
// valid UTF-8 without NUL bytes (rejected by PostgreSQL), cut on a rune
// boundary so it fits a MySQL TEXT column.
func ChannelProbeText(text string) string {
	text = strings.ReplaceAll(strings.ToValidUTF8(text, "�"), "\x00", "")
	if len(text) <= channelProbeTextMaxBytes {
		return text
	}
	cut := channelProbeTextMaxBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

func GetAllChannelProbes() ([]*ChannelProbe, error) {
	var probes []*ChannelProbe
	err := DB.Order("id desc").Find(&probes).Error
	return probes, err
}

func GetChannelProbeById(id int) (*ChannelProbe, error) {
	var probe ChannelProbe
	if err := DB.First(&probe, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &probe, nil
}

func (probe *ChannelProbe) Insert() error {
	return DB.Create(probe).Error
}

// Update saves the editable fields and leaves the run state untouched.
func (probe *ChannelProbe) Update() error {
	return DB.Model(probe).Select(
		"name", "channel_id", "endpoint_type", "headers", "body",
		"interval_seconds", "enabled", "next_run_at", "updated_at",
	).Updates(probe).Error
}

func DeleteChannelProbe(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("probe_id = ?", id).Delete(&ChannelProbeResult{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ChannelProbe{}, "id = ?", id).Error
	})
}

func HasDueChannelProbes(now int64) (bool, error) {
	var ids []int
	err := DB.Model(&ChannelProbe{}).
		Where("enabled = ? AND next_run_at <= ?", true, now).
		Limit(1).
		Pluck("id", &ids).Error
	return len(ids) > 0, err
}

// ClaimDueChannelProbes moves every due probe to its next run time and
// returns the probes this caller claimed. The conditional update keeps a probe
// from being claimed twice when claims overlap.
func ClaimDueChannelProbes(now int64) ([]*ChannelProbe, error) {
	var due []*ChannelProbe
	if err := DB.Where("enabled = ? AND next_run_at <= ?", true, now).Order("next_run_at").Find(&due).Error; err != nil {
		return nil, err
	}
	claimed := make([]*ChannelProbe, 0, len(due))
	for _, probe := range due {
		next := now + int64(probe.IntervalSeconds)
		result := DB.Model(&ChannelProbe{}).
			Where("id = ? AND next_run_at = ?", probe.Id, probe.NextRunAt).
			UpdateColumn("next_run_at", next)
		if result.Error != nil {
			return claimed, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		probe.NextRunAt = next
		claimed = append(claimed, probe)
	}
	return claimed, nil
}

// RecordChannelProbeResult stores a run, updates the probe's last run state,
// and prunes results beyond ChannelProbeResultKeep.
func RecordChannelProbeResult(result *ChannelProbeResult) error {
	result.Response = ChannelProbeText(result.Response)
	result.Error = ChannelProbeText(result.Error)
	if result.CreatedAt == 0 {
		result.CreatedAt = common.GetTimestamp()
	}
	if err := DB.Create(result).Error; err != nil {
		return err
	}
	if err := DB.Model(&ChannelProbe{}).Where("id = ?", result.ProbeId).UpdateColumns(map[string]any{
		"last_run_at":  result.CreatedAt,
		"last_success": result.Success,
		"last_error":   result.Error,
	}).Error; err != nil {
		return err
	}
	var cutoff []int
	if err := DB.Model(&ChannelProbeResult{}).
		Where("probe_id = ?", result.ProbeId).
		Order("id desc").
		Offset(ChannelProbeResultKeep).
		Limit(1).
		Pluck("id", &cutoff).Error; err != nil {
		return err
	}
	if len(cutoff) == 0 {
		return nil
	}
	return DB.Where("probe_id = ? AND id <= ?", result.ProbeId, cutoff[0]).Delete(&ChannelProbeResult{}).Error
}

func GetChannelProbeResults(probeId int, offset int, limit int) ([]*ChannelProbeResult, int64, error) {
	var total int64
	query := DB.Model(&ChannelProbeResult{}).Where("probe_id = ?", probeId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var results []*ChannelProbeResult
	err := query.Order("id desc").Offset(offset).Limit(limit).Find(&results).Error
	return results, total, err
}

// GetChannelProbeTargets lists every channel with the fields needed to pick
// and label a probe target.
func GetChannelProbeTargets() ([]*Channel, error) {
	var channels []*Channel
	err := DB.Model(&Channel{}).Select("id", "name", "alias", "type", "status").Order("id desc").Find(&channels).Error
	return channels, err
}
