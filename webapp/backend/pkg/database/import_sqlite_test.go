package database

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newImportSource creates a migrated SQLite database holding the rows most likely to be
// damaged by a copy: false values under a default:true tag, soft-deleted rows, associated
// rows, changed settings, and rows with serial ids.
func newImportSource(t *testing.T) (string, *gorm.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scrutiny.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, (&scrutinyRepository{gormClient: db, logger: logrus.New()}).Migrate(context.Background()))

	require.NoError(t, db.Exec(`INSERT INTO devices (device_id, wwn, device_name, serial_number, model_name, smart_display_mode, has_forced_failure, muted, archived)
		VALUES ('live', '0x5000', 'sda', 'S1', 'M', 'raw', false, true, false)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO devices (device_id, wwn, device_name, serial_number, model_name, deleted_at)
		VALUES ('gone', '0x5001', 'sdb', 'S2', 'M', ?)`, time.Now()).Error)
	require.NoError(t, db.Exec(`INSERT INTO notify_urls (id, url, label, source, heartbeat_enabled, created_at, updated_at)
		VALUES (7, 'discord://token@channel', 'alerts', 'ui', false, ?, ?)`, time.Now(), time.Now()).Error)
	require.NoError(t, db.Exec(`INSERT INTO zfs_pools (guid, name, host_id) VALUES ('pool-1', 'tank', 'nas')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO zfs_vdevs (pool_guid, name, guid) VALUES ('pool-1', 'sda', 'vdev-1'), ('pool-1', 'sdb', 'vdev-2')`).Error)
	require.NoError(t, db.Exec(`UPDATE settings SET setting_value_string = 'straight' WHERE setting_key_name = 'line_stroke'`).Error)
	// Runtime state of the running install, which must not be imported.
	require.NoError(t, db.Exec(`INSERT INTO scheduler_leases (name, holder, expires_at_unix_ms) VALUES ('scheduler', 'old-host', ?)`, time.Now().Add(time.Hour).UnixMilli()).Error)
	require.NoError(t, db.Exec(`INSERT INTO notification_outbox (created_at_unix_ms, state, body) VALUES (?, 'queued', '{}')`, time.Now().UnixMilli()).Error)
	return path, db
}

func TestImportSQLite(t *testing.T) {
	ctx := context.Background()
	cfg := newPostgresTestConfig(t)
	path, _ := newImportSource(t)

	summary, err := ImportSQLite(ctx, cfg, path, logrus.New())
	require.NoError(t, err)
	require.Equal(t, 2, summary["devices"], "soft-deleted rows are copied too")
	require.Equal(t, 1, summary["notify_urls"])
	require.Equal(t, 2, summary["zfs_vdevs"])

	target := newPostgresTestRepository(t, cfg).gormClient

	var url models.NotifyUrl
	require.NoError(t, target.First(&url, 7).Error)
	require.False(t, url.HeartbeatEnabled, "a false value under a default:true tag survives the copy")

	var live models.Device
	require.NoError(t, target.First(&live, "device_id = ?", "live").Error)
	require.True(t, live.Muted)
	require.Equal(t, "raw", live.SmartDisplayMode)
	var gone models.Device
	require.NoError(t, target.Unscoped().First(&gone, "device_id = ?", "gone").Error)
	require.NotNil(t, gone.DeletedAt)

	var lineStroke []models.SettingEntry
	require.NoError(t, target.Where("setting_key_name = ?", "line_stroke").Find(&lineStroke).Error)
	require.Len(t, lineStroke, 1)
	require.Equal(t, "straight", lineStroke[0].SettingValueString)

	next := models.NotifyUrl{URL: "ntfy://topic"}
	require.NoError(t, target.Create(&next).Error, "the id sequence continues after the imported rows")
	require.Greater(t, next.ID, uint(7))

	var leases, outbox int64
	require.NoError(t, target.Table("scheduler_leases").Count(&leases).Error)
	require.NoError(t, target.Table("notification_outbox").Count(&outbox).Error)
	require.Zero(t, leases, "the source install's leader lease is runtime state and is not imported")
	require.Zero(t, outbox, "undelivered notifications of the source install are not imported")
	require.NotContains(t, summary, "scheduler_leases")
	require.NotContains(t, summary, "notification_outbox")

	_, err = ImportSQLite(ctx, cfg, path, logrus.New())
	require.ErrorContains(t, err, "already has data")
}

// A WAL-mode database opened read-only needs to create its -shm file next to it. In a
// read-only directory that fails with SQLite's bare "unable to open database file"; the import
// must say what to do instead.
func TestImportSQLite_ExplainsReadOnlyWALDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced for this user")
	}
	path, source := newImportSource(t)
	require.NoError(t, source.Exec("PRAGMA journal_mode=WAL").Error)
	sqlDB, err := source.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	dir := filepath.Dir(path)
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	cfg, err := config.Create()
	require.NoError(t, err)
	cfg.Set("web.database.type", "postgres")
	cfg.Set("web.database.dsn", "postgres://unused@127.0.0.1:1/unused")

	_, err = ImportSQLite(context.Background(), cfg, path, logrus.New())
	require.ErrorContains(t, err, "directory must be writable")
}

func TestImportSQLite_RefusesOutdatedSource(t *testing.T) {
	cfg := newPostgresTestConfig(t)
	path, source := newImportSource(t)
	require.NoError(t, source.Exec(`DELETE FROM migrations WHERE id = 'm20260925000000'`).Error)

	_, err := ImportSQLite(context.Background(), cfg, path, logrus.New())
	require.ErrorContains(t, err, "missing migration m20260925000000")
}

func TestImportSQLite_RequiresPostgresTarget(t *testing.T) {
	path, _ := newImportSource(t)
	cfg, err := config.Create()
	require.NoError(t, err)

	_, err = ImportSQLite(context.Background(), cfg, path, logrus.New())
	require.ErrorContains(t, err, "web.database.type set to postgres")
}
