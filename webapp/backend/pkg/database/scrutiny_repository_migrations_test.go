package database

import (
	"context"
	"fmt"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
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
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

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

func TestMigrateAttributeOverridesSupportsDistinctDeviceSelectors(t *testing.T) {
	repo := createMigrationTestRepository(t)
	require.NoError(t, repo.Migrate(context.Background()))

	first := models.AttributeOverride{Protocol: "NVMe", AttributeId: "media_errors", DeviceID: "b62e6d86-6ce0-50da-8bff-b2ee54c4af4e", Action: "ignore"}
	second := models.AttributeOverride{Protocol: "NVMe", AttributeId: "media_errors", DeviceID: "c4ac4ff4-1a4d-52aa-9724-40fbc47dd306", Action: "ignore"}
	require.NoError(t, repo.gormClient.Create(&first).Error)
	require.NoError(t, repo.gormClient.Create(&second).Error)

	duplicate := models.AttributeOverride{Protocol: "NVMe", AttributeId: "media_errors", DeviceID: first.DeviceID, Action: "force_status", Status: "passed"}
	require.Error(t, repo.gormClient.Create(&duplicate).Error)
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

func TestMigrateRemovesOrphanBlankDeviceRows(t *testing.T) {
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
		"m20260508000000",
		"m20260510000000",
		"m20260514000000",
		"m20260516000000",
		"m20260523000000",
		"m20260524000000",
	})
	ctx := context.Background()

	// Recreate the devices table with the post-m20260401000000 schema (device_id as primary key)
	// to simulate the state of the database before the m20260528000000 migration runs.
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
	model_family TEXT,
	model_name TEXT,
	interface_type TEXT,
	interface_speed TEXT,
	serial_number TEXT,
	firmware TEXT,
	rotation_speed INTEGER,
	capacity INTEGER,
	form_factor TEXT,
	smart_support TEXT,
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

	// Insert one blank orphan row (all identifying fields empty) and one valid row.
	require.NoError(t, repo.gormClient.Exec(`
		INSERT INTO devices (device_id, wwn, device_name, model_name, serial_number, smart_display_mode)
		VALUES
			('blank-uuid', '', '', '', '', 'scrutiny'),
			('valid-uuid', 'wwn-1', 'sda', 'Samsung SSD 870', 'SN123', 'scrutiny')
	`).Error)

	require.NoError(t, repo.Migrate(ctx))

	var deviceCount int64
	require.NoError(t, repo.gormClient.Raw(`SELECT COUNT(*) FROM devices`).Scan(&deviceCount).Error)
	require.Equal(t, int64(1), deviceCount, "blank orphan row should have been removed")

	var remainingID string
	require.NoError(t, repo.gormClient.Raw(`SELECT device_id FROM devices LIMIT 1`).Scan(&remainingID).Error)
	require.Equal(t, "valid-uuid", remainingID, "valid device should be preserved")
}

func TestMigrateCreatesDeviceSelfTestsTable(t *testing.T) {
	repo := createMigrationTestRepository(t)

	require.NoError(t, repo.Migrate(context.Background()))
	require.True(t, repo.gormClient.Migrator().HasTable(&models.DeviceSelfTest{}))
}

func TestMigrateAddsDashboardPageSizeSetting(t *testing.T) {
	repo := createMigrationTestRepository(t)

	require.NoError(t, repo.Migrate(context.Background()))

	var setting models.SettingEntry
	require.NoError(t, repo.gormClient.Where("setting_key_name = ?", "dashboard_page_size").First(&setting).Error)
	require.Equal(t, "numeric", setting.SettingDataType)
	require.Equal(t, 25, setting.SettingValueNumeric)
}

func TestMigrateAddsDashboardHostPageSizeSetting(t *testing.T) {
	repo := createMigrationTestRepository(t)

	require.NoError(t, repo.Migrate(context.Background()))

	var setting models.SettingEntry
	require.NoError(t, repo.gormClient.Where("setting_key_name = ?", "dashboard_host_page_size").First(&setting).Error)
	require.Equal(t, "numeric", setting.SettingDataType)
	require.Equal(t, 10, setting.SettingValueNumeric)
}

func TestMigrateEnablesTemperatureHistoryStorage(t *testing.T) {
	repo := createMigrationTestRepository(t)

	require.NoError(t, repo.Migrate(context.Background()))

	var setting models.SettingEntry
	require.NoError(t, repo.gormClient.Where("setting_key_name = ?", "collector.store_temperature_history").First(&setting).Error)
	require.Equal(t, "bool", setting.SettingDataType)
	require.True(t, setting.SettingValueBool)
}

func TestTemperatureSettingsPersistAcrossConfigReload(t *testing.T) {
	repo := createMigrationTestRepository(t)
	require.NoError(t, repo.Migrate(context.Background()))
	var err error
	repo.appConfig, err = config.Create()
	require.NoError(t, err)
	settings, err := repo.LoadSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Collector.StoreTempHistory)
	require.False(t, settings.Metrics.NotifyOnTemperature)
	require.Equal(t, models.DefaultTemperatureThresholdCelsius, settings.Metrics.TemperatureThresholdCelsius)
	require.Equal(t, models.DefaultTemperatureDurationMinutes, settings.Metrics.TemperatureDurationMinutes)
	for _, store := range []bool{false, true} {
		settings.Collector.StoreTempHistory = store
		settings.Metrics.NotifyOnTemperature = true
		settings.Metrics.TemperatureThresholdCelsius = 65
		settings.Metrics.TemperatureDurationMinutes = 0
		require.NoError(t, repo.SaveSettings(context.Background(), *settings))
		repo.appConfig, err = config.Create()
		require.NoError(t, err)
		settings, err = repo.LoadSettings(context.Background())
		require.NoError(t, err)
		require.Equal(t, store, settings.Collector.StoreTempHistory)
		require.Equal(t, store, repo.appConfig.GetBool("user.collector.store_temperature_history"))
		require.True(t, settings.Metrics.NotifyOnTemperature)
		require.Equal(t, 65, settings.Metrics.TemperatureThresholdCelsius)
		require.Zero(t, settings.Metrics.TemperatureDurationMinutes)
	}
}

func TestTemperatureStorageMigrationPreservesValues(t *testing.T) {
	for _, value := range []bool{false, true} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			repo := createMigrationTestRepository(t)
			legacy := models.SettingEntry{SettingKeyName: "store_temperature_history", SettingDataType: "bool", SettingValueBool: value}
			require.NoError(t, repo.gormClient.Create(&legacy).Error)
			require.NoError(t, migrateTemperatureStorageKey(repo.gormClient))
			require.NoError(t, migrateTemperatureStorageKey(repo.gormClient))
			var saved models.SettingEntry
			require.NoError(t, repo.gormClient.First(&saved, legacy.ID).Error)
			require.Equal(t, "collector.store_temperature_history", saved.SettingKeyName)
			require.Equal(t, value, saved.SettingValueBool)
		})
	}
}

func TestTemperatureMigrationsPreserveExistingCanonicalSettings(t *testing.T) {
	repo := createMigrationTestRepository(t)
	entries := []models.SettingEntry{
		{SettingKeyName: "store_temperature_history", SettingDataType: "bool", SettingValueBool: true},
		{SettingKeyName: "collector.store_temperature_history", SettingDataType: "bool", SettingValueBool: false},
		{SettingKeyName: "metrics.temperature_duration_minutes", SettingDataType: "numeric", SettingValueNumeric: 0},
		{SettingKeyName: "metrics.notify_on_temperature", SettingDataType: "bool", SettingValueBool: true},
		{SettingKeyName: "metrics.temperature_threshold_celsius", SettingDataType: "numeric", SettingValueNumeric: 65},
	}
	require.NoError(t, repo.gormClient.Create(&entries).Error)
	for range 2 {
		require.NoError(t, migrateTemperatureStorageKey(repo.gormClient))
		require.NoError(t, migrateTemperatureNotificationSettings(repo.gormClient))
	}
	var saved models.SettingEntry
	require.NoError(t, repo.gormClient.First(&saved, entries[1].ID).Error)
	require.False(t, saved.SettingValueBool)
	var duration models.SettingEntry
	require.NoError(t, repo.gormClient.First(&duration, entries[2].ID).Error)
	require.Zero(t, duration.SettingValueNumeric)
	var enabled, threshold models.SettingEntry
	require.NoError(t, repo.gormClient.First(&enabled, entries[3].ID).Error)
	require.True(t, enabled.SettingValueBool)
	require.NoError(t, repo.gormClient.First(&threshold, entries[4].ID).Error)
	require.Equal(t, 65, threshold.SettingValueNumeric)
	var count int64
	require.NoError(t, repo.gormClient.Model(&models.SettingEntry{}).Count(&count).Error)
	require.EqualValues(t, 4, count)
}

func TestMigrateSelfTestChronologyPreservesLegacyHistory(t *testing.T) {
	repo := createMigrationTestRepository(t)
	require.NoError(t, repo.gormClient.Exec(`CREATE TABLE device_self_tests (
        id INTEGER PRIMARY KEY, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME,
        device_identity TEXT, device_id TEXT, device_wwn TEXT, type_value INTEGER,
        type_string TEXT, status_value INTEGER, status_string TEXT, status_passed NUMERIC, lifetime_hours INTEGER
    )`).Error)
	require.NoError(t, repo.gormClient.Exec(`CREATE UNIQUE INDEX idx_device_self_tests_identity ON device_self_tests(device_identity,type_value,lifetime_hours)`).Error)
	require.NoError(t, repo.gormClient.Exec(`CREATE INDEX idx_device_self_tests_history ON device_self_tests(device_identity,lifetime_hours)`).Error)
	require.NoError(t, repo.gormClient.Exec(`INSERT INTO device_self_tests(id,device_identity,device_id,device_wwn,type_value,lifetime_hours) VALUES (42,'wwn-1','device-1','wwn-1',1,2464)`).Error)
	require.NoError(t, repo.gormClient.Exec(`INSERT INTO migrations(id) VALUES ('m20260616000000')`).Error)
	require.NoError(t, repo.Migrate(context.Background()))
	require.NoError(t, repo.Migrate(context.Background()))
	var row models.DeviceSelfTest
	require.NoError(t, repo.gormClient.First(&row, 42).Error)
	require.Equal(t, 2464, row.LifetimeHours)
	require.Nil(t, row.EffectiveLifetimeHours)
	require.Zero(t, row.ObservedAt)
	require.False(t, repo.gormClient.Migrator().HasIndex(&models.DeviceSelfTest{}, "idx_device_self_tests_identity"))
	require.NoError(t, repo.gormClient.Exec(`INSERT INTO device_self_tests(device_identity,type_value,lifetime_hours,effective_lifetime_hours) VALUES ('wwn-1',1,2464,68000)`).Error)
}

// A pre-existing override must survive the pinned_value migration with the column
// NULL rather than 0: zero is a legitimate acknowledged value, so a defaulted column
// would read as "acknowledged at 0" for every override created before the feature.
func TestMigrateAttributeOverridesAddsNullablePinnedValue(t *testing.T) {
	repo := createMigrationTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Migrate(ctx))

	existing := models.AttributeOverride{Protocol: "NVMe", AttributeId: "media_errors", DeviceID: "b62e6d86-6ce0-50da-8bff-b2ee54c4af4e", Action: "ignore"}
	require.NoError(t, repo.gormClient.Create(&existing).Error)

	var stored models.AttributeOverride
	require.NoError(t, repo.gormClient.First(&stored, existing.ID).Error)
	require.Nil(t, stored.PinnedValue)

	pinned := int64(0)
	acknowledged := models.AttributeOverride{
		Protocol:    "NVMe",
		AttributeId: "media_errors",
		DeviceID:    "c4ac4ff4-1a4d-52aa-9724-40fbc47dd306",
		Action:      "acknowledge",
		PinnedValue: &pinned,
	}
	require.NoError(t, repo.gormClient.Create(&acknowledged).Error)

	var readBack models.AttributeOverride
	require.NoError(t, repo.gormClient.First(&readBack, acknowledged.ID).Error)
	require.NotNil(t, readBack.PinnedValue)
	require.Equal(t, int64(0), *readBack.PinnedValue)
}
