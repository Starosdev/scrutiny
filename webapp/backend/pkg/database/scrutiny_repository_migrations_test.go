package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20220716214900"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260122000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260301000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createMigrationTestRepositoryWithAppliedMigrations(t *testing.T, appliedMigrations []string) *scrutinyRepository {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "scrutiny.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&m20260301000000.Device{},
		&m20220716214900.Setting{},
		&m20260122000000.AttributeOverride{},
	))

	require.NoError(t, db.Exec(`CREATE TABLE migrations (id TEXT NOT NULL PRIMARY KEY)`).Error)

	for _, id := range appliedMigrations {
		require.NoError(t, db.Exec(`INSERT INTO migrations (id) VALUES (?)`, id).Error)
	}

	return &scrutinyRepository{
		gormClient: db,
		logger:     logrus.New(),
	}
}

func createMigrationTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	return createMigrationTestRepositoryWithAppliedMigrations(t, []string{
		"20201107210306",
		"20220503113100",
		"20220503120000",
		"m20220509170100",
		"m20220709181300",
		"m20220716214900",
		"m20250221084400",
		"m20251108044508",
		"m20260108000000",
		"m20260122000000",
		"m20260129000000",
		"m20260131000000",
		"m20260202000000",
		"m20260225000000",
		"m20260226000000",
		"m20260301000000",
		"m20260514000000",
		"m20260516000000",
	})
}

func TestMigrateBackfillsDistinctDeviceIDsForLegacyDevicesWithMissingWWN(t *testing.T) {
	repo := createMigrationTestRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.gormClient.Exec(`
		INSERT INTO devices (wwn, model_name, serial_number, smart_display_mode)
		VALUES
			(NULL, 'NVMe Drive', 'SERIAL001', 'scrutiny'),
			(NULL, 'NVMe Drive', 'SERIAL002', 'scrutiny')
	`).Error)

	err := repo.Migrate(ctx)
	require.NoError(t, err)

	var deviceCount int64
	require.NoError(t, repo.gormClient.Raw(`SELECT COUNT(*) FROM devices`).Scan(&deviceCount).Error)
	require.Equal(t, int64(2), deviceCount)

	var distinctDeviceIDCount int64
	require.NoError(t, repo.gormClient.Raw(`
		SELECT COUNT(DISTINCT device_id)
		FROM devices
		WHERE device_id IS NOT NULL AND device_id != ''
	`).Scan(&distinctDeviceIDCount).Error)
	require.Equal(t, int64(2), distinctDeviceIDCount)

	var nullWWNCount int64
	require.NoError(t, repo.gormClient.Raw(`SELECT COUNT(*) FROM devices WHERE wwn IS NULL`).Scan(&nullWWNCount).Error)
	require.Equal(t, int64(2), nullWWNCount)
}

func TestMigratePreservesDeviceColumnsAcrossSQLiteTableRebuilds(t *testing.T) {
	repo := createMigrationTestRepositoryWithAppliedMigrations(t, []string{
		"20201107210306",
		"20220503113100",
		"20220503120000",
		"m20220509170100",
		"m20220709181300",
		"m20220716214900",
		"m20250221084400",
		"m20251108044508",
		"m20260108000000",
		"m20260122000000",
		"m20260129000000",
		"m20260131000000",
		"m20260202000000",
		"m20260225000000",
		"m20260226000000",
		"m20260301000000",
		"m20260315000000",
		"m20260401000000",
		"m20260402000000",
		"m20260410000000",
		"m20260411000000",
		"m20260413000000",
		"m20260414000000",
		"m20260421000000",
	})
	ctx := context.Background()

	require.NoError(t, repo.gormClient.Exec(`DROP TABLE devices`).Error)
	require.NoError(t, repo.gormClient.Exec(`
CREATE TABLE devices (
	device_id TEXT PRIMARY KEY,
	wwn TEXT,
	created_at DATETIME,
	updated_at DATETIME,
	deleted_at DATETIME,
	device_name TEXT,
	device_uuid TEXT,
	device_serial_id TEXT,
	device_label TEXT,
	manufacturer TEXT,
	model_name TEXT,
	interface_type TEXT,
	interface_speed TEXT,
	serial_number TEXT,
	firmware TEXT,
	rotation_speed INTEGER,
	capacity INTEGER,
	form_factor TEXT,
	smart_support NUMERIC,
	device_protocol TEXT,
	device_type TEXT,
	label TEXT,
	host_id TEXT,
	collector_version TEXT,
	smart_display_mode TEXT DEFAULT 'scrutiny',
	device_status INTEGER,
	has_forced_failure NUMERIC DEFAULT 0,
	archived NUMERIC,
	muted NUMERIC,
	missed_ping_timeout_override INTEGER DEFAULT 0
)`).Error)

	require.NoError(t, repo.gormClient.Exec(`
		INSERT INTO devices (
			device_id, wwn, created_at, updated_at, deleted_at,
			device_name, device_uuid, device_serial_id, device_label,
			manufacturer, model_name, interface_type, interface_speed,
			serial_number, firmware, rotation_speed, capacity, form_factor,
			smart_support, device_protocol, device_type, label, host_id,
			collector_version, smart_display_mode, device_status,
			has_forced_failure, archived, muted, missed_ping_timeout_override
		) VALUES (
			'dev1', 'wwn1', '2026-05-01 12:00:00', '2026-05-02 12:00:00', NULL,
			'disk0', 'uuid1', 'serialid1', 'label1',
			'Seagate', 'IronWolf', 'SATA', '6 Gbps',
			'SN123', 'FW1', 7200, 4000, '3.5',
			1, 'ata', 'hdd', 'NAS', 'host1',
			'v1', 'scrutiny', 5,
			1, 1, 1, 42
		)
	`).Error)

	err := repo.Migrate(ctx)
	require.NoError(t, err)

	var deviceCount int64
	require.NoError(t, repo.gormClient.Raw(`SELECT COUNT(*) FROM devices`).Scan(&deviceCount).Error)
	require.Equal(t, int64(1), deviceCount)

	rows, err := repo.gormClient.Raw(`
		SELECT
			device_id, wwn, device_name, device_uuid, device_serial_id, device_label,
			manufacturer, COALESCE(model_family, ''), model_name, interface_type, interface_speed, serial_number,
			firmware, rotation_speed, capacity, form_factor, smart_support,
			device_protocol, device_type, label, host_id, collector_version,
			smart_display_mode, device_status, has_forced_failure, archived, muted,
			missed_ping_timeout_override
		FROM devices
		LIMIT 1
	`).Rows()
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())

	var (
		deviceID, wwn, deviceName, deviceUUID, deviceSerialID, deviceLabel  string
		manufacturer, modelFamily, modelName, interfaceType, interfaceSpeed string
		serialNumber, firmware, formFactor, smartSupport                    string
		deviceProtocol, deviceType, label, hostID, collectorVersion         string
		smartDisplayMode                                                    string
		rotationSpeed, capacity, deviceStatus                               int64
		hasForcedFailure, archived, muted                                   bool
		missedPingTimeoutOverride                                           int64
	)
	require.NoError(t, rows.Scan(
		&deviceID, &wwn, &deviceName, &deviceUUID, &deviceSerialID, &deviceLabel,
		&manufacturer, &modelFamily, &modelName, &interfaceType, &interfaceSpeed, &serialNumber,
		&firmware, &rotationSpeed, &capacity, &formFactor, &smartSupport,
		&deviceProtocol, &deviceType, &label, &hostID, &collectorVersion,
		&smartDisplayMode, &deviceStatus, &hasForcedFailure, &archived, &muted,
		&missedPingTimeoutOverride,
	))

	require.NotEmpty(t, deviceID)
	require.Equal(t, "wwn1", wwn)
	require.Equal(t, "disk0", deviceName)
	require.Equal(t, "uuid1", deviceUUID)
	require.Equal(t, "serialid1", deviceSerialID)
	require.Equal(t, "label1", deviceLabel)
	require.Equal(t, "Seagate", manufacturer)
	require.Equal(t, "", modelFamily)
	require.Equal(t, "IronWolf", modelName)
	require.Equal(t, "SATA", interfaceType)
	require.Equal(t, "6 Gbps", interfaceSpeed)
	require.Equal(t, "SN123", serialNumber)
	require.Equal(t, "FW1", firmware)
	require.Equal(t, int64(7200), rotationSpeed)
	require.Equal(t, int64(4000), capacity)
	require.Equal(t, "3.5", formFactor)
	require.Equal(t, `{"available":true}`, smartSupport)
	require.Equal(t, "ata", deviceProtocol)
	require.Equal(t, "hdd", deviceType)
	require.Equal(t, "NAS", label)
	require.Equal(t, "host1", hostID)
	require.Equal(t, "v1", collectorVersion)
	require.Equal(t, "scrutiny", smartDisplayMode)
	require.Equal(t, int64(5), deviceStatus)
	require.True(t, hasForcedFailure)
	require.True(t, archived)
	require.True(t, muted)
	require.Equal(t, int64(42), missedPingTimeoutOverride)
}

func TestAttributeOverridesSchemaSurvivesLaterAutoMigrate(t *testing.T) {
	repo := createMigrationTestRepositoryWithAppliedMigrations(t, []string{
		"20201107210306",
		"20220503113100",
		"20220503120000",
		"m20220509170100",
		"m20220709181300",
		"m20220716214900",
		"m20250221084400",
		"m20251108044508",
		"m20260108000000",
		"m20260122000000",
		"m20260129000000",
		"m20260131000000",
		"m20260202000000",
		"m20260225000000",
		"m20260226000000",
		"m20260301000000",
		"m20260315000000",
		"m20260401000000",
		"m20260402000000",
		"m20260410000000",
		"m20260411000000",
		"m20260413000000",
		"m20260421000000",
		"m20260508000000",
		"m20260510000000",
		"m20260514000000",
		"m20260516000000",
	})
	ctx := context.Background()

	require.NoError(t, repo.gormClient.Exec(`DROP TABLE attribute_overrides`).Error)
	require.NoError(t, repo.gormClient.Exec(`
CREATE TABLE attribute_overrides (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME,
	updated_at DATETIME,
	protocol TEXT NOT NULL,
	attribute_id TEXT NOT NULL,
	wwn TEXT DEFAULT '',
	action TEXT DEFAULT '',
	status TEXT DEFAULT '',
	warn_above INTEGER,
	fail_above INTEGER,
	source TEXT DEFAULT 'ui',
	deleted_at DATETIME
)`).Error)

	require.NoError(t, repo.gormClient.Exec(`CREATE UNIQUE INDEX idx_override_lookup ON attribute_overrides (protocol, attribute_id, wwn)`).Error)
	require.NoError(t, repo.gormClient.Exec(`
		INSERT INTO attribute_overrides (
			id, created_at, updated_at, protocol, attribute_id, wwn,
			action, status, warn_above, fail_above, source, deleted_at
		) VALUES (
			1, '2026-05-01 12:00:00', '2026-05-02 12:00:00', 'NVMe', 'media_errors', 'wwn1',
			'set_threshold', 'warn', 10, 20, 'ui', NULL
		)
	`).Error)

	require.NoError(t, repo.Migrate(ctx))
	require.NoError(t, repo.gormClient.AutoMigrate(&models.AttributeOverride{}))

	rows, err := repo.gormClient.Raw(`
		SELECT
			id, protocol, attribute_id, wwn, action, status, warn_above, fail_above, source
		FROM attribute_overrides
		WHERE id = 1
	`).Rows()
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())

	var (
		id                         int64
		protocol, attributeID, wwn string
		action, status, source     string
		warnAbove, failAbove       int64
	)
	require.NoError(t, rows.Scan(
		&id, &protocol, &attributeID, &wwn, &action, &status, &warnAbove, &failAbove, &source,
	))

	require.Equal(t, int64(1), id)
	require.Equal(t, "NVMe", protocol)
	require.Equal(t, "media_errors", attributeID)
	require.Equal(t, "wwn1", wwn)
	require.Equal(t, "set_threshold", action)
	require.Equal(t, "warn", status)
	require.Equal(t, int64(10), warnAbove)
	require.Equal(t, int64(20), failAbove)
	require.Equal(t, "ui", source)
}

func TestMigrateSelfHealsDriftedDeviceSchemaWhenMigrationWasRecorded(t *testing.T) {
	repo := createMigrationTestRepositoryWithAppliedMigrations(t, []string{
		"20201107210306",
		"20220503113100",
		"20220503120000",
		"m20220509170100",
		"m20220709181300",
		"m20220716214900",
		"m20250221084400",
		"m20251108044508",
		"m20260108000000",
		"m20260122000000",
		"m20260129000000",
		"m20260131000000",
		"m20260202000000",
		"m20260225000000",
		"m20260226000000",
		"m20260301000000",
		"m20260315000000",
		"m20260401000000",
		"m20260402000000",
		"m20260410000000",
		"m20260411000000",
		"m20260413000000",
		"m20260421000000",
		"m20260508000000",
		"m20260510000000",
		"m20260514000000",
		"m20260516000000",
		"m20260523000000",
		"m20260524000000",
	})
	ctx := context.Background()

	require.NoError(t, repo.gormClient.Exec(`DROP TABLE devices`).Error)
	require.NoError(t, repo.gormClient.Exec(`
CREATE TABLE devices (
	device_id TEXT PRIMARY KEY,
	wwn TEXT,
	created_at DATETIME,
	updated_at DATETIME,
	deleted_at DATETIME,
	device_name TEXT,
	device_uuid TEXT,
	device_serial_id TEXT,
	device_label TEXT,
	manufacturer TEXT,
	model_name TEXT,
	interface_type TEXT,
	interface_speed TEXT,
	serial_number TEXT,
	firmware TEXT,
	rotation_speed INTEGER,
	capacity INTEGER,
	form_factor TEXT,
	smart_support NUMERIC,
	device_protocol TEXT,
	device_type TEXT,
	label TEXT,
	host_id TEXT,
	collector_version TEXT
)`).Error)

	require.NoError(t, repo.Migrate(ctx))
	require.True(t, repo.gormClient.Migrator().HasColumn(&models.Device{}, "model_family"))
	require.True(t, repo.gormClient.Migrator().HasColumn(&models.Device{}, "smart_display_mode"))
	require.True(t, repo.gormClient.Migrator().HasColumn(&models.Device{}, "device_status"))
	require.True(t, repo.gormClient.Migrator().HasColumn(&models.Device{}, "has_forced_failure"))
	require.True(t, repo.gormClient.Migrator().HasColumn(&models.Device{}, "archived"))
	require.True(t, repo.gormClient.Migrator().HasColumn(&models.Device{}, "muted"))
	require.True(t, repo.gormClient.Migrator().HasColumn(&models.Device{}, "missed_ping_timeout_override"))
}
