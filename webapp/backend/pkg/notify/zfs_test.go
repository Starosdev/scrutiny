package notify

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestZFSFailureTypes(t *testing.T) {
	t1 := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)
	online := func() *models.ZFSPool {
		return &models.ZFSPool{Status: models.ZFSPoolStatusOnline, ScrubState: models.ZFSScrubStateFinished, ScrubEndTime: &t1}
	}
	withStatus := func(status models.ZFSPoolStatus) *models.ZFSPool {
		pool := online()
		pool.Status = status
		return pool
	}
	withErrors := func(checksum int64) *models.ZFSPool {
		pool := online()
		pool.TotalChecksumErrors = checksum
		return pool
	}
	withScrubErrors := func(end time.Time) *models.ZFSPool {
		pool := online()
		pool.ScrubErrorsCount = 2
		pool.ScrubEndTime = &end
		return pool
	}

	for _, tc := range []struct {
		name     string
		previous *models.ZFSPool
		current  *models.ZFSPool
		repeat   bool
		want     []string
	}{
		{name: "healthy first upload", current: online()},
		{name: "healthy stays healthy", previous: online(), current: online()},
		{name: "becomes degraded", previous: online(), current: withStatus(models.ZFSPoolStatusDegraded), want: []string{NotifyFailureTypeZFSPoolDegraded}},
		{name: "faulted first upload", current: withStatus(models.ZFSPoolStatusFaulted), want: []string{NotifyFailureTypeZFSPoolDegraded}},
		{name: "degraded worsens to faulted", previous: withStatus(models.ZFSPoolStatusDegraded), current: withStatus(models.ZFSPoolStatusFaulted), want: []string{NotifyFailureTypeZFSPoolDegraded}},
		{name: "still degraded without repeat", previous: withStatus(models.ZFSPoolStatusDegraded), current: withStatus(models.ZFSPoolStatusDegraded)},
		{name: "still degraded with repeat", previous: withStatus(models.ZFSPoolStatusDegraded), current: withStatus(models.ZFSPoolStatusDegraded), repeat: true, want: []string{NotifyFailureTypeZFSPoolDegraded}},
		{name: "recovers to online", previous: withStatus(models.ZFSPoolStatusDegraded), current: online()},
		{name: "error total grows", previous: withErrors(1), current: withErrors(2), want: []string{NotifyFailureTypeZFSDeviceErrors}},
		{name: "error total unchanged", previous: withErrors(2), current: withErrors(2)},
		{name: "error total unchanged with repeat", previous: withErrors(2), current: withErrors(2), repeat: true, want: []string{NotifyFailureTypeZFSDeviceErrors}},
		{name: "errors on first upload", current: withErrors(1), want: []string{NotifyFailureTypeZFSDeviceErrors}},
		{name: "scrub finishes with errors", previous: online(), current: withScrubErrors(t2), want: []string{NotifyFailureTypeZFSScrubErrors}},
		{name: "same scrub reported again", previous: withScrubErrors(t2), current: withScrubErrors(t2)},
		{name: "same scrub reported again with repeat", previous: withScrubErrors(t2), current: withScrubErrors(t2), repeat: true, want: []string{NotifyFailureTypeZFSScrubErrors}},
		{name: "newer scrub with same error count", previous: withScrubErrors(t1), current: withScrubErrors(t2), want: []string{NotifyFailureTypeZFSScrubErrors}},
		{name: "scanning with stale errors", previous: online(), current: func() *models.ZFSPool {
			pool := withScrubErrors(t2)
			pool.ScrubState = models.ZFSScrubStateScanning
			return pool
		}()},
		{name: "degraded and errors", previous: online(), current: func() *models.ZFSPool {
			pool := withStatus(models.ZFSPoolStatusDegraded)
			pool.TotalReadErrors = 4
			return pool
		}(), want: []string{NotifyFailureTypeZFSPoolDegraded, NotifyFailureTypeZFSDeviceErrors}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ZFSFailureTypes(tc.previous, tc.current, tc.repeat))
		})
	}
}

func TestNewZFSNotify(t *testing.T) {
	cfg, err := config.Create()
	require.NoError(t, err)
	pool := &models.ZFSPool{
		GUID:                "1234567890",
		HostID:              "nas",
		Name:                "tank",
		Status:              models.ZFSPoolStatusDegraded,
		Health:              "DEGRADED",
		ScrubState:          models.ZFSScrubStateFinished,
		ScrubErrorsCount:    2,
		TotalChecksumErrors: 7,
	}

	n := NewZFSNotify(logrus.New(), cfg, pool, []string{NotifyFailureTypeZFSPoolDegraded, NotifyFailureTypeZFSScrubErrors})

	require.Equal(t, "ZFS", n.Payload.DeviceType)
	require.Equal(t, "1234567890", n.Payload.DeviceSerial)
	require.Equal(t, "nas", n.Payload.HostId)
	require.Equal(t, NotifyFailureTypeZFSPoolDegraded, n.Payload.FailureType)
	require.Equal(t, "Scrutiny ZFS pool problem (ZFSPoolDegraded) detected on pool: tank", n.Payload.Subject)
	require.Contains(t, n.Payload.Message, "Failure Type: ZFSPoolDegraded, ZFSScrubErrors")
	require.Contains(t, n.Payload.Message, "Status: DEGRADED")
	require.Contains(t, n.Payload.Message, "Error Totals: read=0 write=0 checksum=7")
	require.Contains(t, n.Payload.Message, "Scrub Errors: 2")
	require.Contains(t, n.Payload.HTMLMessage, "ZFS POOL PROBLEM")
	require.Contains(t, n.Payload.HTMLMessage, "tank")
}
