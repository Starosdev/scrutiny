package models

import (
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
)

// AttributeOverride represents a user-configured override for SMART attribute evaluation
// stored in the database. This allows users to ignore attributes, force their status,
// or set custom warning/failure thresholds via the UI.
type AttributeOverride struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	WarnAbove   *int64    `json:"warn_above,omitempty"`
	FailAbove   *int64    `json:"fail_above,omitempty"`
	Protocol    string    `json:"protocol" gorm:"not null;uniqueIndex:idx_override_lookup"`
	AttributeId string    `json:"attribute_id" gorm:"not null;uniqueIndex:idx_override_lookup"`
	WWN         string    `json:"wwn,omitempty" gorm:"uniqueIndex:idx_override_lookup"`
	Action      string    `json:"action,omitempty"`
	Status      string    `json:"status,omitempty"`
	Source      string    `json:"source" gorm:"default:'ui'"`
	ID          uint      `json:"id" gorm:"primaryKey"`
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
