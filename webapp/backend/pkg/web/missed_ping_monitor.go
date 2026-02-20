package web

import (
	"context"
	"fmt"
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

	// Status tracking for diagnostics
	lastCheckTime time.Time
	nextCheckTime time.Time
	lastError     error
	lastErrorTime time.Time
	statusMu      sync.RWMutex // Separate mutex for status fields
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

	// Set initial next check time
	m.statusMu.Lock()
	m.nextCheckTime = time.Now().Add(checkInterval)
	m.statusMu.Unlock()

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

			// Update next check time
			m.statusMu.Lock()
			m.nextCheckTime = time.Now().Add(checkInterval)
			m.statusMu.Unlock()
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
		m.logger.Errorf("Failed to get/create repository: %v", err)
		return nil, err
	}

	settings, err := deviceRepo.LoadSettings(m.ctx)
	if err != nil {
		m.resetRepo()
		m.logger.Errorf("Failed to load settings from database: %v", err)
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
		m.logger.Errorf("Failed to load devices from database: %v", err)
		return nil, err
	}

	lastSeenTimes, err := deviceRepo.GetDevicesLastSeenTimes(m.ctx)
	if err != nil {
		m.resetRepo()
		m.logger.Errorf("Failed to query last seen times from InfluxDB: %v (Check InfluxDB connection and bucket configuration)", err)
		return nil, err
	}

	m.logger.Debugf("Loaded missed ping check data: %d devices, timeout=%dm", len(devices), timeoutMinutes)

	return &checkMissedPingsData{
		timeoutMinutes: timeoutMinutes,
		timeout:        time.Duration(timeoutMinutes) * time.Minute,
		devices:        devices,
		lastSeenTimes:  lastSeenTimes,
	}, nil
}

// checkDevice checks a single device for missed pings and returns digest data if missed.
// Returns nil if the device is not missed (healthy, skipped, or already notified recently).
func (m *MissedPingMonitor) checkDevice(device *models.Device, data *checkMissedPingsData, now time.Time) *notify.MissedPingDigestDevice {
	if device.Archived || device.Muted {
		m.logger.Debugf("Skipping device %s - archived: %v, muted: %v", device.WWN, device.Archived, device.Muted)
		return nil
	}

	lastSeen, exists := data.lastSeenTimes[device.WWN]
	if !exists {
		m.logger.Debugf("Device %s has no last seen time (newly registered?)", device.WWN)
		return nil
	}

	if now.Sub(lastSeen) <= data.timeout {
		m.clearNotificationState(device.WWN)
		return nil
	}

	// Device has missed pings -- check dedup before including in digest
	m.mu.RLock()
	lastNotified, alreadyNotified := m.notifiedDevices[device.WWN]
	m.mu.RUnlock()

	if alreadyNotified {
		timeSinceNotification := time.Since(lastNotified)
		if timeSinceNotification < time.Duration(data.timeoutMinutes)*time.Minute {
			m.logger.Debugf("Already notified about device %s %v ago, skipping", device.WWN, timeSinceNotification.Round(time.Minute))
			return nil
		}
	}

	m.logger.Warnf("Device %s (%s) has not sent data for %v (threshold: %d minutes)",
		device.WWN, device.DeviceName, time.Since(lastSeen).Round(time.Minute), data.timeoutMinutes)

	return &notify.MissedPingDigestDevice{
		WWN:          device.WWN,
		DeviceName:   device.DeviceName,
		SerialNumber: device.SerialNumber,
		HostId:       device.HostId,
		Label:        device.Label,
		LastSeen:     lastSeen,
	}
}

func (m *MissedPingMonitor) checkMissedPings() {
	if m.ctx.Err() != nil {
		return
	}

	// Update last check time
	m.statusMu.Lock()
	m.lastCheckTime = time.Now()
	m.statusMu.Unlock()

	data, err := m.loadCheckData()
	if err != nil {
		m.logger.Errorf("Failed to load data for missed ping check: %v", err)

		// Store error for diagnostics
		m.statusMu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.statusMu.Unlock()
		return
	}
	if data == nil {
		m.logger.Info("Missed ping notifications are disabled")

		// Clear error since feature is intentionally disabled
		m.statusMu.Lock()
		m.lastError = nil
		m.statusMu.Unlock()
		return
	}

	// Clear previous error on successful data load
	m.statusMu.Lock()
	m.lastError = nil
	m.statusMu.Unlock()

	now := time.Now()
	currentDeviceWWNs := make(map[string]bool, len(data.devices))
	var missed []notify.MissedPingDigestDevice

	for _, device := range data.devices {
		currentDeviceWWNs[device.WWN] = true
		if md := m.checkDevice(&device, data, now); md != nil {
			missed = append(missed, *md)
		}
	}

	if len(missed) > 0 {
		m.sendMissedPingDigest(missed, data.timeoutMinutes)
	}

	m.cleanupStaleNotifications(currentDeviceWWNs)
}

func (m *MissedPingMonitor) sendMissedPingDigest(devices []notify.MissedPingDigestDevice, timeoutMinutes int) {
	notification := notify.NewMissedPingDigest(m.logger, m.appEngine.Config, devices, timeoutMinutes)
	if err := notification.Send(); err != nil {
		if err.Error() == "no notification endpoints configured" {
			m.logger.Warnf("Missed pings detected for %d device(s) but no notification endpoints are configured.", len(devices))
			return
		}
		m.logger.Errorf("Failed to send missed ping digest notification: %v", err)
		return
	}

	// Mark all devices as notified
	m.mu.Lock()
	now := time.Now()
	for _, d := range devices {
		m.notifiedDevices[d.WWN] = now
	}
	m.mu.Unlock()

	m.logger.Infof("Sent missed ping digest notification for %d device(s)", len(devices))
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

// GetStatus returns a comprehensive snapshot of the monitor's current status for diagnostics
func (m *MissedPingMonitor) GetStatus(ctx context.Context) (*models.MissedPingStatusData, error) {
	// Get status timing information
	m.statusMu.RLock()
	lastCheck := m.lastCheckTime
	nextCheck := m.nextCheckTime
	lastErr := m.lastError
	lastErrTime := m.lastErrorTime
	m.statusMu.RUnlock()

	// Load current settings
	deviceRepo, err := m.getOrCreateRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to access repository: %w", err)
	}

	settings, err := deviceRepo.LoadSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	// Set defaults if not configured
	timeoutMinutes := DefaultMissedPingTimeoutMinutes
	checkInterval := DefaultMissedPingCheckIntervalMins
	enabled := false

	if settings != nil {
		enabled = settings.Metrics.NotifyOnMissedPing
		if settings.Metrics.MissedPingTimeoutMinutes > 0 {
			timeoutMinutes = settings.Metrics.MissedPingTimeoutMinutes
		}
		if settings.Metrics.MissedPingCheckIntervalMins > 0 {
			checkInterval = settings.Metrics.MissedPingCheckIntervalMins
		}
	}

	// Get device counts
	devices, err := deviceRepo.GetDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	totalDevices := len(devices)
	monitoredDevices := 0
	for _, device := range devices {
		if !device.Archived && !device.Muted {
			monitoredDevices++
		}
	}

	// Get last seen times from InfluxDB
	lastSeenTimes, _ := deviceRepo.GetDevicesLastSeenTimes(ctx)

	// Get notified devices info
	m.mu.RLock()
	notifiedDevices := make([]string, 0, len(m.notifiedDevices))
	notifiedDetails := make([]models.NotifiedDeviceInfo, 0, len(m.notifiedDevices))

	for wwn, notifyTime := range m.notifiedDevices {
		notifiedDevices = append(notifiedDevices, wwn)

		// Find device details
		var deviceName string
		for _, device := range devices {
			if device.WWN == wwn {
				deviceName = device.DeviceName
				break
			}
		}

		// Get last seen time from InfluxDB
		var lastSeenTime time.Time
		if lst, ok := lastSeenTimes[wwn]; ok {
			lastSeenTime = lst
		}

		notifiedDetails = append(notifiedDetails, models.NotifiedDeviceInfo{
			WWN:              wwn,
			DeviceName:       deviceName,
			NotificationTime: notifyTime.Format(time.RFC3339),
			LastSeenTime:     lastSeenTime.Format(time.RFC3339),
		})
	}
	m.mu.RUnlock()

	// Validate InfluxDB buckets
	influxStatus := m.validateInfluxDBBuckets(ctx, deviceRepo)

	// Check notification configuration
	notifyUrls := m.appEngine.Config.GetStringSlice("notify.urls")
	notifyConfigured := len(notifyUrls) > 0

	status := &models.MissedPingStatusData{
		Enabled:                enabled,
		TimeoutMinutes:         timeoutMinutes,
		CheckIntervalMinutes:   checkInterval,
		NotifyConfigured:       notifyConfigured,
		NotifyEndpointCount:    len(notifyUrls),
		LastCheckTime:          lastCheck.Format(time.RFC3339),
		NextCheckTime:          nextCheck.Format(time.RFC3339),
		MonitorRunning:         m.ctx.Err() == nil,
		TotalDevices:           totalDevices,
		MonitoredDevices:       monitoredDevices,
		NotifiedDevices:        notifiedDevices,
		NotifiedDevicesDetails: notifiedDetails,
		InfluxDBStatus:         influxStatus,
	}

	if lastErr != nil {
		status.LastError = lastErr.Error()
		status.LastErrorTime = lastErrTime.Format(time.RFC3339)
	}

	return status, nil
}

// validateInfluxDBBuckets checks if all required InfluxDB buckets exist
func (m *MissedPingMonitor) validateInfluxDBBuckets(ctx context.Context, repo database.DeviceRepo) models.InfluxDBStatusInfo {
	// Get base bucket name from config
	baseBucket := m.appEngine.Config.GetString("web.influxdb.bucket")

	// Expected buckets (same as used in GetDevicesLastSeenTimes query)
	expectedBuckets := []string{
		baseBucket,
		baseBucket + "_weekly",
		baseBucket + "_monthly",
		baseBucket + "_yearly",
	}

	// Query available buckets from InfluxDB
	availableBuckets, err := repo.GetAvailableInfluxDBBuckets(ctx)
	if err != nil {
		return models.InfluxDBStatusInfo{
			Available:      false,
			BucketsFound:   []string{},
			BucketsMissing: expectedBuckets,
			Error:          err.Error(),
		}
	}

	// Check which buckets exist
	bucketSet := make(map[string]bool)
	for _, bucket := range availableBuckets {
		bucketSet[bucket] = true
	}

	bucketsFound := []string{}
	bucketsMissing := []string{}

	for _, expected := range expectedBuckets {
		if bucketSet[expected] {
			bucketsFound = append(bucketsFound, expected)
		} else {
			bucketsMissing = append(bucketsMissing, expected)
		}
	}

	return models.InfluxDBStatusInfo{
		Available:      len(bucketsMissing) == 0,
		BucketsFound:   bucketsFound,
		BucketsMissing: bucketsMissing,
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
