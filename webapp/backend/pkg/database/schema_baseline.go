package database

import (
	"context"
	"fmt"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// baselineIndexes are indexes the SQLite migration history creates with raw SQL rather than
// through model tags, so AutoMigrate alone does not build them.
var baselineIndexes = []string{
	// Plain, not unique: drives can share a WWN (#851). See m20260914000000.
	"CREATE INDEX IF NOT EXISTS idx_devices_wwn ON devices (wwn)",
	"CREATE INDEX IF NOT EXISTS idx_devices_deleted_at ON devices (deleted_at)",
}

// createBaselineSchema builds the current schema directly from schemaModels, for a database that
// starts empty and never ran the SQLite migration history.
func createBaselineSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(schemaModels()...); err != nil {
		return err
	}
	for _, stmt := range baselineIndexes {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// initializeBaseline creates the current schema and default settings in an empty database. The
// default settings come from running the real migration history on a throwaway in-memory SQLite
// database, so they cannot drift from what an upgraded SQLite install holds.
func initializeBaseline(ctx context.Context, db *gorm.DB, logger logrus.FieldLogger) error {
	if err := createBaselineSchema(db); err != nil {
		return fmt.Errorf("create baseline schema: %w", err)
	}

	reference, err := migratedReferenceDatabase(ctx, logger)
	if err != nil {
		return fmt.Errorf("build reference settings: %w", err)
	}
	defer func() {
		if sqlDB, err := reference.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	var settings []models.SettingEntry
	if err := reference.Order("id").Find(&settings).Error; err != nil {
		return fmt.Errorf("read reference settings: %w", err)
	}
	if err := db.WithContext(ctx).CreateInBatches(uniqueSettings(settings), 100).Error; err != nil {
		return fmt.Errorf("seed settings: %w", err)
	}
	return nil
}

// migratedReferenceDatabase returns an in-memory SQLite database with the full migration history
// applied and no user data.
func migratedReferenceDatabase(ctx context.Context, logger logrus.FieldLogger) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger:                                   gormLogger.Default.LogMode(gormLogger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, err
	}
	// Every connection to "file::memory:" is a separate database, so keep exactly one.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	reference := &scrutinyRepository{gormClient: db, logger: logger}
	if err := reference.Migrate(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// uniqueSettings keeps one row per setting key. The SQLite history seeds line_stroke twice (in
// m20220716214900 and m20221115214900) and its settings table has no unique key constraint, while
// a baseline database enforces one. The last row by ID wins, as it does when LoadSettings reads
// SQLite rows in their default order and each later row overrides the earlier one.
func uniqueSettings(entries []models.SettingEntry) []models.SettingEntry {
	last := make(map[string]int, len(entries))
	for i, e := range entries {
		last[e.SettingKeyName] = i
	}
	out := make([]models.SettingEntry, 0, len(last))
	for i, e := range entries {
		if last[e.SettingKeyName] == i {
			e.ID = 0
			out = append(out, e)
		}
	}
	return out
}
