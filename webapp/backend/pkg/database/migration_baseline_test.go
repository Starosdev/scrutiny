package database

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The fixtures in testdata are real databases, not reconstructions: each was produced by running
// that release's own Migrate on an empty SQLite file and inserting a few rows with plain SQL.
//   - scrutiny-v1.70.0.db.gz: Staros v1.70.0, the minimum upgrade-from version. Two devices (one
//     archived), an attribute override, a notify URL, an API token holding the legacy 'full' scope,
//     and dashboard_page_size changed to 50. It still has the UNIQUE idx_devices_wwn from #851.
//   - scrutiny-v1.69.0.db.gz: Staros v1.69.0, one release too old. Two devices.
//   - scrutiny-analogj-v0.9.5.db.gz: AnalogJ/scrutiny v0.9.5, with one device whose WWN and
//     serial are empty, so the AnalogJ repair (#694) needs no InfluxDB here.

// openFixture copies a gzipped fixture to a temporary file and returns its path.
func openFixture(t *testing.T, name string) string {
	t.Helper()
	in, err := os.Open(filepath.Join("testdata", name+".gz"))
	require.NoError(t, err)
	defer func() { _ = in.Close() }()
	gz, err := gzip.NewReader(in)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	out, err := os.Create(path)
	require.NoError(t, err)
	_, err = io.Copy(out, gz) //nolint:gosec // trusted test fixture
	require.NoError(t, err)
	require.NoError(t, out.Close())
	return path
}

func appliedMigrationIDs(t *testing.T, db *gorm.DB) map[string]bool {
	t.Helper()
	var ids []string
	require.NoError(t, db.Table("migrations").Pluck("id", &ids).Error)
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func requireEveryMigrationApplied(t *testing.T, db *gorm.DB) {
	t.Helper()
	applied := appliedMigrationIDs(t, db)
	for _, m := range (&scrutinyRepository{}).schemaMigrations(context.Background()) {
		require.True(t, applied[m.ID], "migration %s not recorded", m.ID)
	}
}

func fileSHA256(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test temp file
	require.NoError(t, err)
	return sha256.Sum256(data)
}

// freshBaselineSchema is the schema Migrate gives an empty SQLite database.
func freshBaselineSchema(t *testing.T) map[string][]string {
	t.Helper()
	repo := openSQLiteRepo(t, filepath.Join(t.TempDir(), "fresh.db"))
	require.NoError(t, repo.Migrate(context.Background()))
	return sqliteSchemaDetail(t, repo.gormClient)
}

func TestMigrateFreshSQLiteInstallUsesBaseline(t *testing.T) {
	ctx := context.Background()
	repo := openSQLiteRepo(t, filepath.Join(t.TempDir(), "scrutiny.db"))
	require.NoError(t, repo.Migrate(ctx))

	// SCHEMA_INIT is written only by the gormigrate InitSchema hook, so its presence proves the
	// baseline ran instead of the history.
	applied := appliedMigrationIDs(t, repo.gormClient)
	require.True(t, applied["SCHEMA_INIT"], "fresh install did not take the baseline path")
	requireEveryMigrationApplied(t, repo.gormClient)

	chain := migratedSQLite(t)
	require.Empty(t, schemaDetailDiff(sqliteSchemaDetail(t, chain), sqliteSchemaDetail(t, repo.gormClient)), "chain (-) vs fresh install (+)")

	var chainSettings, freshSettings []models.SettingEntry
	require.NoError(t, chain.Order("setting_key_name").Find(&chainSettings).Error)
	require.NoError(t, repo.gormClient.Order("setting_key_name").Find(&freshSettings).Error)
	require.Equal(t, settingValues(uniqueSettings(chainSettings)), settingValues(freshSettings))

	for _, model := range schemaModels() {
		if _, ok := model.(*models.SettingEntry); ok {
			continue
		}
		var chainRows, freshRows int64
		require.NoError(t, chain.Model(model).Count(&chainRows).Error)
		require.NoError(t, repo.gormClient.Model(model).Count(&freshRows).Error)
		require.Equal(t, chainRows, freshRows, "%T row count", model)
	}

	require.NoError(t, repo.Migrate(ctx), "a second run is a no-op")
	require.Equal(t, applied, appliedMigrationIDs(t, repo.gormClient))
}

func TestMigrateRefusesDatabaseOlderThanMinimumWithoutChangingIt(t *testing.T) {
	path := openFixture(t, "scrutiny-v1.69.0.db")
	before := fileSHA256(t, path)

	repo := openSQLiteRepo(t, path)
	err := repo.Migrate(context.Background())
	require.ErrorIs(t, err, ErrDatabaseTooOld)
	require.Contains(t, err.Error(), "1.76.x")
	require.Contains(t, err.Error(), "m20260809000000")
	require.NoError(t, repo.Close())

	require.Equal(t, before, fileSHA256(t, path), "the refused database was modified")
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		_, statErr := os.Stat(path + suffix)
		require.True(t, os.IsNotExist(statErr), "unexpected %s file", suffix)
	}
}

func TestMigrateUpgradesMinimumVersionDatabase(t *testing.T) {
	ctx := context.Background()
	repo := openSQLiteRepo(t, openFixture(t, "scrutiny-v1.70.0.db"))
	require.True(t, appliedMigrationIDs(t, repo.gormClient)[minimumUpgradeMigrationID])

	require.NoError(t, repo.Migrate(ctx))

	applied := appliedMigrationIDs(t, repo.gormClient)
	require.False(t, applied["SCHEMA_INIT"], "an existing database must not take the baseline path")
	requireEveryMigrationApplied(t, repo.gormClient)
	require.Empty(t, schemaDetailDiff(freshBaselineSchema(t), sqliteSchemaDetail(t, repo.gormClient)), "fresh install (-) vs upgraded 1.70.0 (+)")

	var devices []models.Device
	require.NoError(t, repo.gormClient.Order("device_name").Find(&devices).Error)
	require.Len(t, devices, 2)
	require.Equal(t, "ZDH1ABCD", devices[0].SerialNumber)
	require.False(t, devices[0].Archived)
	require.True(t, devices[1].Archived)
	require.False(t, devices[1].AutoArchived, "existing archives are manual")

	var override models.AttributeOverride
	require.NoError(t, repo.gormClient.First(&override).Error)
	require.Equal(t, "ignore", override.Action)
	require.Equal(t, devices[0].DeviceID, override.DeviceID)

	var notifyURLs []models.NotifyUrl
	require.NoError(t, repo.gormClient.Find(&notifyURLs).Error)
	require.Len(t, notifyURLs, 1)
	require.True(t, notifyURLs[0].HeartbeatEnabled, "pre-existing URLs keep the heartbeat default")

	// The legacy 'full' token survives the api_tokens rebuild unchanged, and still cannot
	// authenticate, since 'full' is not a valid scope.
	var token models.ApiToken
	require.NoError(t, repo.gormClient.Where("token_hash = ?", "legacy-hash").First(&token).Error)
	require.Equal(t, "full", token.Scope)
	require.False(t, models.IsValidAPITokenScope(token.Scope))
	require.Error(t, repo.gormClient.Create(&models.ApiToken{Name: "dup", TokenHash: "legacy-hash", Scope: models.APITokenScopeRead}).Error,
		"the token_hash unique index survives the rebuild")

	var pageSize models.SettingEntry
	require.NoError(t, repo.gormClient.Where("setting_key_name = ?", "dashboard_page_size").First(&pageSize).Error)
	require.Equal(t, 50, pageSize.SettingValueNumeric, "user settings survive the upgrade")

	require.NoError(t, repo.Migrate(ctx), "a second run is a no-op")
}

func TestMigrateRunsFullHistoryForAnalogJDatabase(t *testing.T) {
	ctx := context.Background()
	repo := openSQLiteRepo(t, openFixture(t, "scrutiny-analogj-v0.9.5.db"))
	require.True(t, appliedMigrationIDs(t, repo.gormClient)[analogJScrutinyUUIDMigrationID])

	fresh, err := classifySQLiteDatabase(repo.gormClient)
	require.NoError(t, err)
	require.False(t, fresh)

	require.NoError(t, repo.Migrate(ctx))
	requireEveryMigrationApplied(t, repo.gormClient)
	require.False(t, appliedMigrationIDs(t, repo.gormClient)["SCHEMA_INIT"])
	require.Empty(t, schemaDetailDiff(freshBaselineSchema(t), sqliteSchemaDetail(t, repo.gormClient)), "fresh install (-) vs upgraded AnalogJ (+)")

	var devices []models.Device
	require.NoError(t, repo.gormClient.Find(&devices).Error)
	require.Len(t, devices, 1)
	require.Equal(t, "sdc", devices[0].DeviceName)
	require.NotEmpty(t, devices[0].DeviceID)
}

func TestClassifySQLiteDatabase(t *testing.T) {
	cases := []struct {
		name      string
		setup     []string
		wantFresh bool
		wantErr   error
	}{
		{name: "empty", wantFresh: true},
		{name: "empty migrations table", setup: []string{"CREATE TABLE migrations (id TEXT PRIMARY KEY)"}, wantFresh: true},
		{name: "tables without history", setup: []string{"CREATE TABLE devices (wwn TEXT)"}},
		{name: "tables with empty history", setup: []string{"CREATE TABLE devices (wwn TEXT)", "CREATE TABLE migrations (id TEXT PRIMARY KEY)"}},
		{name: "upstream history", setup: []string{
			"CREATE TABLE migrations (id TEXT PRIMARY KEY)",
			"INSERT INTO migrations VALUES ('20201107210306'), ('m20250221084400'), ('m20260216155600'), ('g20220802211500')",
		}},
		{name: "at minimum version", setup: []string{
			"CREATE TABLE migrations (id TEXT PRIMARY KEY)",
			"INSERT INTO migrations VALUES ('m20250609210800'), ('" + minimumUpgradeMigrationID + "')",
		}},
		{name: "staros history before minimum", setup: []string{
			"CREATE TABLE migrations (id TEXT PRIMARY KEY)",
			"INSERT INTO migrations VALUES ('m20250221084400'), ('m20250609210800'), ('m20260809000000')",
		}, wantErr: ErrDatabaseTooOld},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "c.db")), productionGormConfig())
			require.NoError(t, err)
			for _, stmt := range tc.setup {
				require.NoError(t, db.Exec(stmt).Error)
			}
			fresh, err := classifySQLiteDatabase(db)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantFresh, fresh)
		})
	}
}
