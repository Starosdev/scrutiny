package filesystem

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/stretchr/testify/require"
)

const sampleMounts = `sysfs /sys sysfs rw 0 0
proc /proc proc rw 0 0
/dev/sda1 / ext4 rw 0 0
/dev/sdb1 /data xfs rw 0 0
overlay /var/lib/docker/overlay2/abc overlay rw 0 0
tank/home /tank/home zfs rw 0 0
`

func TestCollectSnapshotsFiltersPseudoFilesystemsAndZFS(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	reader := strings.NewReader(sampleMounts)

	statfsFn := func(path string) (statfsResult, error) {
		switch path {
		case "/":
			return statfsResult{totalBytes: 1000, availableBytes: 250}, nil
		case "/data":
			return statfsResult{totalBytes: 2000, availableBytes: 500}, nil
		default:
			return statfsResult{}, errors.New("unexpected path")
		}
	}

	snapshots, status, err := collectSnapshots(reader, "host-a", now, statfsFn)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	require.Equal(t, models.FilesystemHostStatusAvailable, status.Status)
	require.Equal(t, 2, status.FilesystemCount)
	require.Equal(t, "/", snapshots[0].MountPoint)
	require.Equal(t, "/data", snapshots[1].MountPoint)
	require.InDelta(t, 75.0, snapshots[0].UsedPercent, 0.001)
}

func TestCollectSnapshotsMarksUnavailableWhenEligibleMountsCannotBeRead(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	reader := strings.NewReader("/dev/sda1 / ext4 rw 0 0\n")

	snapshots, status, err := collectSnapshots(reader, "host-a", now, func(path string) (statfsResult, error) {
		return statfsResult{}, errors.New("permission denied")
	})
	require.NoError(t, err)
	require.Len(t, snapshots, 0)
	require.Equal(t, models.FilesystemHostStatusUnavailable, status.Status)
	require.Equal(t, "collector could not inspect eligible host mounts", status.Reason)
}

func TestCollectSnapshotsAllowsEmptyEligibleSet(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	reader := strings.NewReader("tmpfs /run tmpfs rw 0 0\noverlay /overlay overlay rw 0 0\n")

	snapshots, status, err := collectSnapshots(reader, "host-a", now, func(path string) (statfsResult, error) {
		return statfsResult{}, nil
	})
	require.NoError(t, err)
	require.Len(t, snapshots, 0)
	require.Equal(t, models.FilesystemHostStatusAvailable, status.Status)
	require.Equal(t, 0, status.FilesystemCount)
}
