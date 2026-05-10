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

func createFilesystemTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.FilesystemCapacity{}, &models.FilesystemHostStatus{}))

	return &scrutinyRepository{
		gormClient: db,
	}
}

func TestSaveFilesystemSummaryReplacesSnapshotsPerHost(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	err := repo.SaveFilesystemSummary(ctx, models.FilesystemSummaryUpload{
		Filesystems: []models.FilesystemCapacity{
			{
				HostID:         "atlas",
				MountPoint:     "/",
				SourceDevice:   "/dev/sda1",
				FilesystemType: "ext4",
				TotalBytes:     100,
				UsedBytes:      60,
				AvailableBytes: 40,
				UsedPercent:    60,
				UpdatedAt:      now,
			},
		},
		Hosts: []models.FilesystemHostStatus{
			{
				HostID:          "atlas",
				Status:          models.FilesystemHostStatusAvailable,
				FilesystemCount: 1,
				UpdatedAt:       now,
			},
		},
	})
	require.NoError(t, err)

	err = repo.SaveFilesystemSummary(ctx, models.FilesystemSummaryUpload{
		Filesystems: []models.FilesystemCapacity{
			{
				HostID:         "atlas",
				MountPoint:     "/data",
				SourceDevice:   "/dev/sdb1",
				FilesystemType: "xfs",
				TotalBytes:     200,
				UsedBytes:      50,
				AvailableBytes: 150,
				UsedPercent:    25,
				UpdatedAt:      now.Add(time.Minute),
			},
		},
		Hosts: []models.FilesystemHostStatus{
			{
				HostID:          "atlas",
				Status:          models.FilesystemHostStatusAvailable,
				FilesystemCount: 1,
				UpdatedAt:       now.Add(time.Minute),
			},
		},
	})
	require.NoError(t, err)

	filesystems, hosts, err := repo.GetFilesystemSummary(ctx)
	require.NoError(t, err)
	require.Len(t, filesystems["atlas"], 1)
	require.Equal(t, "/data", filesystems["atlas"][0].MountPoint)
	require.Equal(t, models.FilesystemHostStatusAvailable, hosts["atlas"].Status)
}

func TestSaveFilesystemSummaryPersistsUnavailableHostStatus(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	err := repo.SaveFilesystemSummary(ctx, models.FilesystemSummaryUpload{
		Hosts: []models.FilesystemHostStatus{
			{
				HostID:          "atlas",
				Status:          models.FilesystemHostStatusUnavailable,
				Reason:          "collector could not inspect eligible host mounts",
				FilesystemCount: 0,
				UpdatedAt:       now,
			},
		},
	})
	require.NoError(t, err)

	filesystems, hosts, err := repo.GetFilesystemSummary(ctx)
	require.NoError(t, err)
	require.Len(t, filesystems["atlas"], 0)
	require.Equal(t, models.FilesystemHostStatusUnavailable, hosts["atlas"].Status)
	require.Equal(t, "collector could not inspect eligible host mounts", hosts["atlas"].Reason)
}
