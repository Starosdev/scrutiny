package database

import (
	"context"
	"path/filepath"
	"sort"
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

// postgresSchemaDetail describes the current PostgreSQL schema: every column's type, length,
// nullability, and default; every index definition; and every constraint.
func postgresSchemaDetail(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var out []string
	var columns []string
	require.NoError(t, db.Raw(`SELECT table_name || '.' || column_name || ' ' || data_type || ' len=' || coalesce(character_maximum_length::text, '') ||
		' null=' || is_nullable || ' default=' || coalesce(column_default, '<none>')
		FROM information_schema.columns WHERE table_schema = current_schema() AND table_name <> 'migrations'`).Scan(&columns).Error)
	out = append(out, columns...)
	var indexes []string
	require.NoError(t, db.Raw(`SELECT replace(indexdef, current_schema() || '.', '') FROM pg_indexes
		WHERE schemaname = current_schema() AND tablename <> 'migrations'`).Scan(&indexes).Error)
	out = append(out, indexes...)
	var constraints []string
	require.NoError(t, db.Raw(`SELECT c.conrelid::regclass::text || ' ' || c.contype::text || ' ' || pg_get_constraintdef(c.oid)
		FROM pg_constraint c JOIN pg_namespace n ON n.oid = c.connamespace
		WHERE n.nspname = current_schema() AND c.conrelid::regclass::text <> 'migrations'`).Scan(&constraints).Error)
	out = append(out, constraints...)
	sort.Strings(out)
	return out
}

// TestPostgres_SchemaDoesNotDependOnCreationVersion checks that a PostgreSQL database created by
// 1.76.x and upgraded ends with the same schema as one 2.0 creates. The 1.76.x database is built
// the way v1.76.5 built it, independently of createBaselineSchema: AutoMigrate of schemaModels
// plus baselineIndexes, with the only model differences since v1.76.5 undone (the
// api_tokens.scope default and devices.auto_archived; git diff v1.76.5 -- pkg/models) and every
// migration but the two that change them recorded as applied.
func TestPostgres_SchemaDoesNotDependOnCreationVersion(t *testing.T) {
	ctx := context.Background()
	created20 := newPostgresTestRepository(t, newPostgresTestConfig(t))
	require.NoError(t, created20.Migrate(ctx))

	created176 := newPostgresTestRepository(t, newPostgresTestConfig(t))
	require.NoError(t, created176.gormClient.AutoMigrate(schemaModels()...))
	for _, stmt := range baselineIndexes {
		require.NoError(t, created176.gormClient.Exec(stmt).Error)
	}
	require.NoError(t, created176.gormClient.Exec(`CREATE TABLE migrations (id varchar(255) PRIMARY KEY)`).Error)
	for _, m := range created176.schemaMigrations(ctx) {
		require.NoError(t, created176.gormClient.Exec(`INSERT INTO migrations (id) VALUES (?)`, m.ID).Error)
	}
	for _, stmt := range []string{
		`ALTER TABLE api_tokens ALTER COLUMN scope SET DEFAULT 'full'`,
		`ALTER TABLE devices DROP COLUMN auto_archived`,
		`DELETE FROM migrations WHERE id IN ('m20261013000000', 'm20261013000001')`,
		`INSERT INTO api_tokens (name, token_hash, created_at, updated_at) VALUES ('legacy', 'legacy-hash', now(), now())`,
		`INSERT INTO devices (device_id, wwn, archived, created_at, updated_at) VALUES ('d1', 'w1', true, now(), now())`,
	} {
		require.NoError(t, created176.gormClient.Exec(stmt).Error)
	}
	require.NoError(t, created176.Migrate(ctx))

	require.Equal(t, postgresSchemaDetail(t, created20.gormClient), postgresSchemaDetail(t, created176.gormClient))

	var token models.ApiToken
	require.NoError(t, created176.gormClient.Where("token_hash = ?", "legacy-hash").First(&token).Error)
	require.Equal(t, "full", token.Scope, "existing rows are left as they are")
	var device models.Device
	require.NoError(t, created176.gormClient.Where(queryDeviceID, "d1").First(&device).Error)
	require.True(t, device.Archived)
	require.False(t, device.AutoArchived)
}
