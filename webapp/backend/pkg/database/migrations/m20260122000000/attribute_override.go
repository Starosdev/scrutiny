package m20260122000000

import (
	"gorm.io/gorm"
)

// AttributeOverride represents a user-configured override for SMART attribute evaluation
// This is the migration version of the model
type AttributeOverride struct {
	gorm.Model

	Protocol    string `gorm:"not null;index:idx_override_lookup"`
	AttributeId string `gorm:"not null;index:idx_override_lookup"`
	WWN         string `gorm:"index:idx_override_lookup"`
	Action      string
	Status      string
	WarnAbove   *int64
	FailAbove   *int64
	Source      string `gorm:"default:'ui'"`
}

// TableName specifies the table name for GORM
func (AttributeOverride) TableName() string {
	return "attribute_overrides"
}
