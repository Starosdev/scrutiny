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

	// Persistent repository connection (created once, reused)
	deviceRepo database.DeviceRepo
	repoMu     sync.Mutex

	// Channel to signal shutdown and context for cancellation
	stopCh chan struct{}
	ctx    context.Context
	cancel context.CancelFunc

	// WaitGroup to track when the run goroutine has finished
	wg sync.WaitGroup
}

// NewMissedPingMonitor creates a new missed ping monitor
func NewMissedPingMonitor(ae *AppEngine) *MissedPingMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &MissedPingMonitor{
		appEngine:       ae,
		logger:          ae.Logger,
		notifiedDevices: make(map[string]time.Time),
		stopCh:          make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start begins the background monitoring loop
func (m *MissedPingMonitor) Start() {
	m.wg.Add(1)
	go m.run()
}

// Stop signals the monitor to stop and waits for it to finish
func (m *MissedPingMonitor) Stop() {
	m.logger.Debug("Stopping missed ping monitor...")
	m.cancel() // Cancel the context first to interrupt any in-flight operations
	close(m.stopCh)
	m.wg.Wait() // Wait for the run goroutine to finish

	// Close the persistent repository connection if it exists
	m.repoMu.Lock()
	if m.deviceRepo != nil {
		m.deviceRepo.Close()
		m.deviceRepo = nil
	}
	m.repoMu.Unlock()

	m.logger.Info("Missed ping monitor stopped")
}

func (m *MissedPingMonitor) run() {
	defer m.wg.Done()

	// Load initial settings to get check interval
	checkInterval := m.getCheckInterval()
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	m.logger.Infof("Missed ping monitor started with check interval: %v", checkInterval)

	for {
		select {
		case <-m.stopCh:
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

// getOrCreateRepo returns the persistent repository, creating it if necessary
func (m *MissedPingMonitor) getOrCreateRepo() (database.DeviceRepo, error) {
	m.repoMu.Lock()
	defer m.repoMu.Unlock()

	if m.deviceRepo != nil {
		return m.deviceRepo, nil
	}

	repo, err := database.NewScrutinyRepository(m.appEngine.Config, m.logger)
	if err != nil {
		return nil, err
	}

	m.deviceRepo = repo
	return m.deviceRepo, nil
}

// resetRepo closes and clears the persistent repository (called on connection errors)
func (m *MissedPingMonitor) resetRepo() {
	m.repoMu.Lock()
	defer m.repoMu.Unlock()

	if m.deviceRepo != nil {
		m.deviceRepo.Close()
		m.deviceRepo = nil
	}
}

func (m *MissedPingMonitor) getCheckInterval() time.Duration {
	// Check if context is already cancelled
	if m.ctx.Err() != nil {
		return time.Duration(DefaultMissedPingCheckIntervalMins) * time.Minute
	}

	// Try to get or create repository
	deviceRepo, err := m.getOrCreateRepo()
	if err != nil {
		m.logger.Warnf("Failed to create repository for settings: %v, using default interval", err)
		return time.Duration(DefaultMissedPingCheckIntervalMins) * time.Minute
	}

	settings, err := deviceRepo.LoadSettings(m.ctx)
	if err != nil {
		// On error, reset the repo so it will be recreated next time
		m.resetRepo()
		return time.Duration(DefaultMissedPingCheckIntervalMins) * time.Minute
	}
	if settings == nil {
		return time.Duration(DefaultMissedPingCheckIntervalMins) * time.Minute
	}

	interval := settings.Metrics.MissedPingCheckIntervalMins
	if interval <= 0 {
		interval = DefaultMissedPingCheckIntervalMins
	}

	return time.Duration(interval) * time.Minute
}

// checkMissedPingsData holds the data needed to check for missed pings
type checkMissedPingsData struct {
	timeoutMinutes int
	timeout        time.Duration
	devices        []models.Device
	lastSeenTimes  map[string]time.Time
}

// loadCheckData loads all data needed for missed ping checks
func (m *MissedPingMonitor) loadCheckData() (*checkMissedPingsData, error) {
	deviceRepo, err := m.getOrCreateRepo()
	if err != nil {
		return nil, err
	}

	settings, err := deviceRepo.LoadSettings(m.ctx)
	if err != nil {
		m.resetRepo()
		return nil, err
	}

	if settings == nil || !settings.Metrics.NotifyOnMissedPing {
		return nil, nil // Feature disabled, not an error
	}

	timeoutMinutes := settings.Metrics.MissedPingTimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = DefaultMissedPingTimeoutMinutes
	}

	devices, err := deviceRepo.GetDevices(m.ctx)
	if err != nil {
		m.resetRepo()
		return nil, err
	}

	lastSeenTimes, err := deviceRepo.GetDevicesLastSeenTimes(m.ctx)
	if err != nil {
		m.resetRepo()
		return nil, err
	}

	return &checkMissedPingsData{
		timeoutMinutes: timeoutMinutes,
		timeout:        time.Duration(timeoutMinutes) * time.Minute,
		devices:        devices,
		lastSeenTimes:  lastSeenTimes,
	}, nil
}

// processDevice checks a single device for missed pings
func (m *MissedPingMonitor) processDevice(device models.Device, data *checkMissedPingsData, now time.Time) {
	if device.Archived || device.Muted {
		m.logger.Debugf("Skipping device %s - archived: %v, muted: %v", device.WWN, device.Archived, device.Muted)
		return
	}

	lastSeen, exists := data.lastSeenTimes[device.WWN]
	if !exists {
		m.logger.Debugf("Device %s has no last seen time (newly registered?)", device.WWN)
		return
	}

	if now.Sub(lastSeen) > data.timeout {
		m.handleMissedPing(device, lastSeen, data.timeoutMinutes)
	} else {
		m.clearNotificationState(device.WWN)
	}
}

func (m *MissedPingMonitor) checkMissedPings() {
	if m.ctx.Err() != nil {
		return
	}

	data, err := m.loadCheckData()
	if err != nil {
		m.logger.Errorf("Failed to load data for missed ping check: %v", err)
		return
	}
	if data == nil {
		m.logger.Debug("Missed ping notifications are disabled")
		return
	}

	now := time.Now()
	currentDeviceWWNs := make(map[string]bool, len(data.devices))

	for _, device := range data.devices {
		currentDeviceWWNs[device.WWN] = true
		m.processDevice(device, data, now)
	}

	m.cleanupStaleNotifications(currentDeviceWWNs)
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

// cleanupStaleNotifications removes entries from notifiedDevices for devices that no longer exist
func (m *MissedPingMonitor) cleanupStaleNotifications(currentDeviceWWNs map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for wwn := range m.notifiedDevices {
		if !currentDeviceWWNs[wwn] {
			delete(m.notifiedDevices, wwn)
			m.logger.Debugf("Cleaned up stale notification state for deleted device %s", wwn)
		}
	}
}

// GetNotifiedDevicesCount returns the number of devices currently in the notified state (for testing)
func (m *MissedPingMonitor) GetNotifiedDevicesCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.notifiedDevices)
}

// IsDeviceNotified returns whether a device is currently in the notified state (for testing)
func (m *MissedPingMonitor) IsDeviceNotified(wwn string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.notifiedDevices[wwn]
	return exists
}
