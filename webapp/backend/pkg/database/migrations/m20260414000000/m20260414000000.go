package m20260414000000

import (
	"time"
)

// AttributeOverride removes soft-deletes. The UNIQUE constraint/index on
// Protocol/AttributeID/WWN is applied after table recreation in the migration
// to avoid running into errors where idx_override_lookup already exists.
type AttributeOverride struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	WarnAbove   *int64    `json:"warn_above,omitempty"`
	FailAbove   *int64    `json:"fail_above,omitempty"`
	Protocol    string    `json:"protocol" gorm:"not null"`
	AttributeId string    `json:"attribute_id" gorm:"not null"`
	WWN         string    `json:"wwn,omitempty"`
	Action      string    `json:"action,omitempty"`
	Status      string    `json:"status,omitempty"`
	Source      string    `json:"source" gorm:"default:'ui'"`
}
