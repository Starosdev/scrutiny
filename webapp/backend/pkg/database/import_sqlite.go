package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260924000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormLogger "gorm.io/gorm/logger"
)

// ImportSummary reports how many rows ImportSQLite copied per table.
type ImportSummary map[string]int

// ImportSQLite copies every relational table from the SQLite database at sourcePath into the
// configured PostgreSQL database (#880). InfluxDB data is not touched: point the PostgreSQL
// install at the same InfluxDB.
//
// The source must be on the current schema: run this Scrutiny version on it once first. The
// target must hold no data. The target is migrated first, which creates the baseline on a new
// database, and the copy runs in one transaction, so a failed import leaves no partial data.
func ImportSQLite(ctx context.Context, appConfig config.Interface, sourcePath string, logger logrus.FieldLogger) (ImportSummary, error) {
	if appConfig.GetString(cfgDatabaseType) != dialectPostgres {
		return nil, fmt.Errorf("import-sqlite needs web.database.type set to postgres, got %q", appConfig.GetString(cfgDatabaseType))
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return nil, fmt.Errorf("cannot read SQLite database: %w", err)
	}

	// Read-only, so a mistyped path or a running server cannot be changed by the import.
	source, err := gorm.Open(sqlite.Open("file:"+sourcePath+"?mode=ro"), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	defer func() {
		if sqlDB, dbErr := source.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	}()
	if schemaErr := requireCurrentSchema(ctx, source); schemaErr != nil {
		return nil, explainSQLiteOpenError(schemaErr, sourcePath)
	}

	target, err := openGormDatabase(appConfig, logger)
	if err != nil {
		return nil, err
	}
	targetRepo := &scrutinyRepository{appConfig: appConfig, gormClient: target, logger: logger}
	defer func() { _ = targetRepo.Close() }()
	if err = targetRepo.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("prepare PostgreSQL database: %w", err)
	}
	if emptyErr := requireEmptyTarget(ctx, target); emptyErr != nil {
		return nil, emptyErr
	}

	var summary ImportSummary
	err = target.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var copyErr error
		summary, copyErr = copyAllTables(ctx, source, tx)
		return copyErr
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// copyAllTables replaces the target's default settings with the source's and copies every other
// table, inside the caller's transaction.
func copyAllTables(ctx context.Context, source *gorm.DB, tx *gorm.DB) (ImportSummary, error) {
	summary := ImportSummary{}
	// The baseline seeded default settings; the source's settings replace them.
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.SettingEntry{}).Error; err != nil {
		return nil, err
	}
	var settings []models.SettingEntry
	if err := source.WithContext(ctx).Order("id").Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	settings = uniqueSettings(settings)
	if err := tx.CreateInBatches(settings, 200).Error; err != nil {
		return nil, fmt.Errorf("write settings: %w", err)
	}
	summary["settings"] = len(settings)

	for _, model := range importedModels() {
		table, count, err := copyTable(ctx, source, tx, model)
		if err != nil {
			return nil, fmt.Errorf("copy %s: %w", table, err)
		}
		summary[table] = count
		if err := resetIDSequence(tx, model); err != nil {
			return nil, fmt.Errorf("reset %s id sequence: %w", table, err)
		}
	}
	return summary, nil
}

// importedModels lists the tables copied row by row. Settings are copied separately, and the
// leader lease and the notification outbox are left out: they are runtime state of the source
// install, and a copied lease would name a holder that no longer runs.
func importedModels() []interface{} {
	var out []interface{}
	for _, model := range schemaModels() {
		switch model.(type) {
		case *models.SettingEntry, *models.NotificationOutbox, *m20260924000000.SchedulerLease:
			continue
		}
		out = append(out, model)
	}
	return out
}

// explainSQLiteOpenError adds the likely fix to SQLite's bare open errors. A database in WAL
// mode, which is Scrutiny's default, needs its -shm file even when opened read-only, and SQLite
// cannot create it in a read-only directory such as a :ro volume.
func explainSQLiteOpenError(err error, sourcePath string) error {
	msg := err.Error()
	if !strings.Contains(msg, "unable to open database file") && !strings.Contains(msg, "readonly database") {
		return err
	}
	return fmt.Errorf("%w\n\nThe directory must be writable: SQLite needs to create %s-shm next to the database, "+
		"even though the import only reads it. Mount the directory read-write, or copy %s and any "+
		"%s-wal file to a writable directory and import from there", err, filepath.Base(sourcePath), filepath.Base(sourcePath), filepath.Base(sourcePath))
}

// requireCurrentSchema refuses a source that has not run every migration this version knows,
// since its tables would not match the target's.
func requireCurrentSchema(ctx context.Context, source *gorm.DB) error {
	var applied []string
	if err := source.WithContext(ctx).Table("migrations").Pluck("id", &applied).Error; err != nil {
		return fmt.Errorf("read SQLite migrations table: %w", err)
	}
	have := make(map[string]bool, len(applied))
	for _, id := range applied {
		have[id] = true
	}
	for _, m := range (&scrutinyRepository{}).schemaMigrations(ctx) {
		if !have[m.ID] {
			return fmt.Errorf("the SQLite database is missing migration %s: start this version of Scrutiny on it once, stop it, then import", m.ID)
		}
	}
	return nil
}

// requireEmptyTarget refuses a target that already holds data other than the default settings,
// so an import never merges into or overwrites a live database.
func requireEmptyTarget(ctx context.Context, target *gorm.DB) error {
	for _, model := range importedModels() {
		var count int64
		if err := target.WithContext(ctx).Unscoped().Model(model).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			stmt := &gorm.Statement{DB: target}
			_ = stmt.Parse(model)
			return fmt.Errorf("the PostgreSQL database already has data in %s; import only into a new database", stmt.Schema.Table)
		}
	}
	return nil
}

// copyTable copies every row of model's table, including soft-deleted rows.
// ponytail: loads a whole table into memory; the relational tables hold device and setting
// metadata (time series live in InfluxDB), so they stay small. Batch the read if that changes.
func copyTable(ctx context.Context, source *gorm.DB, target *gorm.DB, model interface{}) (string, int, error) {
	stmt := &gorm.Statement{DB: target}
	if err := stmt.Parse(model); err != nil {
		return "", 0, err
	}
	table := stmt.Schema.Table

	rows := reflect.New(reflect.SliceOf(reflect.TypeOf(model).Elem()))
	if err := source.WithContext(ctx).Unscoped().Find(rows.Interface()).Error; err != nil {
		return table, 0, err
	}
	count := rows.Elem().Len()
	if count == 0 {
		return table, 0, nil
	}
	if err := target.Omit(clause.Associations).CreateInBatches(rows.Interface(), 200).Error; err != nil {
		return table, count, err
	}

	// On create, GORM replaces a zero value with the field's default tag (for example a copied
	// heartbeat_enabled=false becomes true), even with Select("*"), and writes the default into
	// the struct too. Updates never apply defaults, so write those fields back from a fresh read
	// of the source.
	var defaulted []string
	for _, field := range stmt.Schema.Fields {
		if field.DBName != "" && !field.PrimaryKey && field.DefaultValueInterface != nil {
			defaulted = append(defaulted, field.DBName)
		}
	}
	if len(defaulted) == 0 {
		return table, count, nil
	}
	original := reflect.New(reflect.SliceOf(reflect.TypeOf(model).Elem()))
	if err := source.WithContext(ctx).Unscoped().Find(original.Interface()).Error; err != nil {
		return table, count, err
	}
	for i := 0; i < original.Elem().Len(); i++ {
		row := original.Elem().Index(i).Addr().Interface()
		if err := target.Unscoped().Model(row).Select(defaulted).Updates(row).Error; err != nil {
			return table, count, err
		}
	}
	return table, count, nil
}

// resetIDSequence moves a serial id sequence past the copied rows, so the next insert does not
// collide with an imported id. Tables without a serial id column are left alone.
func resetIDSequence(tx *gorm.DB, model interface{}) error {
	stmt := &gorm.Statement{DB: tx}
	if err := stmt.Parse(model); err != nil {
		return err
	}
	if stmt.Schema.LookUpField("id") == nil {
		return nil
	}
	table := stmt.Schema.Table
	return tx.Exec(fmt.Sprintf(
		`SELECT setval(pg_get_serial_sequence('%[1]s', 'id'), COALESCE((SELECT MAX(id) FROM %[1]s), 1), (SELECT MAX(id) FROM %[1]s) IS NOT NULL)`,
		table,
	)).Error
}
