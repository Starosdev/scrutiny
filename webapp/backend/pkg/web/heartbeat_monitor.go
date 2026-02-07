package web

import (
	"context"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultHeartbeatIntervalHours is the default interval between heartbeat notifications
	DefaultHeartbeatIntervalHours = 24
)

// HeartbeatMonitor sends periodic "all clear" notifications when all drives are healthy
type HeartbeatMonitor struct {
	appEngine *AppEngine
	logger    logrus.FieldLogger

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
	statusMu      sync.RWMutex
}

// NewHeartbeatMonitor creates a new heartbeat monitor
func NewHeartbeatMonitor(ae *AppEngine) *HeartbeatMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &HeartbeatMonitor{
		appEngine: ae,
		logger:    ae.Logger,
		stopCh:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins the background heartbeat loop
func (m *HeartbeatMonitor) Start() {
	m.wg.Add(1)
	go m.run()
}

// Stop signals the monitor to stop and waits for it to finish
func (m *HeartbeatMonitor) Stop() {
	m.logger.Debug("Stopping heartbeat monitor...")
	m.cancel()
	close(m.stopCh)
	m.wg.Wait()

	// Close the persistent repository connection if it exists
	m.repoMu.Lock()
	if m.deviceRepo != nil {
		m.deviceRepo.Close()
		m.deviceRepo = nil
	}
	m.repoMu.Unlock()

	m.logger.Info("Heartbeat monitor stopped")
}

func (m *HeartbeatMonitor) run() {
	defer m.wg.Done()

	// Load initial settings to get heartbeat interval
	heartbeatInterval := m.getHeartbeatInterval()
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	m.logger.Infof("Heartbeat monitor started with interval: %v", heartbeatInterval)

	// Set initial next check time
	m.statusMu.Lock()
	m.nextCheckTime = time.Now().Add(heartbeatInterval)
	m.statusMu.Unlock()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAndSendHeartbeat()

			// Update ticker interval in case settings changed
			newInterval := m.getHeartbeatInterval()
			if newInterval != heartbeatInterval {
				ticker.Reset(newInterval)
				heartbeatInterval = newInterval
				m.logger.Debugf("Heartbeat interval updated to: %v", heartbeatInterval)
			}

			// Update next check time
			m.statusMu.Lock()
			m.nextCheckTime = time.Now().Add(heartbeatInterval)
			m.statusMu.Unlock()
		}
	}
}

// getOrCreateRepo returns the persistent repository, creating it if necessary
func (m *HeartbeatMonitor) getOrCreateRepo() (database.DeviceRepo, error) {
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
func (m *HeartbeatMonitor) resetRepo() {
	m.repoMu.Lock()
	defer m.repoMu.Unlock()

	if m.deviceRepo != nil {
		m.deviceRepo.Close()
		m.deviceRepo = nil
	}
}

func (m *HeartbeatMonitor) getHeartbeatInterval() time.Duration {
	// Check if context is already cancelled
	if m.ctx.Err() != nil {
		return time.Duration(DefaultHeartbeatIntervalHours) * time.Hour
	}

	// Try to get or create repository
	deviceRepo, err := m.getOrCreateRepo()
	if err != nil {
		m.logger.Warnf("Failed to create repository for heartbeat settings: %v, using default interval", err)
		return time.Duration(DefaultHeartbeatIntervalHours) * time.Hour
	}

	settings, err := deviceRepo.LoadSettings(m.ctx)
	if err != nil {
		m.resetRepo()
		return time.Duration(DefaultHeartbeatIntervalHours) * time.Hour
	}
	if settings == nil {
		return time.Duration(DefaultHeartbeatIntervalHours) * time.Hour
	}

	interval := settings.Metrics.HeartbeatIntervalHours
	if interval <= 0 {
		interval = DefaultHeartbeatIntervalHours
	}

	return time.Duration(interval) * time.Hour
}

func (m *HeartbeatMonitor) checkAndSendHeartbeat() {
	if m.ctx.Err() != nil {
		return
	}

	// Update last check time
	m.statusMu.Lock()
	m.lastCheckTime = time.Now()
	m.statusMu.Unlock()

	deviceRepo, err := m.getOrCreateRepo()
	if err != nil {
		m.logger.Errorf("Failed to get/create repository for heartbeat: %v", err)
		m.statusMu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.statusMu.Unlock()
		return
	}

	settings, err := deviceRepo.LoadSettings(m.ctx)
	if err != nil {
		m.resetRepo()
		m.logger.Errorf("Failed to load settings for heartbeat: %v", err)
		m.statusMu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.statusMu.Unlock()
		return
	}

	if settings == nil || !settings.Metrics.HeartbeatEnabled {
		m.logger.Debug("Heartbeat notifications are disabled")
		m.statusMu.Lock()
		m.lastError = nil
		m.statusMu.Unlock()
		return
	}

	devices, err := deviceRepo.GetDevices(m.ctx)
	if err != nil {
		m.resetRepo()
		m.logger.Errorf("Failed to load devices for heartbeat: %v", err)
		m.statusMu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.statusMu.Unlock()
		return
	}

	// Clear previous error on successful data load
	m.statusMu.Lock()
	m.lastError = nil
	m.statusMu.Unlock()

	// Filter to monitored devices (not archived, not muted)
	totalCount := len(devices)
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

	// Don't send heartbeat if no monitored devices
	if monitoredCount == 0 {
		m.logger.Debug("No monitored devices found, skipping heartbeat")
		return
	}

	// Don't send heartbeat if any device has failures
	if !allHealthy {
		m.logger.Debug("Active drive failures detected, skipping heartbeat (failure notifications take priority)")
		return
	}

	// All monitored drives are healthy -- send heartbeat
	m.logger.Infof("All %d monitored drives healthy, sending heartbeat notification", monitoredCount)

	notification := notify.NewHeartbeat(m.logger, m.appEngine.Config, monitoredCount, totalCount)
	if err := notification.Send(); err != nil {
		if err.Error() == "no notification endpoints configured" {
			m.logger.Warn("Heartbeat ready but no notification endpoints are configured. Configure notify.urls in scrutiny.yaml to receive heartbeat alerts.")
			return
		}
		m.logger.Errorf("Failed to send heartbeat notification: %v", err)
		return
	}

	m.logger.Info("Heartbeat notification sent successfully")
}
