package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// productionGormConfig matches newGormConfig on the settings that change DDL.
func productionGormConfig() *gorm.Config {
	return &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true}
}

// openSQLiteRepo opens path with the production DDL settings.
func openSQLiteRepo(t *testing.T, path string) *scrutinyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), productionGormConfig())
	require.NoError(t, err)
	repo := &scrutinyRepository{gormClient: db, logger: logrus.New()}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestAPITokenWithoutScopeIsNotGivenLegacyFull(t *testing.T) {
	repo := openSQLiteRepo(t, filepath.Join(t.TempDir(), "tokens.db"))
	require.NoError(t, repo.Migrate(context.Background()))

	token := models.ApiToken{Name: "no scope", TokenHash: "h"}
	require.NoError(t, repo.gormClient.Create(&token).Error)
	var stored models.ApiToken
	require.NoError(t, repo.gormClient.First(&stored, token.ID).Error)
	require.Empty(t, stored.Scope)
	require.False(t, models.IsValidAPITokenScope(stored.Scope))
}

func TestAutoArchivedFlag(t *testing.T) {
	ctx := context.Background()
	repo := openSQLiteRepo(t, filepath.Join(t.TempDir(), "devices.db"))
	require.NoError(t, repo.Migrate(ctx))
	for _, id := range []string{"a", "b"} {
		require.NoError(t, repo.gormClient.Create(&models.Device{DeviceID: id, WWN: "wwn-" + id, HostId: "nas"}).Error)
	}
	load := func(id string) models.Device {
		var d models.Device
		require.NoError(t, repo.gormClient.Where(queryDeviceID, id).First(&d).Error)
		return d
	}

	require.NoError(t, repo.AutoArchiveDevice(ctx, "a"))
	require.True(t, load("a").Archived)
	require.True(t, load("a").AutoArchived)

	require.NoError(t, repo.UpdateDeviceArchived(ctx, "a", false))
	require.False(t, load("a").Archived)
	require.False(t, load("a").AutoArchived, "a manual unarchive clears the flag")

	require.NoError(t, repo.UpdateDeviceArchived(ctx, "b", true))
	require.True(t, load("b").Archived)
	require.False(t, load("b").AutoArchived, "a manual archive is not an auto-archive")

	require.NoError(t, repo.AutoArchiveDevice(ctx, "a"))
	_, err := repo.UpdateHostArchived(ctx, "nas", true)
	require.NoError(t, err)
	require.False(t, load("a").AutoArchived, "a host archive is manual")
}

// A PostgreSQL database created by 1.76.x has the scope default and lacks auto_archived; the
// forward migrations must bring it to the baseline a new PostgreSQL database gets.
func TestPostgres_ScopeDefaultAndAutoArchivedMigrationsRun(t *testing.T) {
	ctx := context.Background()
	repo := newPostgresTestRepository(t, newPostgresTestConfig(t))
	require.NoError(t, repo.Migrate(ctx))

	columnDefault := func(table, column string) *string {
		var dflt *string
		require.NoError(t, repo.gormClient.Raw(
			`SELECT column_default FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			table, column).Scan(&dflt).Error)
		return dflt
	}
	require.Nil(t, columnDefault("api_tokens", "scope"), "a new database has no scope default")
	baselineAutoArchived := columnDefault("devices", "auto_archived")
	require.NotNil(t, baselineAutoArchived)

	for _, stmt := range []string{
		`ALTER TABLE api_tokens ALTER COLUMN scope SET DEFAULT 'full'`,
		`ALTER TABLE devices DROP COLUMN auto_archived`,
		`DELETE FROM migrations WHERE id IN ('m20261013000000', 'm20261013000001')`,
		`INSERT INTO api_tokens (name, token_hash, created_at, updated_at) VALUES ('legacy', 'legacy-hash', now(), now())`,
		`INSERT INTO devices (device_id, wwn, archived, created_at, updated_at) VALUES ('d1', 'w1', true, now(), now())`,
	} {
		require.NoError(t, repo.gormClient.Exec(stmt).Error)
	}
	require.NoError(t, repo.Migrate(ctx))

	require.Nil(t, columnDefault("api_tokens", "scope"))
	require.Equal(t, baselineAutoArchived, columnDefault("devices", "auto_archived"))
	var token models.ApiToken
	require.NoError(t, repo.gormClient.Where("token_hash = ?", "legacy-hash").First(&token).Error)
	require.Equal(t, "full", token.Scope, "existing rows are left as they are")
	var device models.Device
	require.NoError(t, repo.gormClient.Where(queryDeviceID, "d1").First(&device).Error)
	require.True(t, device.Archived)
	require.False(t, device.AutoArchived)
}
