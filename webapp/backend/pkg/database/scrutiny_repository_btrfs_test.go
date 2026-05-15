package database

import (
	"context"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createBtrfsTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.BtrfsFilesystem{}, &models.BtrfsDevice{}))

	return &scrutinyRepository{gormClient: db}
}

func TestRegisterBtrfsFilesystemReplacesDevices(t *testing.T) {
	repo := createBtrfsTestRepository(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	filesystem := models.BtrfsFilesystem{
		UUID:        "11111111-2222-3333-4444-555555555555",
		HostID:      "atlas",
		MountPoint:  "/",
		Status:      models.BtrfsFilesystemStatusOnline,
		DeviceCount: 1,
		UpdatedAt:   now,
		Devices: []models.BtrfsDevice{
			{DeviceID: 1, Path: "/dev/sda1", Size: 100},
		},
	}
	require.NoError(t, repo.RegisterBtrfsFilesystem(ctx, &filesystem))

	filesystem.DeviceCount = 2
	filesystem.Devices = []models.BtrfsDevice{
		{DeviceID: 1, Path: "/dev/sda1", Size: 100},
		{DeviceID: 2, Path: "/dev/sdb1", Size: 200},
	}
	require.NoError(t, repo.RegisterBtrfsFilesystem(ctx, &filesystem))

	loaded, err := repo.GetBtrfsFilesystemDetails(ctx, filesystem.UUID)
	require.NoError(t, err)
	require.Len(t, loaded.Devices, 2)
	require.Equal(t, 2, loaded.DeviceCount)
}

func TestBtrfsFilesystemActions(t *testing.T) {
	repo := createBtrfsTestRepository(t)
	ctx := context.Background()

	filesystem := models.BtrfsFilesystem{
		UUID:   "11111111-2222-3333-4444-555555555555",
		HostID: "atlas",
		Status: models.BtrfsFilesystemStatusOnline,
	}
	require.NoError(t, repo.RegisterBtrfsFilesystem(ctx, &filesystem))
	require.NoError(t, repo.UpdateBtrfsFilesystemArchived(ctx, filesystem.UUID, true))
	require.NoError(t, repo.UpdateBtrfsFilesystemMuted(ctx, filesystem.UUID, true))
	require.NoError(t, repo.UpdateBtrfsFilesystemLabel(ctx, filesystem.UUID, "tank"))

	loaded, err := repo.GetBtrfsFilesystemDetails(ctx, filesystem.UUID)
	require.NoError(t, err)
	require.True(t, loaded.Archived)
	require.True(t, loaded.Muted)
	require.Equal(t, "tank", loaded.Label)
}
