package database

import (
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260514000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260930000000"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// An existing btrfs_filesystems table keeps its rows and gains the statfs columns, zeroed.
func TestMigrateBtrfsStatfsColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "btrfs.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&m20260514000000.BtrfsFilesystem{}))
	require.NoError(t, db.Exec(`INSERT INTO btrfs_filesystems (uuid, used) VALUES ('fs-1', 42)`).Error)

	require.NoError(t, m20260930000000.Migrate(db))

	require.True(t, db.Migrator().HasColumn("btrfs_filesystems", "statfs_used"))
	require.True(t, db.Migrator().HasColumn("btrfs_filesystems", "statfs_available"))
	var row struct {
		Used            int64
		StatfsUsed      int64
		StatfsAvailable int64
	}
	require.NoError(t, db.Raw(`SELECT used, statfs_used, statfs_available FROM btrfs_filesystems WHERE uuid = 'fs-1'`).Scan(&row).Error)
	require.Equal(t, int64(42), row.Used)
	require.Equal(t, int64(0), row.StatfsUsed)
	require.Equal(t, int64(0), row.StatfsAvailable)
}
