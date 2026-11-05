package notify

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestBtrfsFailureTypes(t *testing.T) {
	t1 := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)
	online := func() *models.BtrfsFilesystem {
		return &models.BtrfsFilesystem{Status: models.BtrfsFilesystemStatusOnline, ScrubState: models.BtrfsScrubStateFinished, ScrubFinishedAt: &t1,
			Devices: []models.BtrfsDevice{{DeviceID: 1}, {DeviceID: 2}}}
	}
	degraded := func() *models.BtrfsFilesystem {
		fs := online()
		fs.Status = models.BtrfsFilesystemStatusDegraded
		fs.DeviceMissing = 1
		return fs
	}
	withDeviceErrors := func(write int64) *models.BtrfsFilesystem {
		fs := online()
		fs.Devices[1].WriteIOErrors = write
		return fs
	}
	withScrubErrors := func(finished time.Time) *models.BtrfsFilesystem {
		fs := online()
		fs.ScrubCsumErrors = 3
		fs.ScrubFinishedAt = &finished
		return fs
	}

	for _, tc := range []struct {
		name     string
		previous *models.BtrfsFilesystem
		current  *models.BtrfsFilesystem
		repeat   bool
		want     []string
	}{
		{name: "healthy first upload", current: online()},
		{name: "healthy stays healthy", previous: online(), current: online()},
		{name: "becomes degraded", previous: online(), current: degraded(), want: []string{NotifyFailureTypeBtrfsDegraded}},
		{name: "degraded first upload", current: degraded(), want: []string{NotifyFailureTypeBtrfsDegraded}},
		{name: "still degraded without repeat", previous: degraded(), current: degraded()},
		{name: "still degraded with repeat", previous: degraded(), current: degraded(), repeat: true, want: []string{NotifyFailureTypeBtrfsDegraded}},
		{name: "device counter grows", previous: withDeviceErrors(1), current: withDeviceErrors(2), want: []string{NotifyFailureTypeBtrfsDeviceErrors}},
		{name: "device counter unchanged", previous: withDeviceErrors(2), current: withDeviceErrors(2)},
		{name: "device counter unchanged with repeat", previous: withDeviceErrors(2), current: withDeviceErrors(2), repeat: true, want: []string{NotifyFailureTypeBtrfsDeviceErrors}},
		{name: "new device with errors", previous: online(), current: withDeviceErrors(1), want: []string{NotifyFailureTypeBtrfsDeviceErrors}},
		{name: "scrub finishes with errors", previous: online(), current: withScrubErrors(t2), want: []string{NotifyFailureTypeBtrfsScrubErrors}},
		{name: "same scrub reported again", previous: withScrubErrors(t2), current: withScrubErrors(t2)},
		{name: "same scrub reported again with repeat", previous: withScrubErrors(t2), current: withScrubErrors(t2), repeat: true, want: []string{NotifyFailureTypeBtrfsScrubErrors}},
		{name: "newer scrub with same error counts", previous: withScrubErrors(t1), current: withScrubErrors(t2), want: []string{NotifyFailureTypeBtrfsScrubErrors}},
		{name: "running scrub with stale errors", previous: online(), current: func() *models.BtrfsFilesystem {
			fs := withScrubErrors(t2)
			fs.ScrubState = models.BtrfsScrubStateRunning
			return fs
		}()},
		{name: "degraded and device errors", previous: online(), current: func() *models.BtrfsFilesystem {
			fs := degraded()
			fs.Devices[0].ReadIOErrors = 5
			return fs
		}(), want: []string{NotifyFailureTypeBtrfsDegraded, NotifyFailureTypeBtrfsDeviceErrors}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, BtrfsFailureTypes(tc.previous, tc.current, tc.repeat))
		})
	}
}

func TestNewBtrfsNotify(t *testing.T) {
	cfg, err := config.Create()
	require.NoError(t, err)
	finished := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	filesystem := &models.BtrfsFilesystem{
		UUID:            "11111111-2222-3333-4444-555555555555",
		HostID:          "nas",
		Label:           "tank",
		MountPoint:      "/mnt/tank",
		Status:          models.BtrfsFilesystemStatusDegraded,
		DeviceMissing:   1,
		ScrubState:      models.BtrfsScrubStateFinished,
		ScrubFinishedAt: &finished,
		ScrubCsumErrors: 3,
		Devices: []models.BtrfsDevice{
			{DeviceID: 1, Path: "/dev/sda", Missing: true},
			{DeviceID: 2, Path: "/dev/sdb", CorruptionErrors: 7},
		},
	}

	n := NewBtrfsNotify(logrus.New(), cfg, filesystem, []string{NotifyFailureTypeBtrfsDegraded, NotifyFailureTypeBtrfsScrubErrors})

	require.Equal(t, "Btrfs", n.Payload.DeviceType)
	require.Equal(t, filesystem.UUID, n.Payload.DeviceSerial)
	require.Equal(t, "nas", n.Payload.HostId)
	require.Equal(t, NotifyFailureTypeBtrfsDegraded, n.Payload.FailureType)
	require.Equal(t, "Scrutiny Btrfs problem (BtrfsDegraded) detected on filesystem: tank", n.Payload.Subject)
	require.Contains(t, n.Payload.Message, "Failure Type: BtrfsDegraded, BtrfsScrubErrors")
	require.Contains(t, n.Payload.Message, "Status: DEGRADED")
	require.Contains(t, n.Payload.Message, "Scrub Errors: read=0 csum=3 verify=0 super=0")
	require.Contains(t, n.Payload.Message, "Device 1 (/dev/sda): missing")
	require.Contains(t, n.Payload.Message, "Device 2 (/dev/sdb): write_io_errs=0 read_io_errs=0 flush_io_errs=0 corruption_errs=7 generation_errs=0")
	require.Contains(t, n.Payload.HTMLMessage, "BTRFS PROBLEM")
	require.Contains(t, n.Payload.HTMLMessage, "/dev/sdb")
}
