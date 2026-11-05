package m20261027000000

import "gorm.io/gorm"

// FilesystemDevice is frozen at the schema this migration creates.
type FilesystemDevice struct {
	HostID     string `gorm:"primaryKey"`
	MountPoint string `gorm:"primaryKey"`
	DeviceName string `gorm:"primaryKey;index"`
}

func (FilesystemDevice) TableName() string {
	return "filesystem_devices"
}

// Migrate adds the filesystem_devices table, which links a filesystem to the
// whole disks backing it so per-drive free space can be reported (#470).
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&FilesystemDevice{})
}
