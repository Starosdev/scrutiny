package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260924000000"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newLeaseTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "leases.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, m20260924000000.Migrate(db))
	repo := &scrutinyRepository{gormClient: db}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestTryAcquireLease(t *testing.T) {
	ctx := context.Background()
	repo := newLeaseTestRepository(t)

	ok, err := repo.TryAcquireLease(ctx, "scheduler", "a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "a free lease is acquired")

	ok, err = repo.TryAcquireLease(ctx, "scheduler", "b", time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "a live lease held by another holder is refused")

	ok, err = repo.TryAcquireLease(ctx, "scheduler", "a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "the holder renews its own lease")

	ok, err = repo.TryAcquireLease(ctx, "other", "b", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "leases are independent by name")
}

func TestTryAcquireLease_TakesOverExpiredLease(t *testing.T) {
	ctx := context.Background()
	repo := newLeaseTestRepository(t)

	ok, err := repo.TryAcquireLease(ctx, "scheduler", "a", -time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = repo.TryAcquireLease(ctx, "scheduler", "b", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "an expired lease passes to the next holder")

	ok, err = repo.TryAcquireLease(ctx, "scheduler", "a", time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "the previous holder cannot reclaim a live lease")
}

func TestReleaseLease(t *testing.T) {
	ctx := context.Background()
	repo := newLeaseTestRepository(t)

	ok, err := repo.TryAcquireLease(ctx, "scheduler", "a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, repo.ReleaseLease(ctx, "scheduler", "b"))
	ok, err = repo.TryAcquireLease(ctx, "scheduler", "b", time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "a release by a non-holder changes nothing")

	require.NoError(t, repo.ReleaseLease(ctx, "scheduler", "a"))
	ok, err = repo.TryAcquireLease(ctx, "scheduler", "b", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "a released lease is free at once")
}
