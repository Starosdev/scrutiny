package web

import (
	"context"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/sirupsen/logrus"
)

const (
	// Default values for missed ping configuration
	DefaultMissedPingTimeoutMinutes    = 60
	DefaultMissedPingCheckIntervalMins = 5
)

// MissedPingMonitor monitors devices for missed collector pings and sends notifications
type MissedPingMonitor struct {
	appEngine *AppEngine
	logger    logrus.FieldLogger

	// Track which devices we've already notified about to avoid spam
	// Key: device WWN, Value: last notification time
	notifiedDevices map[string]time.Time
	mu              sync.RWMutex

	// Channel to signal shutdown
	stopCh chan struct{}
}

// NewMissedPingMonitor creates a new missed ping monitor
func NewMissedPingMonitor(ae *AppEngine) *MissedPingMonitor {
	return &MissedPingMonitor{
		appEngine:       ae,
		logger:          ae.Logger,
		notifiedDevices: make(map[string]time.Time),
		stopCh:          make(chan struct{}),
	}
}

// Start begins the background monitoring loop
func (m *MissedPingMonitor) Start() {
	go m.run()
}

// Stop signals the monitor to stop
func (m *MissedPingMonitor) Stop() {
	close(m.stopCh)
}

func (m *MissedPingMonitor) run() {
	// Load initial settings to get check interval
	checkInterval := m.getCheckInterval()
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	m.logger.Infof("Missed ping monitor started with check interval: %v", checkInterval)

	for {
		select {
		case <-m.stopCh:
			m.logger.Info("Missed ping monitor stopped")
			return
		case <-ticker.C:
			m.checkMissedPings()

			// Update ticker interval in case settings changed
			newInterval := m.getCheckInterval()
			if newInterval != checkInterval {
				ticker.Reset(newInterval)
				checkInterval = newInterval
				m.logger.Debugf("Missed ping check interval updated to: %v", checkInterval)
			}
		}
	}
}

func (m *MissedPingMonitor) getCheckInterval() time.Duration {
	// Try to load settings from database
	deviceRepo, err := database.NewScrutinyRepository(m.appEngine.Config, m.logger)
	if err != nil {
		m.logger.Warnf("Failed to create repository for settings: %v, using default interval", err)
		return time.Duration(DefaultMissedPingCheckIntervalMins) * time.Minute
	}
	defer deviceRepo.Close()

	settings, err := deviceRepo.LoadSettings(context.Background())
	if err != nil || settings == nil {
		return time.Duration(DefaultMissedPingCheckIntervalMins) * time.Minute
	}

	interval := settings.Metrics.MissedPingCheckIntervalMins
	if interval <= 0 {
		interval = DefaultMissedPingCheckIntervalMins
	}

	return time.Duration(interval) * time.Minute
}

func (m *MissedPingMonitor) checkMissedPings() {
	ctx := context.Background()

	// Create a new repository for this check
	deviceRepo, err := database.NewScrutinyRepository(m.appEngine.Config, m.logger)
	if err != nil {
		m.logger.Errorf("Failed to create repository for missed ping check: %v", err)
		return
	}
	defer deviceRepo.Close()

	// Load settings
	settings, err := deviceRepo.LoadSettings(ctx)
	if err != nil {
		m.logger.Errorf("Failed to load settings: %v", err)
		return
	}

	// Check if feature is enabled
	if settings == nil || !settings.Metrics.NotifyOnMissedPing {
		m.logger.Debug("Missed ping notifications are disabled")
		return
	}

	// Get timeout threshold
	timeoutMinutes := settings.Metrics.MissedPingTimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = DefaultMissedPingTimeoutMinutes
	}
	timeout := time.Duration(timeoutMinutes) * time.Minute

	// Get all devices
	devices, err := deviceRepo.GetDevices(ctx)
	if err != nil {
		m.logger.Errorf("Failed to get devices: %v", err)
		return
	}

	// Get last seen times for all devices
	lastSeenTimes, err := deviceRepo.GetDevicesLastSeenTimes(ctx)
	if err != nil {
		m.logger.Errorf("Failed to get device last seen times: %v", err)
		return
	}

	now := time.Now()

	for _, device := range devices {
		// Skip archived or muted devices
		if device.Archived || device.Muted {
			m.logger.Debugf("Skipping device %s - archived: %v, muted: %v", device.WWN, device.Archived, device.Muted)
			continue
		}

		lastSeen, exists := lastSeenTimes[device.WWN]
		if !exists {
			// Device has never sent data - this might be a newly registered device
			m.logger.Debugf("Device %s has no last seen time (newly registered?)", device.WWN)
			continue
		}

		timeSinceLastSeen := now.Sub(lastSeen)

		if timeSinceLastSeen > timeout {
			m.handleMissedPing(device, lastSeen, timeoutMinutes)
		} else {
			// Device is healthy - clear any previous notification state
			m.clearNotificationState(device.WWN)
		}
	}
}

func (m *MissedPingMonitor) handleMissedPing(device models.Device, lastSeen time.Time, timeoutMinutes int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we've already notified about this device recently
	// We don't want to spam notifications - only notify once per timeout period
	if lastNotified, exists := m.notifiedDevices[device.WWN]; exists {
		timeSinceNotification := time.Since(lastNotified)
		// Don't re-notify for at least the timeout period
		if timeSinceNotification < time.Duration(timeoutMinutes)*time.Minute {
			m.logger.Debugf("Already notified about device %s %v ago, skipping", device.WWN, timeSinceNotification.Round(time.Minute))
			return
		}
	}

	m.logger.Warnf("Device %s (%s) has not sent data for %v (threshold: %d minutes)",
		device.WWN, device.DeviceName, time.Since(lastSeen).Round(time.Minute), timeoutMinutes)

	// Send notification
	notification := notify.NewMissedPing(m.logger, m.appEngine.Config, device, lastSeen, timeoutMinutes)
	if err := notification.Send(); err != nil {
		m.logger.Errorf("Failed to send missed ping notification for device %s: %v", device.WWN, err)
		return
	}

	// Record that we've notified about this device
	m.notifiedDevices[device.WWN] = time.Now()
	m.logger.Infof("Sent missed ping notification for device %s", device.WWN)
}

func (m *MissedPingMonitor) clearNotificationState(wwn string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.notifiedDevices[wwn]; exists {
		delete(m.notifiedDevices, wwn)
		m.logger.Debugf("Cleared missed ping notification state for device %s (device is now healthy)", wwn)
	}
}
