package m20260906000000

import "time"

// AttributeOverride is the migration-scoped struct after adding pinned_value,
// which stores the value an "acknowledge" override is pinned to (#775). The
// column is nullable: every pre-existing override predates the action and must
// stay unpinned. AutoMigrate adds only the missing column, so re-running the
// migration on a database that already has it is a no-op.
type AttributeOverride struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	WarnAbove   *int64    `json:"warn_above,omitempty"`
	FailAbove   *int64    `json:"fail_above,omitempty"`
	PinnedValue *int64    `json:"pinned_value,omitempty"`
	Protocol    string    `json:"protocol" gorm:"not null;uniqueIndex:idx_override_lookup"`
	AttributeId string    `json:"attribute_id" gorm:"not null;uniqueIndex:idx_override_lookup"`
	DeviceID    string    `json:"device_id,omitempty" gorm:"uniqueIndex:idx_override_lookup"`
	WWN         string    `json:"wwn,omitempty" gorm:"uniqueIndex:idx_override_lookup"`
	Action      string    `json:"action,omitempty"`
	Status      string    `json:"status,omitempty"`
	Source      string    `json:"source" gorm:"default:'ui'"`
	ID          uint      `json:"id" gorm:"primaryKey"`
}

func (AttributeOverride) TableName() string {
	return "attribute_overrides"
}
