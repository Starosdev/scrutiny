package overrides

import (
	"fmt"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/mitchellh/mapstructure"
)

// AttributeOverrideAction defines what action to take for an override
type AttributeOverrideAction string

const (
	AttributeOverrideActionIgnore      AttributeOverrideAction = "ignore"
	AttributeOverrideActionForceStatus AttributeOverrideAction = "force_status"
)

// AttributeOverride defines a user-configured override for SMART attribute evaluation
type AttributeOverride struct {
	// Required: Protocol type (ATA, NVMe, SCSI)
	Protocol string `json:"protocol" mapstructure:"protocol"`

	// Required: Attribute ID (string for all protocols)
	// ATA: "5", "187", etc.
	// ATA DevStats: "devstat_7_8"
	// NVMe: "media_errors", "percentage_used"
	// SCSI: "scsi_grown_defect_list"
	AttributeId string `json:"attribute_id" mapstructure:"attribute_id"`

	// Optional: Limit override to specific device by WWN
	WWN string `json:"wwn,omitempty" mapstructure:"wwn"`

	// Optional: Action to take (ignore or force_status)
	// If not set, custom thresholds are applied
	Action AttributeOverrideAction `json:"action,omitempty" mapstructure:"action"`

	// For force_status action: the status to set
	// Values: "passed", "warn", "failed"
	Status string `json:"status,omitempty" mapstructure:"status"`

	// Custom threshold: warn when value exceeds this
	WarnAbove *int64 `json:"warn_above,omitempty" mapstructure:"warn_above"`

	// Custom threshold: fail when value exceeds this (takes precedence over warn)
	FailAbove *int64 `json:"fail_above,omitempty" mapstructure:"fail_above"`
}

// Matches checks if this override applies to the given attribute
func (ao *AttributeOverride) Matches(protocol, attributeId, wwn string) bool {
	if ao.Protocol != protocol {
		return false
	}
	if ao.AttributeId != attributeId {
		return false
	}
	// WWN is optional - if not set, matches all devices
	if ao.WWN != "" && ao.WWN != wwn {
		return false
	}
	return true
}

// GetForcedStatus converts the status string to AttributeStatus
func (ao *AttributeOverride) GetForcedStatus() pkg.AttributeStatus {
	switch ao.Status {
	case "passed":
		return pkg.AttributeStatusPassed
	case "warn":
		return pkg.AttributeStatusWarningScrutiny
	case "failed":
		return pkg.AttributeStatusFailedScrutiny
	default:
		return pkg.AttributeStatusPassed
	}
}

// FindOverride searches the override list for a matching override
func FindOverride(overrides []AttributeOverride, protocol, attributeId, wwn string) *AttributeOverride {
	for i := range overrides {
		if overrides[i].Matches(protocol, attributeId, wwn) {
			return &overrides[i]
		}
	}
	return nil
}

// Result contains the outcome of applying an override
type Result struct {
	// ShouldIgnore indicates the attribute should be completely ignored
	ShouldIgnore bool
	// Status is the forced status (if set)
	Status *pkg.AttributeStatus
	// StatusReason explains why the status was set
	StatusReason string
	// WarnAbove is the custom warning threshold
	WarnAbove *int64
	// FailAbove is the custom failure threshold
	FailAbove *int64
}

// ParseOverrides converts raw config data to typed AttributeOverride slice
func ParseOverrides(cfg config.Interface) []AttributeOverride {
	var result []AttributeOverride
	if cfg == nil {
		return result
	}

	raw := cfg.Get("smart.attribute_overrides")
	if raw == nil {
		return result
	}

	// Use mapstructure to decode the raw config into typed structs
	if err := mapstructure.Decode(raw, &result); err != nil {
		return result
	}
	return result
}

// Apply checks if an override exists for the given attribute and returns the result.
// Returns nil if no override matches.
func Apply(cfg config.Interface, protocol, attributeId, wwn string) *Result {
	overrideList := ParseOverrides(cfg)
	override := FindOverride(overrideList, protocol, attributeId, wwn)

	if override == nil {
		return nil
	}

	result := &Result{}

	switch override.Action {
	case AttributeOverrideActionIgnore:
		result.ShouldIgnore = true
		result.StatusReason = "Attribute ignored by user configuration"

	case AttributeOverrideActionForceStatus:
		status := override.GetForcedStatus()
		result.Status = &status
		result.StatusReason = "Status forced by user configuration"
	}

	// Custom thresholds (can be combined with force_status or standalone)
	result.WarnAbove = override.WarnAbove
	result.FailAbove = override.FailAbove

	return result
}

// ApplyThresholds evaluates custom thresholds against a value.
// Returns the status to set based on thresholds, or nil if no threshold exceeded.
// FailAbove takes precedence over WarnAbove.
func ApplyThresholds(result *Result, value int64) *pkg.AttributeStatus {
	if result == nil {
		return nil
	}

	// FailAbove takes precedence
	if result.FailAbove != nil && value > *result.FailAbove {
		status := pkg.AttributeStatusFailedScrutiny
		return &status
	}

	if result.WarnAbove != nil && value > *result.WarnAbove {
		status := pkg.AttributeStatusWarningScrutiny
		return &status
	}

	return nil
}

// MergeOverrides combines config file overrides with database overrides.
// Database overrides take precedence over config file overrides when they
// match the same protocol+attributeId+wwn combination.
func MergeOverrides(configOverrides, dbOverrides []AttributeOverride) []AttributeOverride {
	// Create map keyed by protocol+attributeId+wwn for deduplication
	merged := make(map[string]AttributeOverride)

	// Add config overrides first (lower priority)
	for _, o := range configOverrides {
		key := fmt.Sprintf("%s|%s|%s", o.Protocol, o.AttributeId, o.WWN)
		merged[key] = o
	}

	// Add/override with database overrides (higher priority)
	for _, o := range dbOverrides {
		key := fmt.Sprintf("%s|%s|%s", o.Protocol, o.AttributeId, o.WWN)
		merged[key] = o
	}

	result := make([]AttributeOverride, 0, len(merged))
	for _, o := range merged {
		result = append(result, o)
	}
	return result
}

// ApplyWithOverrides checks if an override exists in the provided list and returns the result.
// This is used when the caller has already merged config and database overrides.
// Returns nil if no override matches.
func ApplyWithOverrides(overrideList []AttributeOverride, protocol, attributeId, wwn string) *Result {
	override := FindOverride(overrideList, protocol, attributeId, wwn)

	if override == nil {
		return nil
	}

	result := &Result{}

	switch override.Action {
	case AttributeOverrideActionIgnore:
		result.ShouldIgnore = true
		result.StatusReason = "Attribute ignored by user configuration"

	case AttributeOverrideActionForceStatus:
		status := override.GetForcedStatus()
		result.Status = &status
		result.StatusReason = "Status forced by user configuration"
	}

	// Custom thresholds (can be combined with force_status or standalone)
	result.WarnAbove = override.WarnAbove
	result.FailAbove = override.FailAbove

	return result
}
