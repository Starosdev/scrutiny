package database

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
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
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "history.db")), productionGormConfig())
	require.NoError(t, err)
	repo := &scrutinyRepository{gormClient: db, logger: logrus.New()}
	t.Cleanup(func() { _ = repo.Close() })
	require.NoError(t, repo.migrateSQLite(context.Background(), false))
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

// sqliteSchemaDetail describes a SQLite schema at the level SQLite itself enforces: per table,
// every column's declared type, NOT NULL, default, and primary key position; every index's
// uniqueness, origin, partial flag, and ordered columns; every foreign key; whether the table DDL
// uses AUTOINCREMENT, CHECK, COLLATE, or WITHOUT ROWID; and every trigger and view.
//
// Column order is left out on purpose. ALTER TABLE ADD COLUMN appends while AutoMigrate follows
// struct order, so a new database and an upgraded one list columns differently, as do upgraded
// databases from different starting versions. No SQL here reads or writes columns by position,
// and migrations must keep naming columns explicitly (TESTING.md). Other DDL formatting differs
// between AutoMigrate and hand-written statements without changing behaviour.
func sqliteSchemaDetail(t *testing.T, db *gorm.DB) map[string][]string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> 'migrations' ORDER BY name`).Scan(&tables).Error)
	out := map[string][]string{}
	for _, table := range tables {
		var entries []string
		var columns []struct {
			Name      string
			Type      string
			NotNull   int     `gorm:"column:notnull"`
			DfltValue *string `gorm:"column:dflt_value"`
			Pk        int
		}
		require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA table_info(%q)", table)).Scan(&columns).Error)
		for _, c := range columns {
			dflt := "<none>"
			if c.DfltValue != nil {
				dflt = normalizeSQLiteDefault(*c.DfltValue)
			}
			entries = append(entries, fmt.Sprintf("column %s type=%s notnull=%d default=%s pk=%d", c.Name, strings.ToUpper(c.Type), c.NotNull, dflt, c.Pk))
		}
		var indexes []struct {
			Name    string
			Unique  int
			Origin  string
			Partial int
		}
		require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA index_list(%q)", table)).Scan(&indexes).Error)
		for _, idx := range indexes {
			var cols []string
			require.NoError(t, db.Raw(fmt.Sprintf("SELECT name FROM pragma_index_info(%q) ORDER BY seqno", idx.Name)).Scan(&cols).Error)
			name := idx.Name
			if strings.HasPrefix(name, "sqlite_autoindex_") {
				name = "<autoindex>"
			}
			entries = append(entries, fmt.Sprintf("index %s(%s) unique=%d origin=%s partial=%d", name, strings.Join(cols, ","), idx.Unique, idx.Origin, idx.Partial))
		}
		var fks []struct {
			Table string
			From  string
			To    *string
		}
		require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA foreign_key_list(%q)", table)).Scan(&fks).Error)
		for _, fk := range fks {
			to := ""
			if fk.To != nil {
				to = *fk.To
			}
			entries = append(entries, fmt.Sprintf("fk %s -> %s(%s)", fk.From, fk.Table, to))
		}
		var ddl string
		require.NoError(t, db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&ddl).Error)
		upper := strings.ToUpper(ddl)
		entries = append(entries,
			fmt.Sprintf("table autoincrement=%t check=%d collate=%d without_rowid=%t",
				strings.Contains(upper, "AUTOINCREMENT"), strings.Count(upper, "CHECK"),
				strings.Count(upper, "COLLATE"), strings.Contains(upper, "WITHOUT ROWID")))
		sort.Strings(entries)
		out[table] = entries
	}
	var objects []string
	require.NoError(t, db.Raw(`SELECT type || ' ' || name || ' on ' || tbl_name FROM sqlite_master WHERE type IN ('trigger', 'view') ORDER BY name`).Scan(&objects).Error)
	out["<triggers and views>"] = objects
	return out
}

// normalizeSQLiteDefault folds spellings SQLite stores identically: a default of ui, "ui", or 'ui'
// is the text ui, and TRUE and FALSE are the integers 1 and 0. ” stays distinct from no default.
func normalizeSQLiteDefault(v string) string {
	if len(v) >= 2 && (v[0] == '\'' || v[0] == '"') && v[len(v)-1] == v[0] {
		return "text:" + v[1:len(v)-1]
	}
	switch strings.ToUpper(v) {
	case "TRUE":
		return "1"
	case "FALSE":
		return "0"
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return v
	}
	return "text:" + v
}

// TestBaselineSchemaMatchesMigrationChainExactly is the safety net for fresh installs: a database
// built from the baseline must hold the same tables, column types, NOT NULL constraints,
// defaults, primary keys, indexes, and foreign keys as one built by running every migration.
func TestBaselineSchemaMatchesMigrationChainExactly(t *testing.T) {
	chain := sqliteSchemaDetail(t, migratedSQLite(t))

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "baseline.db")), productionGormConfig())
	require.NoError(t, err)
	require.NoError(t, initializeBaseline(context.Background(), db, logrus.New()))

	require.Empty(t, schemaDetailDiff(chain, sqliteSchemaDetail(t, db)), "chain (-) vs baseline (+)")
}

// schemaDetailDiff lists entries present in only one schema, "-" for want and "+" for got.
func schemaDetailDiff(want, got map[string][]string) []string {
	var diff []string
	tables := map[string]bool{}
	for table := range want {
		tables[table] = true
	}
	for table := range got {
		tables[table] = true
	}
	for table := range tables {
		inGot := map[string]bool{}
		for _, e := range got[table] {
			inGot[e] = true
		}
		inWant := map[string]bool{}
		for _, e := range want[table] {
			inWant[e] = true
			if !inGot[e] {
				diff = append(diff, "- "+table+": "+e)
			}
		}
		for _, e := range got[table] {
			if !inWant[e] {
				diff = append(diff, "+ "+table+": "+e)
			}
		}
	}
	sort.Strings(diff)
	return diff
}
