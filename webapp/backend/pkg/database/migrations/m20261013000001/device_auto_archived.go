package m20261013000001

import "gorm.io/gorm"

// Device holds only the column this migration adds.
type Device struct {
	AutoArchived bool `gorm:"default:false"`
}

func (Device) TableName() string {
	return "devices"
}

// Migrate adds devices.auto_archived, which records that the missed-ping monitor archived a
// device, as opposed to a user.
func Migrate(db *gorm.DB) error {
	if db.Migrator().HasColumn(&Device{}, "AutoArchived") {
		return nil
	}
	return db.Migrator().AddColumn(&Device{}, "AutoArchived")
}
