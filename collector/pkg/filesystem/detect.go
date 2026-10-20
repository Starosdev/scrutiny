package filesystem

import (
	"fmt"
	"io"
	"os"
	"time"

	mountsPkg "github.com/analogj/scrutiny/collector/pkg/mounts"
	"github.com/analogj/scrutiny/collector/pkg/statfs"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

// CollectLinuxSnapshots collects filesystem snapshots from /proc/mounts.
func CollectLinuxSnapshots(hostID string, now time.Time) ([]models.FilesystemCapacity, models.FilesystemHostStatus, error) {
	file, err := os.Open(mountsPkg.ProcMounts)
	if err != nil {
		return nil, models.FilesystemHostStatus{
			HostID:          hostID,
			Status:          models.FilesystemHostStatusUnavailable,
			Reason:          fmt.Sprintf("could not read /proc/mounts: %v", err),
			FilesystemCount: 0,
			UpdatedAt:       now,
		}, nil
	}
	defer file.Close()

	return collectSnapshots(file, hostID, now, statfs.Stat)
}

func collectSnapshots(reader io.Reader, hostID string, now time.Time, statfsFn func(string) (statfs.Result, error)) ([]models.FilesystemCapacity, models.FilesystemHostStatus, error) {
	mountEntries, err := mountsPkg.Parse(reader)
	if err != nil {
		return nil, models.FilesystemHostStatus{}, err
	}

	snapshots := make([]models.FilesystemCapacity, 0)
	eligibleCount := 0
	failedEligibleCount := 0

	for _, mount := range mountEntries {
		if mountsPkg.ExcludedCapacityFSType(mount.FSType) || mountsPkg.ExcludedMountPoint(mount.MountPoint) {
			continue
		}

		eligibleCount++
		stats, err := statfsFn(mount.MountPoint)
		if err != nil {
			failedEligibleCount++
			continue
		}

		usedBytes := stats.Total - stats.Available
		if usedBytes < 0 {
			usedBytes = 0
		}

		usedPercent := 0.0
		if stats.Total > 0 {
			usedPercent = (float64(usedBytes) / float64(stats.Total)) * 100
		}

		snapshots = append(snapshots, models.FilesystemCapacity{
			HostID:         hostID,
			MountPoint:     mount.MountPoint,
			SourceDevice:   mount.Source,
			FilesystemType: mount.FSType,
			TotalBytes:     stats.Total,
			UsedBytes:      usedBytes,
			AvailableBytes: stats.Available,
			UsedPercent:    usedPercent,
			UpdatedAt:      now,
		})
	}

	status := models.FilesystemHostStatus{
		HostID:          hostID,
		Status:          models.FilesystemHostStatusAvailable,
		FilesystemCount: len(snapshots),
		UpdatedAt:       now,
	}

	if eligibleCount > 0 && len(snapshots) == 0 && failedEligibleCount == eligibleCount {
		status.Status = models.FilesystemHostStatusUnavailable
		status.Reason = "collector could not inspect eligible host mounts"
	}

	return snapshots, status, nil
}
