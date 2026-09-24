package database

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// schemaSnapshot describes a database schema independent of dialect: for each table, its columns
// and its indexes as "name(columns) unique".
type schemaSnapshot map[string][]string

func snapshotSchema(t *testing.T, db *gorm.DB) schemaSnapshot {
	t.Helper()
	tables, err := db.Migrator().GetTables()
	require.NoError(t, err)
	snap := schemaSnapshot{}
	for _, table := range tables {
		if table == "migrations" || strings.HasPrefix(table, "sqlite_") {
			continue
		}
		var entries []string
		columns, err := db.Migrator().ColumnTypes(table)
		require.NoError(t, err)
		for _, c := range columns {
			entries = append(entries, "column "+c.Name())
		}
		indexes, err := db.Migrator().GetIndexes(table)
		require.NoError(t, err)
		for _, idx := range indexes {
			unique, _ := idx.Unique()
			primary, _ := idx.PrimaryKey()
			if primary {
				continue
			}
			entries = append(entries, fmt.Sprintf("index %s(%s) unique=%t", idx.Name(), strings.Join(idx.Columns(), ","), unique))
		}
		sort.Strings(entries)
		snap[table] = entries
	}
	return snap
}

func migratedSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "history.db")), &gorm.Config{})
	require.NoError(t, err)
	repo := &scrutinyRepository{gormClient: db, logger: logrus.New()}
	t.Cleanup(func() { _ = repo.Close() })
	require.NoError(t, repo.Migrate(context.Background()))
	return db
}

// TestSchemaModelsMatchMigrationHistory guards the PostgreSQL baseline: schemaModels plus
// baselineIndexes must build the same tables, columns, and indexes as the full SQLite history.
func TestSchemaModelsMatchMigrationHistory(t *testing.T) {
	history := snapshotSchema(t, migratedSQLite(t))

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "models.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, createBaselineSchema(db))
	fromModels := snapshotSchema(t, db)

	require.Equal(t, history, fromModels)
}

func TestInitializeBaselineSeedsSettingsFromHistory(t *testing.T) {
	var historySettings []models.SettingEntry
	require.NoError(t, migratedSQLite(t).Order("setting_key_name").Find(&historySettings).Error)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "baseline.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initializeBaseline(context.Background(), db, logrus.New()))

	var baselineSettings []models.SettingEntry
	require.NoError(t, db.Order("setting_key_name").Find(&baselineSettings).Error)
	require.NotEmpty(t, baselineSettings)
	require.Equal(t, settingValues(uniqueSettings(historySettings)), settingValues(baselineSettings))
	require.Equal(t, snapshotSchema(t, migratedSQLite(t)), snapshotSchema(t, db))
}

func settingValues(entries []models.SettingEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, fmt.Sprintf("%s|%s|%s|%d|%t", e.SettingKeyName, e.SettingDataType, e.SettingValueString, e.SettingValueNumeric, e.SettingValueBool))
	}
	return out
}
