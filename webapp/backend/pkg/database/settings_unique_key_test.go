package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20220716214900"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The migration history seeds line_stroke twice; after migrating there must be one row per key,
// and the database must refuse a second row for a key (#889).
func TestMigratedSettingsHaveUniqueKeys(t *testing.T) {
	db := migratedSQLite(t)

	var count int64
	require.NoError(t, db.Unscoped().Model(&models.SettingEntry{}).Where("setting_key_name = ?", "line_stroke").Count(&count).Error)
	require.Equal(t, int64(1), count)

	err := db.Exec(`INSERT INTO settings (setting_key_name, setting_data_type, setting_value_string) VALUES ('line_stroke', 'string', 'smooth')`).Error
	require.Error(t, err, "a duplicate setting key is rejected")
}

func TestMigrateSettingsUniqueKeyKeepsTheRowLoadSettingsUses(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&m20220716214900.Setting{}))

	deleted := time.Now()
	rows := []struct {
		id      uint
		key     string
		value   string
		deleted bool
	}{
		{1, "a", "old", false},
		{2, "a", "new", false},    // live duplicates: the highest id wins
		{3, "b", "deleted", true}, // a deleted row never beats a live one
		{4, "b", "live", false},
		{5, "b", "deleted2", true},
		{6, "c", "first", true}, // only deleted rows: the highest id stays
		{7, "c", "second", true},
		{8, "d", "only", false},
	}
	for _, r := range rows {
		var deletedAt interface{}
		if r.deleted {
			deletedAt = deleted
		}
		require.NoError(t, db.Exec(`INSERT INTO settings (id, setting_key_name, setting_data_type, setting_value_string, deleted_at) VALUES (?, ?, 'string', ?, ?)`,
			r.id, r.key, r.value, deletedAt).Error)
	}

	require.NoError(t, migrateSettingsUniqueKey(db))

	var kept []m20220716214900.Setting
	require.NoError(t, db.Unscoped().Order("id").Find(&kept).Error)
	var ids []uint
	for _, s := range kept {
		ids = append(ids, s.ID)
	}
	require.Equal(t, []uint{2, 4, 7, 8}, ids)
}

// A PostgreSQL database created before this migration has it still to run; it must succeed
// there too.
func TestPostgres_SettingsUniqueKeyMigrationRuns(t *testing.T) {
	ctx := context.Background()
	repo := newPostgresTestRepository(t, newPostgresTestConfig(t))
	require.NoError(t, repo.Migrate(ctx))
	require.NoError(t, repo.gormClient.Exec(`DELETE FROM migrations WHERE id = 'm20260926000000'`).Error)
	require.NoError(t, repo.Migrate(ctx))

	var applied int64
	require.NoError(t, repo.gormClient.Table("migrations").Where("id = ?", "m20260926000000").Count(&applied).Error)
	require.Equal(t, int64(1), applied)
}
