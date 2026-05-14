package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20220716214900"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260122000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260301000000"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createMigrationTestRepository(t *testing.T) *scrutinyRepository {
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

	appliedMigrations := []string{
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
	}
	for _, id := range appliedMigrations {
		require.NoError(t, db.Exec(`INSERT INTO migrations (id) VALUES (?)`, id).Error)
	}

	return &scrutinyRepository{
		gormClient: db,
		logger:     logrus.New(),
	}
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
