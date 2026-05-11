package filesystem

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

type mountEntry struct {
	Source     string
	MountPoint string
	FSType     string
}

type statfsResult struct {
	totalBytes     int64
	availableBytes int64
}

var excludedFSTypes = map[string]struct{}{
	"autofs":      {},
	"binfmt_misc": {},
	"bpf":         {},
	"cgroup":      {},
	"cgroup2":     {},
	"configfs":    {},
	"debugfs":     {},
	"devpts":      {},
	"devtmpfs":    {},
	"fusectl":     {},
	"hugetlbfs":   {},
	"mqueue":      {},
	"nsfs":        {},
	"overlay":     {},
	"proc":        {},
	"pstore":      {},
	"securityfs":  {},
	"sysfs":       {},
	"tmpfs":       {},
	"tracefs":     {},
	"zfs":         {},
}

var excludedMountPoints = map[string]struct{}{
	"/etc/hostname":          {},
	"/etc/hosts":             {},
	"/etc/resolv.conf":       {},
	"/opt/scrutiny/config":   {},
	"/opt/scrutiny/influxdb": {},
}

// CollectLinuxSnapshots collects filesystem snapshots from /proc/mounts.
func CollectLinuxSnapshots(hostID string, now time.Time) ([]models.FilesystemCapacity, models.FilesystemHostStatus, error) {
	file, err := os.Open("/proc/mounts")
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

	return collectSnapshots(file, hostID, now, statfsForPath)
}

func collectSnapshots(reader io.Reader, hostID string, now time.Time, statfsFn func(string) (statfsResult, error)) ([]models.FilesystemCapacity, models.FilesystemHostStatus, error) {
	mounts, err := parseMounts(reader)
	if err != nil {
		return nil, models.FilesystemHostStatus{}, err
	}

	snapshots := make([]models.FilesystemCapacity, 0)
	eligibleCount := 0
	failedEligibleCount := 0

	for _, mount := range mounts {
		if isExcludedFilesystem(mount.FSType) || isExcludedMountPoint(mount.MountPoint) {
			continue
		}

		eligibleCount++
		stats, err := statfsFn(mount.MountPoint)
		if err != nil {
			failedEligibleCount++
			continue
		}

		usedBytes := stats.totalBytes - stats.availableBytes
		if usedBytes < 0 {
			usedBytes = 0
		}

		usedPercent := 0.0
		if stats.totalBytes > 0 {
			usedPercent = (float64(usedBytes) / float64(stats.totalBytes)) * 100
		}

		snapshots = append(snapshots, models.FilesystemCapacity{
			HostID:         hostID,
			MountPoint:     mount.MountPoint,
			SourceDevice:   mount.Source,
			FilesystemType: mount.FSType,
			TotalBytes:     stats.totalBytes,
			UsedBytes:      usedBytes,
			AvailableBytes: stats.availableBytes,
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

func parseMounts(reader io.Reader) ([]mountEntry, error) {
	mounts := make([]mountEntry, 0)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		mounts = append(mounts, mountEntry{
			Source:     fields[0],
			MountPoint: fields[1],
			FSType:     fields[2],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}

func isExcludedFilesystem(fsType string) bool {
	_, excluded := excludedFSTypes[fsType]
	return excluded
}

func isExcludedMountPoint(mountPoint string) bool {
	_, excluded := excludedMountPoints[mountPoint]
	return excluded
}

func statfsForPath(path string) (statfsResult, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return statfsResult{}, err
	}

	bsize := int64(stat.Bsize)
	if bsize <= 0 {
		return statfsResult{}, fmt.Errorf("unexpected block size %d", stat.Bsize)
	}

	totalBytes := int64(stat.Blocks) * bsize
	availableBytes := int64(stat.Bavail) * bsize

	return statfsResult{
		totalBytes:     totalBytes,
		availableBytes: availableBytes,
	}, nil
}
