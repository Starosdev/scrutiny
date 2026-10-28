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
	require.NoError(t, db.AutoMigrate(&models.FilesystemCapacity{}, &models.FilesystemHostStatus{}, &models.FilesystemDevice{}))

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

// filesystemUpload builds a one-host upload from mount point to backing disks.
func filesystemUpload(hostID string, now time.Time, mounts map[string][]string) models.FilesystemSummaryUpload {
	filesystems := make([]models.FilesystemCapacity, 0, len(mounts))
	for mountPoint, backingDevices := range mounts {
		filesystems = append(filesystems, models.FilesystemCapacity{
			HostID:         hostID,
			MountPoint:     mountPoint,
			FilesystemType: "ext4",
			TotalBytes:     100,
			UsedBytes:      60,
			AvailableBytes: 40,
			UsedPercent:    60,
			UpdatedAt:      now,
			BackingDevices: backingDevices,
		})
	}
	return models.FilesystemSummaryUpload{
		Filesystems: filesystems,
		Hosts: []models.FilesystemHostStatus{
			{HostID: hostID, Status: models.FilesystemHostStatusAvailable, FilesystemCount: len(filesystems), UpdatedAt: now},
		},
	}
}

func TestFilesystemsByHostDeviceGroupsEveryBackingDisk(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	require.NoError(t, repo.SaveFilesystemSummary(ctx, filesystemUpload("atlas", now, map[string][]string{
		"/":       {"sda"},
		"/array":  {"sdb", "sdc"},
		"/remote": nil,
	})))

	byHostDevice, err := repo.filesystemsByHostDevice(ctx)
	require.NoError(t, err)

	// An md or LVM filesystem appears under every disk it spans, because each
	// of those drives is one a user could be looking at.
	require.Len(t, byHostDevice["atlas"]["sdb"], 1)
	require.Equal(t, "/array", byHostDevice["atlas"]["sdb"][0].MountPoint)
	require.Len(t, byHostDevice["atlas"]["sdc"], 1)
	require.Equal(t, "/", byHostDevice["atlas"]["sda"][0].MountPoint)

	// An unattributed filesystem is reported for the host and belongs to no
	// drive. Inventing a key for it would credit its free space to a disk.
	require.Len(t, byHostDevice["atlas"], 3)
	filesystems, _, err := repo.GetFilesystemSummary(ctx)
	require.NoError(t, err)
	require.Len(t, filesystems["atlas"], 3)
}

func TestGetFilesystemSummaryReportsBackingDevices(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	require.NoError(t, repo.SaveFilesystemSummary(ctx, filesystemUpload("atlas", now, map[string][]string{
		// A collector that names one disk twice must not break the primary key.
		"/array": {"sdb", "sdb", "sdc"},
	})))

	filesystems, _, err := repo.GetFilesystemSummary(ctx)
	require.NoError(t, err)
	require.Len(t, filesystems["atlas"], 1)
	require.Equal(t, []string{"sdb", "sdc"}, filesystems["atlas"][0].BackingDevices)
}

func TestSaveFilesystemSummaryDropsStaleDeviceLinks(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	require.NoError(t, repo.SaveFilesystemSummary(ctx, filesystemUpload("atlas", now, map[string][]string{"/data": {"sdb"}})))
	// The mount moved to another disk. The old attribution must not survive,
	// or sdb keeps reporting free space it no longer holds.
	require.NoError(t, repo.SaveFilesystemSummary(ctx, filesystemUpload("atlas", now.Add(time.Minute), map[string][]string{"/data": {"sdz"}})))

	byHostDevice, err := repo.filesystemsByHostDevice(ctx)
	require.NoError(t, err)
	require.NotContains(t, byHostDevice["atlas"], "sdb")
	require.Len(t, byHostDevice["atlas"]["sdz"], 1)
}
