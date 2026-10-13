package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

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

// The structs below create three tables whose migration history left column defaults or NULL
// constraints that the runtime models must not carry. They are used only to create those tables
// and are never written through, so their tags cannot change how GORM inserts rows. The schema
// equivalence test fails if any of them drifts from the migration history.

// baselineNotifyUrl gives heartbeat_enabled the database default of true that m20260608000000
// added. models.NotifyUrl has no default tag, since GORM would turn a false value into true on
// create (#888).
type baselineNotifyUrl struct {
	CreatedAt        time.Time
	UpdatedAt        time.Time
	URL              string `gorm:"not null"`
	Label            string
	Source           string `gorm:"default:'ui'"`
	HeartbeatEnabled bool   `gorm:"default:true"`
	ID               uint   `gorm:"primaryKey"`
}

func (baselineNotifyUrl) TableName() string { return "notify_urls" }

// baselineAttributeOverride carries the empty-string defaults and the NOT NULL device_id that
// m20260122000000 and m20260823000000 created.
type baselineAttributeOverride struct {
	CreatedAt   time.Time
	UpdatedAt   time.Time
	WarnAbove   *int64
	FailAbove   *int64
	PinnedValue *int64
	Protocol    string `gorm:"not null;uniqueIndex:idx_override_lookup"`
	AttributeId string `gorm:"not null;uniqueIndex:idx_override_lookup"`
	DeviceID    string `gorm:"not null;default:'';uniqueIndex:idx_override_lookup"`
	WWN         string `gorm:"default:'';uniqueIndex:idx_override_lookup"`
	Action      string `gorm:"default:''"`
	Status      string `gorm:"default:''"`
	Source      string `gorm:"default:'ui'"`
	ID          uint   `gorm:"primaryKey"`
}

func (baselineAttributeOverride) TableName() string { return "attribute_overrides" }

// baselineSettingEntry leaves setting_key_name nullable, as m20220716214900 created it. The
// unique index from m20260926000000 still rejects duplicate keys.
type baselineSettingEntry struct {
	gorm.Model
	SettingKeyName        string `gorm:"uniqueIndex:idx_settings_setting_key_name"`
	SettingKeyDescription string
	SettingDataType       string
	SettingValueString    string
	SettingValueNumeric   int
	SettingValueBool      bool
}

func (baselineSettingEntry) TableName() string { return "settings" }

// baselineModels is schemaModels with the three tables above swapped for their baseline structs.
// Only SQLite uses it; see createBaselineSchema.
func baselineModels() []interface{} {
	out := schemaModels()
	for i, model := range out {
		switch model.(type) {
		case *models.NotifyUrl:
			out[i] = &baselineNotifyUrl{}
		case *models.AttributeOverride:
			out[i] = &baselineAttributeOverride{}
		case *models.SettingEntry:
			out[i] = &baselineSettingEntry{}
		}
	}
	return out
}

// createBaselineSchema builds the current schema directly from the models, for a database that
// starts empty and never ran the migration history. SQLite uses baselineModels, so a new database
// matches one that ran the SQLite history. PostgreSQL keeps schemaModels, the shape every
// PostgreSQL database has had since 1.76.0, so databases created before and after 2.0 agree.
func createBaselineSchema(db *gorm.DB) error {
	all := baselineModels()
	if db.Name() == dialectPostgres {
		all = schemaModels()
	}
	if err := db.AutoMigrate(all...); err != nil {
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
	if err := reference.migrateSQLite(ctx, false); err != nil {
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
	for i := range entries {
		last[entries[i].SettingKeyName] = i
	}
	out := make([]models.SettingEntry, 0, len(last))
	for i := range entries {
		if last[entries[i].SettingKeyName] == i {
			e := entries[i]
			e.ID = 0
			out = append(out, e)
		}
	}
	return out
}

// minimumUpgradeMigrationID is the last schema migration in Scrutiny 1.70.0 (and 1.69.2, which
// carries the same migrations). A Scrutiny database without it is older than the minimum
// upgrade-from version and is refused (docs/UPGRADE_2.0.md).
const minimumUpgradeMigrationID = "m20260823000000"

// upstreamMigrationIDs are the migrations AnalogJ/scrutiny (through 0.9.5) records. A database
// whose history holds only these comes from AnalogJ/scrutiny, or from a Staros build that had not
// yet diverged from it, and still runs the full history, which the AnalogJ import path needs
// (migrateM20260803000000). m20260105083200 and m20260216155600 exist only upstream.
var upstreamMigrationIDs = map[string]bool{
	"20201107210306":  true,
	"20220503113100":  true,
	"20220503120000":  true,
	"m20220509170100": true,
	"m20220709181300": true,
	"m20220716214900": true,
	"m20221115214900": true,
	"m20231123123300": true,
	"m20240722082740": true,
	"m20250221084400": true,
	"m20260105083200": true,
	"m20260216155600": true,
	"g20220802211500": true,
}

// ErrDatabaseTooOld means a Scrutiny SQLite database predates the minimum upgrade-from version.
var ErrDatabaseTooOld = errors.New("database is older than the minimum upgrade-from version")

// starosMigrationIDs are the schema migrations this fork added, which AnalogJ/scrutiny never
// records.
func starosMigrationIDs() map[string]bool {
	out := map[string]bool{}
	for _, m := range (&scrutinyRepository{}).schemaMigrations(context.Background()) {
		if !upstreamMigrationIDs[m.ID] {
			out[m.ID] = true
		}
	}
	return out
}

// classifySQLiteDatabase decides how a SQLite database is migrated, reading only. It reports
// fresh for a database with no tables, which gets the baseline. Every other database runs the
// migrations it is missing, except a Scrutiny database that has recorded a Staros migration but
// not minimumUpgradeMigrationID, which returns ErrDatabaseTooOld. IDs that are neither upstream
// nor Staros (for example from an unreleased AnalogJ/scrutiny build) do not count as Staros
// history, so such a database still takes the AnalogJ path.
//
// A database with tables but no recorded migrations predates gormigrate (AnalogJ/scrutiny before
// 0.4.0) and keeps running the full history, as it did before the baseline existed.
func classifySQLiteDatabase(db *gorm.DB) (fresh bool, err error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return false, fmt.Errorf("list tables: %w", err)
	}
	userTables := 0
	hasMigrations := false
	for _, table := range tables {
		switch {
		case table == "migrations":
			hasMigrations = true
		case !strings.HasPrefix(table, "sqlite_"):
			userTables++
		}
	}
	var applied []string
	if hasMigrations {
		if err := db.Table("migrations").Pluck("id", &applied).Error; err != nil {
			return false, fmt.Errorf("read migrations table: %w", err)
		}
	}
	if len(applied) == 0 {
		return userTables == 0, nil
	}
	if slices.Contains(applied, minimumUpgradeMigrationID) {
		return false, nil
	}
	staros := starosMigrationIDs()
	newest := ""
	for _, id := range applied {
		if staros[id] && id > newest {
			newest = id
		}
	}
	if newest != "" {
		return false, fmt.Errorf("%w: this database was last migrated by a Scrutiny release older than 1.70.0 "+
			"(its newest migration is %s; %s is required). No schema or data was changed. Start the latest "+
			"Scrutiny 1.76.x release on it once so it migrates, stop it, then start this version",
			ErrDatabaseTooOld, newest, minimumUpgradeMigrationID)
	}
	return false, nil
}

// precheckSQLiteDatabase classifies the SQLite database at path over a read-only connection and
// returns ErrDatabaseTooOld for a database Migrate would refuse, so the refusal happens before the
// normal connection sets journal_mode=WAL on it. A missing file is a new install. Any other
// failure is logged and left to Migrate, which classifies again on the normal connection.
func precheckSQLiteDatabase(path string, logger logrus.FieldLogger) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	db, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		logger.Debugf("SQLite read-only precheck skipped: %v", err)
		return nil
	}
	defer func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	}()
	if _, err := classifySQLiteDatabase(db); err != nil {
		if errors.Is(err, ErrDatabaseTooOld) {
			logger.Errorf("Database migration refused: %v", err)
			return err
		}
		logger.Debugf("SQLite read-only precheck skipped: %v", err)
	}
	return nil
}
