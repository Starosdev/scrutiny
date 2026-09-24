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

// The settings UI adds URLs with heartbeat off by default; the stored value must match (#888).
func TestSaveNotifyUrlKeepsHeartbeatSetting(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "notify.db")), &gorm.Config{})
	require.NoError(t, err)
	repo := &scrutinyRepository{gormClient: db, logger: logrus.New()}
	t.Cleanup(func() { _ = repo.Close() })
	// Use the real migrated schema, whose heartbeat_enabled column defaults to true.
	require.NoError(t, repo.Migrate(ctx))

	for _, enabled := range []bool{false, true} {
		url := &models.NotifyUrl{URL: "ntfy://topic", HeartbeatEnabled: enabled}
		require.NoError(t, repo.SaveNotifyUrl(ctx, url))
		require.Equal(t, enabled, url.HeartbeatEnabled)

		var stored models.NotifyUrl
		require.NoError(t, db.First(&stored, url.ID).Error)
		require.Equal(t, enabled, stored.HeartbeatEnabled)
	}
}
