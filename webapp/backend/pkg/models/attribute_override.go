package models

import (
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
	"gorm.io/gorm"
)

// AttributeOverride represents a user-configured override for SMART attribute evaluation
// stored in the database. This allows users to ignore attributes, force their status,
// or set custom warning/failure thresholds via the UI.
type AttributeOverride struct {
	// Explicit ID field with lowercase JSON tag for frontend compatibility
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// Required: Protocol type (ATA, NVMe, SCSI)
	Protocol string `json:"protocol" gorm:"not null;index:idx_override_lookup"`

	// Required: Attribute ID (string for all protocols)
	// ATA: "5", "187", etc.
	// ATA DevStats: "devstat_7_8"
	// NVMe: "media_errors", "percentage_used"
	// SCSI: "scsi_grown_defect_list"
	AttributeId string `json:"attribute_id" gorm:"not null;index:idx_override_lookup"`

	// Optional: Limit override to specific device by WWN
	// If empty, override applies to all devices
	WWN string `json:"wwn,omitempty" gorm:"index:idx_override_lookup"`

	// Optional: Action to take (ignore or force_status)
	// If not set, custom thresholds are applied
	Action string `json:"action,omitempty"`

	// For force_status action: the status to set
	// Values: "passed", "warn", "failed"
	Status string `json:"status,omitempty"`

	// Custom threshold: warn when value exceeds this
	WarnAbove *int64 `json:"warn_above,omitempty"`

	// Custom threshold: fail when value exceeds this (takes precedence over warn)
	FailAbove *int64 `json:"fail_above,omitempty"`

	// Source indicates where this override came from: "ui" or "config"
	// UI overrides can be deleted from the interface; config overrides cannot
	Source string `json:"source" gorm:"default:'ui'"`
}

// TableName specifies the table name for GORM
func (AttributeOverride) TableName() string {
	return "attribute_overrides"
}

// ToOverride converts the database model to the overrides package type
func (ao *AttributeOverride) ToOverride() overrides.AttributeOverride {
	return overrides.AttributeOverride{
		Protocol:    ao.Protocol,
		AttributeId: ao.AttributeId,
		WWN:         ao.WWN,
		Action:      overrides.AttributeOverrideAction(ao.Action),
		Status:      ao.Status,
		WarnAbove:   ao.WarnAbove,
		FailAbove:   ao.FailAbove,
	}
}

// ConvertToOverrides converts a slice of database models to overrides package types
func ConvertToOverrides(dbOverrides []AttributeOverride) []overrides.AttributeOverride {
	result := make([]overrides.AttributeOverride, len(dbOverrides))
	for i := range dbOverrides {
		result[i] = dbOverrides[i].ToOverride()
	}
	return result
}
