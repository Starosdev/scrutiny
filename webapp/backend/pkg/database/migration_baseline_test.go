package database

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

// sqliteProductionConfig is the configuration NewScrutinyRepository and openSQLiteDatabase read
// for a SQLite database at path, with the default journal_mode=WAL connection pragma. Any other
// key, such as the InfluxDB settings, is an unexpected call that fails the test.
func sqliteProductionConfig(t *testing.T, path string) *mock_config.MockInterface {
	t.Helper()
	cfg := mock_config.NewMockInterface(gomock.NewController(t))
	cfg.EXPECT().GetString("web.database.type").Return("sqlite").AnyTimes()
	cfg.EXPECT().GetString("web.database.location").Return(path).AnyTimes()
	cfg.EXPECT().GetString("web.database.journal_mode").Return("").AnyTimes()
	cfg.EXPECT().GetString("log.level").Return("INFO").AnyTimes()
	return cfg
}

// sqliteContents lists every schema object and every table's row count, read over a read-only
// connection.
func sqliteContents(t *testing.T, path string) []string {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{})
	require.NoError(t, err)
	defer func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	}()
	var objects []struct{ Type, Name, SQL string }
	require.NoError(t, db.Raw(`SELECT type, name, coalesce(sql, '') AS sql FROM sqlite_master ORDER BY type, name`).Scan(&objects).Error)
	var out []string
	for _, o := range objects {
		out = append(out, o.Type+" "+o.Name+" "+o.SQL)
		if o.Type == "table" {
			var n int64
			require.NoError(t, db.Table(o.Name).Count(&n).Error)
			out = append(out, fmt.Sprintf("rows %s %d", o.Name, n))
		}
	}
	return out
}

// The production startup path refuses a too-old database over a read-only connection, before the
// journal_mode=WAL pragma or the InfluxDB setup, so the file is byte-for-byte unchanged.
func TestNewScrutinyRepositoryRefusesDatabaseOlderThanMinimumWithoutChangingIt(t *testing.T) {
	ResetMigrationGuardForTests()
	path := openFixture(t, "scrutiny-v1.69.0.db")
	before := fileSHA256(t, path)

	_, err := NewScrutinyRepository(sqliteProductionConfig(t, path), logrus.New())
	require.ErrorIs(t, err, ErrDatabaseTooOld)
	require.Contains(t, err.Error(), "1.76.x")
	require.Contains(t, err.Error(), "m20260809000000")

	require.Equal(t, before, fileSHA256(t, path), "the refused database was modified")
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		_, statErr := os.Stat(path + suffix)
		require.True(t, os.IsNotExist(statErr), "unexpected %s file", suffix)
	}
}

// Migrate refuses too, on a connection opened with the production DSN. That connection may
// switch a DELETE-mode file to WAL, so this checks schema and data, not the file bytes.
func TestMigrateRefusesDatabaseOlderThanMinimumWithoutChangingSchemaOrData(t *testing.T) {
	path := openFixture(t, "scrutiny-v1.69.0.db")
	before := sqliteContents(t, path)

	db, err := openSQLiteDatabase(sqliteProductionConfig(t, path), logrus.New())
	require.NoError(t, err)
	repo := &scrutinyRepository{gormClient: db, logger: logrus.New()}
	err = repo.Migrate(context.Background())
	require.ErrorIs(t, err, ErrDatabaseTooOld)
	require.Contains(t, err.Error(), "No schema or data was changed")
	require.NoError(t, repo.Close())

	require.Equal(t, before, sqliteContents(t, path))
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
		{name: "upstream history with an unknown upstream migration", setup: []string{
			"CREATE TABLE migrations (id TEXT PRIMARY KEY)",
			"INSERT INTO migrations VALUES ('m20250221084400'), ('m20260216155600'), ('m20260725120000')",
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
