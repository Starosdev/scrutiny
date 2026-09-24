package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newPostgresTestConfig returns a config pointing at a fresh, empty schema in the PostgreSQL server
// named by SCRUTINY_TEST_POSTGRES_DSN (a postgres:// URL). The schema is dropped after the test.
// Without the variable the test is skipped, unless SCRUTINY_REQUIRE_POSTGRES is set, which CI uses
// so a misconfigured job fails instead of passing with nothing tested.
func newPostgresTestConfig(t *testing.T) config.Interface {
	t.Helper()
	dsn := os.Getenv("SCRUTINY_TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("SCRUTINY_REQUIRE_POSTGRES") != "" {
			t.Fatal("SCRUTINY_REQUIRE_POSTGRES is set but SCRUTINY_TEST_POSTGRES_DSN is empty")
		}
		t.Skip("SCRUTINY_TEST_POSTGRES_DSN not set")
	}

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	schema := "scrutiny_test_" + hex.EncodeToString(suffix)

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	cfg, err := config.Create()
	require.NoError(t, err)
	cfg.Set("web.database.type", "postgres")
	cfg.Set("web.database.dsn", u.String())
	return cfg
}

func newPostgresTestRepository(t *testing.T, cfg config.Interface) *scrutinyRepository {
	t.Helper()
	db, err := openGormDatabase(cfg, logrus.New())
	require.NoError(t, err)
	require.Equal(t, dialectPostgres, db.Name())
	repo := &scrutinyRepository{appConfig: cfg, gormClient: db, logger: logrus.New()}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestPostgres_BaselineMatchesSQLiteHistory(t *testing.T) {
	ctx := context.Background()
	repo := newPostgresTestRepository(t, newPostgresTestConfig(t))
	require.NoError(t, repo.Migrate(ctx))
	require.NoError(t, repo.Migrate(ctx), "a second run is a no-op")

	history := migratedSQLite(t)
	require.Equal(t, snapshotSchema(t, history), snapshotSchema(t, repo.gormClient))

	var historySettings, pgSettings []models.SettingEntry
	require.NoError(t, history.Order("id").Find(&historySettings).Error)
	require.NoError(t, repo.gormClient.Order("setting_key_name").Find(&pgSettings).Error)
	want := settingValues(uniqueSettings(historySettings))
	got := settingValues(pgSettings)
	require.ElementsMatch(t, want, got)
}

func TestPostgres_ConcurrentMigrate(t *testing.T) {
	cfg := newPostgresTestConfig(t)
	repos := []*scrutinyRepository{newPostgresTestRepository(t, cfg), newPostgresTestRepository(t, cfg)}

	var wg sync.WaitGroup
	errs := make([]error, len(repos))
	for i, repo := range repos {
		wg.Add(1)
		go func(i int, repo *scrutinyRepository) {
			defer wg.Done()
			errs[i] = repo.Migrate(context.Background())
		}(i, repo)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	var count int64
	require.NoError(t, repos[0].gormClient.Model(&models.SettingEntry{}).Where("setting_key_name = ?", "line_stroke").Count(&count).Error)
	require.Equal(t, int64(1), count, "the baseline ran once")
}

// TestPostgres_RepositorySmoke runs the statements most likely to differ between dialects:
// upserts, conditional updates, and settings round trips.
func TestPostgres_RepositorySmoke(t *testing.T) {
	ctx := context.Background()
	repo := newPostgresTestRepository(t, newPostgresTestConfig(t))
	require.NoError(t, repo.Migrate(ctx))

	settings, err := repo.LoadSettings(ctx)
	require.NoError(t, err)
	require.NotNil(t, settings)

	ok, err := repo.TryAcquireLease(ctx, "scheduler", "a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.TryAcquireLease(ctx, "scheduler", "b", time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = repo.TryAcquireLease(ctx, "scheduler", "a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	row := &models.NotificationOutbox{CreatedAtUnixMs: 1, State: models.NotificationPending, Body: "{}"}
	require.NoError(t, repo.EnqueueNotification(ctx, row))
	ok, err = repo.TransitionNotification(ctx, row.ID, models.NotificationPending, models.NotificationClaimed)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.TransitionNotification(ctx, row.ID, models.NotificationPending, models.NotificationClaimed)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = repo.CompareAndSetSettingValue(ctx, "metrics.report_last_daily_run", "", "2026-01-01T00:00:00Z")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.CompareAndSetSettingValue(ctx, "metrics.report_last_daily_run", "", "2026-01-02T00:00:00Z")
	require.NoError(t, err)
	require.False(t, ok)

	device := models.Device{DeviceID: "d1", WWN: "0x5000", DeviceName: "sda", SerialNumber: "S1", ModelName: "M"}
	require.NoError(t, repo.RegisterDevice(ctx, device))
	require.NoError(t, repo.RegisterDevice(ctx, device), "registering again updates in place")
	devices, err := repo.GetDevices(ctx)
	require.NoError(t, err)
	require.Len(t, devices, 1)
}
