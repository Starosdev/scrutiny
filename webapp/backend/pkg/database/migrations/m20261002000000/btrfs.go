package m20261002000000

import "gorm.io/gorm"

// Migrate drops free_statfs from btrfs_filesystems. statfs_available holds the same figure,
// read from statfs(2) rather than parsed from btrfs-progs output (#929).
func Migrate(db *gorm.DB) error {
	if !db.Migrator().HasColumn("btrfs_filesystems", "free_statfs") {
		return nil
	}
	return db.Exec("ALTER TABLE btrfs_filesystems DROP COLUMN free_statfs").Error
}
