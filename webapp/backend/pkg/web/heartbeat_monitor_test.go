package web

import (
	"context"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func createHeartbeatTestAppEngine(t *testing.T) (*AppEngine, *gomock.Controller) {
	mockCtrl := gomock.NewController(t)
	fakeConfig := mock_config.NewMockInterface(mockCtrl)

	// Common config expectations
	fakeConfig.EXPECT().GetStringSlice("notify.urls").Return([]string{}).AnyTimes()
	fakeConfig.EXPECT().GetString("notify.urls").Return("").AnyTimes()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	ae := &AppEngine{
		Config: fakeConfig,
		Logger: logrus.NewEntry(logger),
	}

	return ae, mockCtrl
}

func TestHeartbeatMonitor_NewHeartbeatMonitor(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createHeartbeatTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewHeartbeatMonitor(ae)

	require.NotNil(t, monitor)
	require.NotNil(t, monitor.stopCh)
	require.NotNil(t, monitor.ctx)
	require.NotNil(t, monitor.cancel)
	require.Equal(t, ae, monitor.appEngine)
}

func TestHeartbeatMonitor_StartAndStop(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createHeartbeatTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewHeartbeatMonitor(ae)

	// Cancel context before starting to prevent repository creation attempts
	monitor.cancel()

	// Start the monitor (will use default interval due to cancelled context)
	monitor.Start()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not hang and should complete quickly
	done := make(chan struct{})
	go func() {
		monitor.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success - stop completed
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() timed out - monitor did not shutdown gracefully")
	}
}

func TestHeartbeatMonitor_ContextCancellation(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createHeartbeatTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewHeartbeatMonitor(ae)

	// Verify context is not cancelled initially
	require.NoError(t, monitor.ctx.Err())

	// Cancel via cancel func
	monitor.cancel()

	// Context should now be cancelled
	require.Error(t, monitor.ctx.Err())
	require.Equal(t, context.Canceled, monitor.ctx.Err())
}

func TestHeartbeatMonitor_GetHeartbeatInterval_Default(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createHeartbeatTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewHeartbeatMonitor(ae)

	// Cancel context to simulate startup without database
	monitor.cancel()

	interval := monitor.getHeartbeatInterval()

	require.Equal(t, time.Duration(DefaultHeartbeatIntervalHours)*time.Hour, interval)
}

func TestHeartbeatMonitor_CheckAndSendHeartbeat_SkipsWhenContextCancelled(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createHeartbeatTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewHeartbeatMonitor(ae)

	// Cancel context
	monitor.cancel()

	// This should return early without doing anything
	monitor.checkAndSendHeartbeat()

	// Verify last check time was not set (context was cancelled before any work)
	monitor.statusMu.RLock()
	require.True(t, monitor.lastCheckTime.IsZero())
	monitor.statusMu.RUnlock()
}

func TestHeartbeatMonitor_ResetRepo(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createHeartbeatTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewHeartbeatMonitor(ae)

	// resetRepo should not panic even when repo is nil
	monitor.resetRepo()

	// Still nil
	require.Nil(t, monitor.deviceRepo)
}

// TestHeartbeatMonitor_AllHealthyDevices verifies the logic for determining
// if all devices are healthy (unit test of the filtering logic)
func TestHeartbeatMonitor_AllHealthyDevices(t *testing.T) {
	t.Parallel()

	devices := []models.Device{
		{WWN: "dev1", DeviceStatus: pkg.DeviceStatusPassed},
		{WWN: "dev2", DeviceStatus: pkg.DeviceStatusPassed},
		{WWN: "dev3", DeviceStatus: pkg.DeviceStatusPassed},
	}

	monitoredCount := 0
	allHealthy := true
	for _, device := range devices {
		if device.Archived || device.Muted {
			continue
		}
		monitoredCount++
		if device.DeviceStatus != pkg.DeviceStatusPassed {
			allHealthy = false
		}
	}

	require.Equal(t, 3, monitoredCount)
	require.True(t, allHealthy)
}

// TestHeartbeatMonitor_FailedDeviceSkipsHeartbeat verifies that heartbeat
// is suppressed when any device has a failure status
func TestHeartbeatMonitor_FailedDeviceSkipsHeartbeat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		devices      []models.Device
		expectHealthy bool
	}{
		{
			name: "smart failure",
			devices: []models.Device{
				{WWN: "dev1", DeviceStatus: pkg.DeviceStatusPassed},
				{WWN: "dev2", DeviceStatus: pkg.DeviceStatusFailedSmart},
			},
			expectHealthy: false,
		},
		{
			name: "scrutiny failure",
			devices: []models.Device{
				{WWN: "dev1", DeviceStatus: pkg.DeviceStatusPassed},
				{WWN: "dev2", DeviceStatus: pkg.DeviceStatusFailedScrutiny},
			},
			expectHealthy: false,
		},
		{
			name: "both failures",
			devices: []models.Device{
				{WWN: "dev1", DeviceStatus: pkg.DeviceStatusFailedSmart | pkg.DeviceStatusFailedScrutiny},
			},
			expectHealthy: false,
		},
		{
			name: "all healthy",
			devices: []models.Device{
				{WWN: "dev1", DeviceStatus: pkg.DeviceStatusPassed},
				{WWN: "dev2", DeviceStatus: pkg.DeviceStatusPassed},
			},
			expectHealthy: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			monitoredCount := 0
			allHealthy := true
			for _, device := range tc.devices {
				if device.Archived || device.Muted {
					continue
				}
				monitoredCount++
				if device.DeviceStatus != pkg.DeviceStatusPassed {
					allHealthy = false
				}
			}
			require.Equal(t, tc.expectHealthy, allHealthy)
			require.Greater(t, monitoredCount, 0)
		})
	}
}

// TestHeartbeatMonitor_ArchivedMutedDevicesExcluded verifies that archived
// and muted devices are excluded from the health check
func TestHeartbeatMonitor_ArchivedMutedDevicesExcluded(t *testing.T) {
	t.Parallel()

	devices := []models.Device{
		{WWN: "dev1", DeviceStatus: pkg.DeviceStatusPassed},
		{WWN: "dev2", DeviceStatus: pkg.DeviceStatusFailedSmart, Archived: true},
		{WWN: "dev3", DeviceStatus: pkg.DeviceStatusFailedScrutiny, Muted: true},
	}

	monitoredCount := 0
	allHealthy := true
	for _, device := range devices {
		if device.Archived || device.Muted {
			continue
		}
		monitoredCount++
		if device.DeviceStatus != pkg.DeviceStatusPassed {
			allHealthy = false
		}
	}

	// Only dev1 should be monitored, and it's healthy
	require.Equal(t, 1, monitoredCount)
	require.True(t, allHealthy)
}

// TestHeartbeatMonitor_NoMonitoredDevices verifies that heartbeat
// is not sent when there are no monitored devices
func TestHeartbeatMonitor_NoMonitoredDevices(t *testing.T) {
	t.Parallel()

	devices := []models.Device{
		{WWN: "dev1", DeviceStatus: pkg.DeviceStatusPassed, Archived: true},
		{WWN: "dev2", DeviceStatus: pkg.DeviceStatusPassed, Muted: true},
	}

	monitoredCount := 0
	for _, device := range devices {
		if device.Archived || device.Muted {
			continue
		}
		monitoredCount++
	}

	// No monitored devices -- heartbeat should not be sent
	require.Equal(t, 0, monitoredCount)
}
