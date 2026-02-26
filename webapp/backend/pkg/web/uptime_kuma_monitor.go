package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultUptimeKumaIntervalSeconds is the default interval between Uptime Kuma pushes
	DefaultUptimeKumaIntervalSeconds = 60

	// maxPushMessageLength is the maximum length for the msg query parameter
	maxPushMessageLength = 250
)

// UptimeKumaMonitor sends periodic health status pushes to an Uptime Kuma Push Monitor endpoint
type UptimeKumaMonitor struct {
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

// NewUptimeKumaMonitor creates a new Uptime Kuma push monitor
func NewUptimeKumaMonitor(ae *AppEngine) *UptimeKumaMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &UptimeKumaMonitor{
		appEngine: ae,
		logger:    ae.Logger,
		stopCh:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins the background push loop
func (m *UptimeKumaMonitor) Start() {
	m.wg.Add(1)
	go m.run()
}

// Stop signals the monitor to stop and waits for it to finish
func (m *UptimeKumaMonitor) Stop() {
	m.logger.Debug("Stopping Uptime Kuma monitor...")
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

	m.logger.Info("Uptime Kuma monitor stopped")
}

func (m *UptimeKumaMonitor) run() {
	defer m.wg.Done()

	// Load initial settings to get interval
	interval := m.getInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.logger.Infof("Uptime Kuma monitor started with interval: %v", interval)

	// Set initial next check time
	m.statusMu.Lock()
	m.nextCheckTime = time.Now().Add(interval)
	m.statusMu.Unlock()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAndPush()

			// Update ticker interval in case settings changed
			newInterval := m.getInterval()
			if newInterval != interval {
				ticker.Reset(newInterval)
				interval = newInterval
				m.logger.Debugf("Uptime Kuma interval updated to: %v", newInterval)
			}

			// Update next check time
			m.statusMu.Lock()
			m.nextCheckTime = time.Now().Add(interval)
			m.statusMu.Unlock()
		}
	}
}

// getOrCreateRepo returns the persistent repository, creating it if necessary
func (m *UptimeKumaMonitor) getOrCreateRepo() (database.DeviceRepo, error) {
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
func (m *UptimeKumaMonitor) resetRepo() {
	m.repoMu.Lock()
	defer m.repoMu.Unlock()

	if m.deviceRepo != nil {
		m.deviceRepo.Close()
		m.deviceRepo = nil
	}
}

func (m *UptimeKumaMonitor) getInterval() time.Duration {
	// Check if context is already cancelled
	if m.ctx.Err() != nil {
		return time.Duration(DefaultUptimeKumaIntervalSeconds) * time.Second
	}

	// Try to get or create repository
	deviceRepo, err := m.getOrCreateRepo()
	if err != nil {
		m.logger.Warnf("Failed to create repository for Uptime Kuma settings: %v, using default interval", err)
		return time.Duration(DefaultUptimeKumaIntervalSeconds) * time.Second
	}

	settings, err := deviceRepo.LoadSettings(m.ctx)
	if err != nil {
		m.resetRepo()
		return time.Duration(DefaultUptimeKumaIntervalSeconds) * time.Second
	}
	if settings == nil {
		return time.Duration(DefaultUptimeKumaIntervalSeconds) * time.Second
	}

	interval := settings.Metrics.UptimeKumaIntervalSeconds
	if interval <= 0 {
		interval = DefaultUptimeKumaIntervalSeconds
	}

	return time.Duration(interval) * time.Second
}

// getPushURL returns the Uptime Kuma push URL, checking config file first then settings DB
func (m *UptimeKumaMonitor) getPushURL(settings *models.Settings) string {
	// Config file / env var takes precedence
	configURL := m.appEngine.Config.GetString("web.uptime_kuma.push_url")
	if configURL != "" {
		return configURL
	}

	// Fall back to settings from the database
	if settings != nil {
		return settings.Metrics.UptimeKumaPushURL
	}

	return ""
}

func (m *UptimeKumaMonitor) checkAndPush() {
	if m.ctx.Err() != nil {
		return
	}

	// Update last check time
	m.statusMu.Lock()
	m.lastCheckTime = time.Now()
	m.statusMu.Unlock()

	deviceRepo, err := m.getOrCreateRepo()
	if err != nil {
		m.logger.Errorf("Failed to get/create repository for Uptime Kuma push: %v", err)
		m.statusMu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.statusMu.Unlock()
		return
	}

	settings, err := deviceRepo.LoadSettings(m.ctx)
	if err != nil {
		m.resetRepo()
		m.logger.Errorf("Failed to load settings for Uptime Kuma push: %v", err)
		m.statusMu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.statusMu.Unlock()
		return
	}

	if settings == nil || !settings.Metrics.UptimeKumaEnabled {
		m.logger.Debug("Uptime Kuma push monitor is disabled")
		m.statusMu.Lock()
		m.lastError = nil
		m.statusMu.Unlock()
		return
	}

	pushURL := m.getPushURL(settings)
	if pushURL == "" {
		m.logger.Debug("Uptime Kuma push URL is not configured, skipping push")
		m.statusMu.Lock()
		m.lastError = nil
		m.statusMu.Unlock()
		return
	}

	devices, err := deviceRepo.GetDevices(m.ctx)
	if err != nil {
		m.resetRepo()
		m.logger.Errorf("Failed to load devices for Uptime Kuma push: %v", err)
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

	status, msg := BuildPushMessage(devices)

	// Measure push duration for the ping parameter
	start := time.Now()
	if err := sendPush(pushURL, status, msg, start); err != nil {
		m.logger.Errorf("Failed to send Uptime Kuma push: %v", err)
		m.statusMu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.statusMu.Unlock()
		return
	}

	m.logger.Infof("Uptime Kuma push sent: status=%s msg=%s", status, msg)
}

// BuildPushMessage determines the push status and message based on device health.
// It filters out archived and muted devices. Returns (status, msg) where status
// is "up" or "down".
func BuildPushMessage(devices []models.Device) (status string, msg string) {
	var monitored []models.Device
	for _, d := range devices {
		if d.Archived || d.Muted {
			continue
		}
		monitored = append(monitored, d)
	}

	totalCount := len(monitored)

	// No monitored devices
	if totalCount == 0 {
		return "up", "No monitored drives found"
	}

	// Check for failures
	var failing []models.Device
	for _, d := range monitored {
		if d.DeviceStatus != pkg.DeviceStatusPassed {
			failing = append(failing, d)
		}
	}

	if len(failing) == 0 {
		return "up", fmt.Sprintf("All %d monitored drives healthy", totalCount)
	}

	// Build failure message
	var parts []string
	for _, d := range failing {
		name := d.DeviceName
		if name == "" {
			name = d.SerialNumber
		}
		dtype := d.DeviceProtocol
		if dtype == "" {
			dtype = "unknown"
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", name, dtype))
	}

	prefix := fmt.Sprintf("%d of %d drives failing: ", len(failing), totalCount)
	detail := strings.Join(parts, ", ")
	fullMsg := prefix + detail

	if len(fullMsg) > maxPushMessageLength {
		// Truncate and add ellipsis
		fullMsg = fullMsg[:maxPushMessageLength-3] + "..."
	}

	return "down", fullMsg
}

// sendPush sends an HTTP GET to the Uptime Kuma push endpoint with status, msg, and ping params
func sendPush(pushURL, status, msg string, start time.Time) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Build URL with query parameters
	u, err := url.Parse(pushURL)
	if err != nil {
		return fmt.Errorf("invalid push URL: %w", err)
	}

	q := u.Query()
	q.Set("status", status)
	q.Set("msg", msg)
	q.Set("ping", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
	u.RawQuery = q.Encode()

	resp, err := client.Get(u.String())
	if err != nil {
		return fmt.Errorf("push request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push returned HTTP %d", resp.StatusCode)
	}

	return nil
}
