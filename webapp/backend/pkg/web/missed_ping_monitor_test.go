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

func createTestAppEngine(t *testing.T) (*AppEngine, *gomock.Controller) {
	return createTestAppEngineWithNotifyUrls(t, []string{})
}

func createTestAppEngineWithNotifyUrls(t *testing.T, notifyUrls []string) (*AppEngine, *gomock.Controller) {
	mockCtrl := gomock.NewController(t)
	fakeConfig := mock_config.NewMockInterface(mockCtrl)

	// Common config expectations
	fakeConfig.EXPECT().GetStringSlice("notify.urls").Return(notifyUrls).AnyTimes()
	fakeConfig.EXPECT().GetString("notify.urls").Return("").AnyTimes()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	ae := &AppEngine{
		Config: fakeConfig,
		Logger: logrus.NewEntry(logger),
	}

	return ae, mockCtrl
}

func TestMissedPingMonitor_NewMissedPingMonitor(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	require.NotNil(t, monitor)
	require.NotNil(t, monitor.notifiedDevices)
	require.NotNil(t, monitor.stopCh)
	require.NotNil(t, monitor.ctx)
	require.NotNil(t, monitor.cancel)
	require.Equal(t, 0, monitor.GetNotifiedDevicesCount())
}

func TestMissedPingMonitor_StartAndStop(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Cancel context before starting to prevent repository creation attempts
	// This simulates a shutdown scenario where the context is cancelled early
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

func TestMissedPingMonitor_ContextCancellation(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Verify context is not cancelled initially
	require.NoError(t, monitor.ctx.Err())

	// Cancel via Stop()
	monitor.cancel()

	// Context should now be cancelled
	require.Error(t, monitor.ctx.Err())
	require.Equal(t, context.Canceled, monitor.ctx.Err())
}

func TestMissedPingMonitor_CleanupStaleNotifications(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Manually add some notified devices
	monitor.notifiedDevices["device1"] = time.Now()
	monitor.notifiedDevices["device2"] = time.Now()
	monitor.notifiedDevices["device3"] = time.Now()

	require.Equal(t, 3, monitor.GetNotifiedDevicesCount())

	// Current devices only include device1 and device2
	currentDevices := map[string]bool{
		"device1": true,
		"device2": true,
	}

	// Cleanup should remove device3
	monitor.cleanupStaleNotifications(currentDevices)

	require.Equal(t, 2, monitor.GetNotifiedDevicesCount())
	require.True(t, monitor.IsDeviceNotified("device1"))
	require.True(t, monitor.IsDeviceNotified("device2"))
	require.False(t, monitor.IsDeviceNotified("device3"))
}

func TestMissedPingMonitor_CleanupStaleNotifications_AllRemoved(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Add notified devices
	monitor.notifiedDevices["device1"] = time.Now()
	monitor.notifiedDevices["device2"] = time.Now()

	// Empty current devices - all should be cleaned up
	currentDevices := map[string]bool{}

	monitor.cleanupStaleNotifications(currentDevices)

	require.Equal(t, 0, monitor.GetNotifiedDevicesCount())
}

func TestMissedPingMonitor_ClearNotificationState(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Add a notified device
	monitor.notifiedDevices["device1"] = time.Now()
	require.True(t, monitor.IsDeviceNotified("device1"))

	// Clear it
	monitor.clearNotificationState("device1")
	require.False(t, monitor.IsDeviceNotified("device1"))

	// Clearing non-existent device should not panic
	monitor.clearNotificationState("non-existent")
}

func TestMissedPingMonitor_CheckDevice_ReturnsMissedWhenNoEndpoints(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	device := models.Device{
		WWN:        "test-wwn",
		DeviceName: "/dev/sda",
	}
	lastSeen := time.Now().Add(-2 * time.Hour)
	timeoutMinutes := 60

	// checkDevice should return a digest device when the device has missed pings
	data := &checkMissedPingsData{
		timeoutMinutes: timeoutMinutes,
		timeout:        time.Duration(timeoutMinutes) * time.Minute,
		lastSeenTimes: map[string]time.Time{
			"test-wwn": lastSeen,
		},
	}
	result := monitor.checkDevice(device, data, time.Now())
	require.NotNil(t, result)
	require.Equal(t, "test-wwn", result.WWN)
}

func TestMissedPingMonitor_CheckDevice_Deduplication(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	device := models.Device{
		WWN:        "test-wwn",
		DeviceName: "/dev/sda",
	}
	lastSeen := time.Now().Add(-2 * time.Hour)
	timeoutMinutes := 60

	// Simulate a successful notification by directly adding to notifiedDevices
	firstNotifyTime := time.Now()
	monitor.mu.Lock()
	monitor.notifiedDevices["test-wwn"] = firstNotifyTime
	monitor.mu.Unlock()

	require.True(t, monitor.IsDeviceNotified("test-wwn"))

	// Second call within timeout period should be skipped (deduplication)
	time.Sleep(10 * time.Millisecond)
	data := &checkMissedPingsData{
		timeoutMinutes: timeoutMinutes,
		timeout:        time.Duration(timeoutMinutes) * time.Minute,
		lastSeenTimes: map[string]time.Time{
			"test-wwn": lastSeen,
		},
	}
	result := monitor.checkDevice(device, data, time.Now())

	// Should return nil (skipped due to dedup) and notification time should not have changed
	require.Nil(t, result)
	monitor.mu.RLock()
	storedTime := monitor.notifiedDevices["test-wwn"]
	monitor.mu.RUnlock()
	require.Equal(t, firstNotifyTime, storedTime)
}

func TestMissedPingMonitor_CheckDevice_SkipsArchived(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	device := models.Device{
		WWN:        "archived-device",
		DeviceName: "/dev/sda",
		Archived:   true,
	}

	data := &checkMissedPingsData{
		timeoutMinutes: 60,
		timeout:        60 * time.Minute,
		lastSeenTimes: map[string]time.Time{
			"archived-device": time.Now().Add(-2 * time.Hour), // Would trigger notification if not archived
		},
	}

	result := monitor.checkDevice(device, data, time.Now())

	// Should return nil since device is archived
	require.Nil(t, result)
}

func TestMissedPingMonitor_CheckDevice_SkipsMuted(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	device := models.Device{
		WWN:        "muted-device",
		DeviceName: "/dev/sda",
		Muted:      true,
	}

	data := &checkMissedPingsData{
		timeoutMinutes: 60,
		timeout:        60 * time.Minute,
		lastSeenTimes: map[string]time.Time{
			"muted-device": time.Now().Add(-2 * time.Hour),
		},
	}

	result := monitor.checkDevice(device, data, time.Now())

	// Should return nil since device is muted
	require.Nil(t, result)
}

func TestMissedPingMonitor_CheckDevice_SkipsNewlyRegistered(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	device := models.Device{
		WWN:        "new-device",
		DeviceName: "/dev/sda",
	}

	data := &checkMissedPingsData{
		timeoutMinutes: 60,
		timeout:        60 * time.Minute,
		lastSeenTimes:  map[string]time.Time{},
	}

	result := monitor.checkDevice(device, data, time.Now())

	// Should return nil since device has no last seen time
	require.Nil(t, result)
}

func TestMissedPingMonitor_CheckDevice_ClearsHealthyDevice(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Pre-populate with a notified device
	monitor.notifiedDevices["healthy-device"] = time.Now().Add(-1 * time.Hour)
	require.True(t, monitor.IsDeviceNotified("healthy-device"))

	device := models.Device{
		WWN:        "healthy-device",
		DeviceName: "/dev/sda",
	}

	data := &checkMissedPingsData{
		timeoutMinutes: 60,
		timeout:        60 * time.Minute,
		lastSeenTimes: map[string]time.Time{
			"healthy-device": time.Now().Add(-5 * time.Minute),
		},
	}

	result := monitor.checkDevice(device, data, time.Now())

	// Should return nil (device is healthy) and clear notification state
	require.Nil(t, result)
	require.False(t, monitor.IsDeviceNotified("healthy-device"))
}

func TestMissedPingMonitor_CheckDevice_ReturnsMissedDevice(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	device := models.Device{
		WWN:          "stale-device",
		DeviceName:   "/dev/sda",
		SerialNumber: "ABC123",
		DeviceStatus: pkg.DeviceStatusPassed,
	}

	data := &checkMissedPingsData{
		timeoutMinutes: 60,
		timeout:        60 * time.Minute,
		lastSeenTimes: map[string]time.Time{
			"stale-device": time.Now().Add(-2 * time.Hour),
		},
	}

	result := monitor.checkDevice(device, data, time.Now())

	// Should return a digest device entry (notification not sent yet, just collected)
	require.NotNil(t, result)
	require.Equal(t, "stale-device", result.WWN)
	require.Equal(t, "/dev/sda", result.DeviceName)
	require.Equal(t, "ABC123", result.SerialNumber)
}

func TestMissedPingMonitor_GetCheckInterval_Default(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Cancel context to simulate startup without database
	monitor.cancel()

	interval := monitor.getCheckInterval()

	require.Equal(t, time.Duration(DefaultMissedPingCheckIntervalMins)*time.Minute, interval)
}

func TestMissedPingMonitor_CheckMissedPings_SkipsWhenContextCancelled(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// Cancel context
	monitor.cancel()

	// This should return early without doing anything
	monitor.checkMissedPings()

	// No devices should be notified
	require.Equal(t, 0, monitor.GetNotifiedDevicesCount())
}

func TestMissedPingMonitor_ResetRepo(t *testing.T) {
	t.Parallel()

	ae, mockCtrl := createTestAppEngine(t)
	defer mockCtrl.Finish()

	monitor := NewMissedPingMonitor(ae)

	// resetRepo should not panic even when repo is nil
	monitor.resetRepo()

	// Still nil
	require.Nil(t, monitor.deviceRepo)
}
