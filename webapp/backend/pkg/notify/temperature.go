package notify

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/sirupsen/logrus"
)

// Each device has its own lock so history reads and delivery retries cannot race
// with a reset or another upload, while unrelated drives proceed independently.
type TemperatureTracker struct {
	mu      sync.Mutex
	devices map[string]*temperatureState
}

type temperatureState struct {
	mu         sync.Mutex
	since      time.Time
	notified   bool
	canSeed    bool
	configured bool
	threshold  int
	duration   time.Duration
}

func NewTemperatureTracker() *TemperatureTracker {
	return &TemperatureTracker{devices: make(map[string]*temperatureState)}
}

func (t *TemperatureTracker) state(deviceID string) *temperatureState {
	t.mu.Lock()
	defer t.mu.Unlock()
	state, exists := t.devices[deviceID]
	if !exists {
		state = &temperatureState{canSeed: true}
		t.devices[deviceID] = state
	}
	return state
}

func (t *TemperatureTracker) HasState(deviceID string) bool {
	s := t.state(deviceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.since.IsZero()
}

func (t *TemperatureTracker) Unmark(deviceID string) {
	s := t.state(deviceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notified = false
}

// Forget retains a tombstone: resuming must not revive an earlier hot history.
func (t *TemperatureTracker) Forget(deviceID string) {
	s := t.state(deviceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reset()
}

func (s *temperatureState) reset() {
	s.since = time.Time{}
	s.notified = false
	s.canSeed = false
}

func (t *TemperatureTracker) Observe(deviceID string, tempC int64, thresholdC int, minDuration time.Duration, now, firstSeen time.Time) (bool, time.Time) {
	s := t.state(deviceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.observe(tempC, thresholdC, minDuration, now, firstSeen)
}

func (s *temperatureState) observe(tempC int64, thresholdC int, minDuration time.Duration, now, firstSeen time.Time) (bool, time.Time) {
	if s.configured && (s.threshold != thresholdC || s.duration != minDuration) {
		s.reset()
	}
	s.configured, s.threshold, s.duration = true, thresholdC, minDuration
	if tempC <= 0 {
		return false, s.since
	}
	if tempC < int64(thresholdC) {
		s.reset()
		return false, s.since
	}
	if s.since.IsZero() {
		s.since = now
		if s.canSeed && !firstSeen.IsZero() && !firstSeen.After(now) {
			s.since = firstSeen
		}
	}
	s.canSeed = false
	if !s.notified && now.Sub(s.since) >= minDuration {
		s.notified = true
		return true, s.since
	}
	return false, s.since
}

// Evaluate serializes the entire observation/send transaction for one device.
// A failed send preserves the streak and retries on the next hot upload.
func (t *TemperatureTracker) Evaluate(deviceID string, enabled bool, tempC int64, thresholdC int, minDuration time.Duration, now time.Time, seed func() time.Time, send func(time.Time) bool) {
	s := t.state(deviceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !enabled {
		s.reset()
		return
	}
	var firstSeen time.Time
	if s.canSeed && tempC > 0 && tempC >= int64(thresholdC) && seed != nil {
		firstSeen = seed()
	}
	if fire, since := s.observe(tempC, thresholdC, minDuration, now, firstSeen); fire {
		s.notified = send(since)
	}
}

// ExceededSince finds the current hot run without changing the caller's slice.
// Unknown readings do not break a run. Invalid/future timestamps are ignored.
func ExceededSince(history []measurements.SmartTemperature, thresholdC int, fallback time.Time) time.Time {
	points := append([]measurements.SmartTemperature(nil), history...)
	sort.SliceStable(points, func(i, j int) bool { return points[i].Date.Before(points[j].Date) })
	since := fallback
	for i := len(points) - 1; i >= 0; i-- {
		point := points[i]
		if point.Date.IsZero() || point.Date.After(fallback) || point.Temp <= 0 {
			continue
		}
		if point.Temp < int64(thresholdC) {
			break
		}
		since = point.Date
	}
	return since
}

func formatTemperature(c int64, unit string) string {
	const fahrenheitScale = 9.0 / 5.0
	const fahrenheitOffset = 32
	const fahrenheitDecimalPlaces = 1 // Whole Celsius degrees convert to tenths of Fahrenheit.
	if unit == "fahrenheit" {
		value := strconv.FormatFloat(float64(c)*fahrenheitScale+fahrenheitOffset, 'f', fahrenheitDecimalPlaces, 64)
		return strings.TrimSuffix(value, ".0") + "°F"
	}
	return fmt.Sprintf("%d°C", c)
}

func NewTemperatureNotify(logger logrus.FieldLogger, appconfig config.Interface, device *models.Device, tempC int64, thresholdC int, since, now time.Time, unit string) Notify {
	payload := NewPayload(*device, false)
	payload.FailureType = NotifyFailureTypeTemperature
	identifier := device.DeviceName
	if label := strings.TrimSpace(device.Label); label != "" {
		identifier = fmt.Sprintf(fmtLabelWithName, label, identifier)
	}
	if host := strings.TrimSpace(device.HostId); host != "" {
		identifier = fmt.Sprintf("[%s]%s", host, identifier)
	}
	elapsed := now.Sub(since).Truncate(time.Minute)
	if elapsed < 0 {
		elapsed = 0
	}
	condition := fmt.Sprintf("%s >= %s for %s", formatTemperature(tempC, unit), formatTemperature(int64(thresholdC), unit), elapsed)
	payload.Subject = fmt.Sprintf("Scrutiny temperature alert (%s) on device: %s", condition, identifier)
	rows := [][2]string{
		{notifyRowFailureType, NotifyFailureTypeTemperature},
		{"Temperature", condition}, {"Device", identifier},
		{notifyRowDeviceSerial, device.SerialNumber}, {notifyRowDeviceType, device.DeviceType},
		{"Exceeded Since", since.Format(time.RFC3339)}, {"Date", payload.Date},
	}
	parts := []string{payload.Subject}
	for _, row := range rows {
		parts = append(parts, row[0]+": "+row[1])
	}
	payload.Message = strings.Join(parts, "\n")
	const temperatureBannerColor = "#f59e0b"
	payload.HTMLMessage = formatNotificationHTML(payload.Subject, "Scrutiny temperature notification", "HIGH TEMPERATURE", temperatureBannerColor, rows, notifyFooterText)
	return Notify{Logger: logger, Config: appconfig, Payload: payload}
}
