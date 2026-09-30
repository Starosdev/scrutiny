package m20260930000000

import "gorm.io/gorm"

// BtrfsFilesystem holds only the columns this migration adds; AutoMigrate leaves the rest alone.
type BtrfsFilesystem struct {
	StatfsUsed      int64
	StatfsAvailable int64
}

func (BtrfsFilesystem) TableName() string {
	return "btrfs_filesystems"
}

// Migrate adds statfs used/available bytes to btrfs_filesystems, so usage can be computed the way
// df does on every btrfs profile.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&BtrfsFilesystem{})
}
