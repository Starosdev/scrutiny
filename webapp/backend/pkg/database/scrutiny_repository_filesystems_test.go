package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createFilesystemTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
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

func TestDedupeFilesystemsBySource(t *testing.T) {
	// A btrfs filesystem mounted twice by subvolume, and a bind mount of a
	// subdirectory: three mounts, one filesystem, one block device.
	filesystems := []models.FilesystemCapacity{
		{HostID: "atlas", MountPoint: "/", SourceDevice: "/dev/sda2", TotalBytes: 2000, AvailableBytes: 500},
		{HostID: "atlas", MountPoint: "/home", SourceDevice: "/dev/sda2", TotalBytes: 2000, AvailableBytes: 500},
		{HostID: "atlas", MountPoint: "/srv/export", SourceDevice: "/dev/sda2", TotalBytes: 2000, AvailableBytes: 500},
		{HostID: "atlas", MountPoint: "/data", SourceDevice: "/dev/sdb1", TotalBytes: 100, AvailableBytes: 40},
	}

	deduped := DedupeFilesystemsBySource(filesystems)
	require.Len(t, deduped, 2)
	require.Equal(t, "/", deduped[0].MountPoint)
	require.Equal(t, "/data", deduped[1].MountPoint)
}

func TestDedupeFilesystemsBySourceKeepsNonDeviceSources(t *testing.T) {
	// Several distinct mounts legitimately share a non-device source string.
	// Collapsing those would hide real capacity.
	filesystems := []models.FilesystemCapacity{
		{HostID: "atlas", MountPoint: "/mnt/a", SourceDevice: "none", TotalBytes: 100},
		{HostID: "atlas", MountPoint: "/mnt/b", SourceDevice: "none", TotalBytes: 200},
		{HostID: "atlas", MountPoint: "/mnt/c", SourceDevice: "", TotalBytes: 300},
	}

	require.Len(t, DedupeFilesystemsBySource(filesystems), 3)
}

func TestFilesystemsByHostDeviceCollapsesRepeatedMountsOfOneFilesystem(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	upload := filesystemUpload("atlas", now, map[string][]string{"/": {"sda"}, "/home": {"sda"}})
	for i := range upload.Filesystems {
		// Both mounts are subvolumes of one btrfs filesystem on one partition.
		upload.Filesystems[i].SourceDevice = "/dev/sda2"
		upload.Filesystems[i].FilesystemType = "btrfs"
	}
	require.NoError(t, repo.SaveFilesystemSummary(ctx, upload))

	byHostDevice, err := repo.filesystemsByHostDevice(ctx)
	require.NoError(t, err)
	// One filesystem on the drive, so the dashboard card can still report a
	// single free figure instead of falling back to a mount count.
	require.Len(t, byHostDevice["atlas"]["sda"], 1)

	// The per-host capacity panel still reports every mount.
	filesystems, _, err := repo.GetFilesystemSummary(ctx)
	require.NoError(t, err)
	require.Len(t, filesystems["atlas"], 2)
}

func TestAttachFilesystemsGivesEachDriveOnlyItsOwn(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	repo.logger = logrus.WithField("test", t.Name())
	ctx := context.Background()
	now := time.Unix(100, 0).UTC()

	require.NoError(t, repo.SaveFilesystemSummary(ctx, filesystemUpload("atlas", now, map[string][]string{
		"/":      {"sda"},
		"/array": {"sdb", "sdc"},
	})))
	require.NoError(t, repo.SaveFilesystemSummary(ctx, filesystemUpload("hermes", now, map[string][]string{
		"/": {"sda"},
	})))

	summaries := map[string]*models.DeviceSummary{
		"atlas-sda":  {Device: models.Device{DeviceID: "atlas-sda", HostId: "atlas", DeviceName: "sda"}},
		"atlas-sdb":  {Device: models.Device{DeviceID: "atlas-sdb", HostId: "atlas", DeviceName: "sdb"}},
		"atlas-sdd":  {Device: models.Device{DeviceID: "atlas-sdd", HostId: "atlas", DeviceName: "sdd"}},
		"hermes-sda": {Device: models.Device{DeviceID: "hermes-sda", HostId: "hermes", DeviceName: "sda"}},
		// A RAID passthrough device's name never names a mount source.
		"atlas-bus": {Device: models.Device{DeviceID: "atlas-bus", HostId: "atlas", DeviceName: "bus/0"}},
		"no-host":   {Device: models.Device{DeviceID: "no-host", DeviceName: "sda"}},
	}

	repo.attachFilesystems(ctx, summaries)

	require.Len(t, summaries["atlas-sda"].Filesystems, 1)
	require.Equal(t, "/", summaries["atlas-sda"].Filesystems[0].MountPoint)
	require.Len(t, summaries["atlas-sdb"].Filesystems, 1)
	require.Equal(t, "/array", summaries["atlas-sdb"].Filesystems[0].MountPoint)
	// Same device name, different host: the other host's filesystem must not
	// be attributed here.
	require.Len(t, summaries["hermes-sda"].Filesystems, 1)
	require.Equal(t, 100, int(summaries["hermes-sda"].Filesystems[0].TotalBytes))
	// A drive with nothing attributed stays empty rather than inheriting
	// anything from its host.
	require.Empty(t, summaries["atlas-sdd"].Filesystems)
	require.Empty(t, summaries["atlas-bus"].Filesystems)
	require.Empty(t, summaries["no-host"].Filesystems)
}

func TestAttachFilesystemsLeavesSummariesIntactWhenTheTableIsUnreadable(t *testing.T) {
	repo := createFilesystemTestRepository(t)
	repo.logger = logrus.WithField("test", t.Name())
	require.NoError(t, repo.gormClient.Migrator().DropTable(&models.FilesystemDevice{}))

	summaries := map[string]*models.DeviceSummary{
		"atlas-sda": {Device: models.Device{DeviceID: "atlas-sda", HostId: "atlas", DeviceName: "sda"}},
	}

	// The dashboard must still render without free-space figures rather than
	// failing because an optional collector's table is gone.
	repo.attachFilesystems(context.Background(), summaries)
	require.Empty(t, summaries["atlas-sda"].Filesystems)
}
