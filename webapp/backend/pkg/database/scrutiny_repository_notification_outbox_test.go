package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260925000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newOutboxTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "outbox.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, m20260925000000.Migrate(db))
	repo := &scrutinyRepository{gormClient: db}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestNotificationOutbox(t *testing.T) {
	ctx := context.Background()
	repo := newOutboxTestRepository(t)

	for i, created := range []int64{100, 200, 300} {
		row := &models.NotificationOutbox{CreatedAtUnixMs: created, State: models.NotificationPending, Body: "{}"}
		require.NoError(t, repo.EnqueueNotification(ctx, row))
		require.Equal(t, uint(i+1), row.ID)
	}

	rows, err := repo.ListNotifications(ctx, models.NotificationPending, 2)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, []uint{1, 2}, []uint{rows[0].ID, rows[1].ID}, "oldest first, limited")

	ok, err := repo.TransitionNotification(ctx, 1, models.NotificationPending, models.NotificationClaimed)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.TransitionNotification(ctx, 1, models.NotificationPending, models.NotificationClaimed)
	require.NoError(t, err)
	require.False(t, ok, "a row can be claimed only once")

	removed, err := repo.DeleteStaleNotifications(ctx, []string{models.NotificationPending, models.NotificationClaimed}, 250)
	require.NoError(t, err)
	require.Equal(t, int64(2), removed, "rows 1 (claimed) and 2 (pending) are older than the cutoff")

	require.NoError(t, repo.DeleteNotifications(ctx, []uint{3}))
	require.NoError(t, repo.DeleteNotifications(ctx, nil))
	rows, err = repo.ListNotifications(ctx, models.NotificationPending, 10)
	require.NoError(t, err)
	require.Empty(t, rows)
}
