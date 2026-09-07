package notify

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const testTemperatureThreshold = 55
const testTemperatureDuration = 30 * time.Minute

func TestTemperatureTrackerExcursions(t *testing.T) {
	state := temperatureState{canSeed: true}
	now := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.UTC)
	for _, step := range []struct {
		temp    int64
		elapsed time.Duration
		fire    bool
		since   time.Duration
	}{
		{0, 0, false, 0}, {-1, 0, false, 0}, {54, 0, false, 0},
		{55, time.Minute, false, time.Minute},
		{60, 30 * time.Minute, false, time.Minute},
		{0, 31 * time.Minute, false, time.Minute},
		{60, 31 * time.Minute, true, time.Minute},
		{60, time.Hour, false, time.Minute},
		{54, time.Hour, false, 0},
		{60, 2 * time.Hour, false, 2 * time.Hour},
		{60, 2*time.Hour + testTemperatureDuration, true, 2 * time.Hour},
	} {
		fire, since := state.observe(step.temp, testTemperatureThreshold, testTemperatureDuration, now.Add(step.elapsed), time.Time{})
		require.Equal(t, step.fire, fire)
		if step.since == 0 {
			require.True(t, since.IsZero())
		} else {
			require.Equal(t, now.Add(step.since), since)
		}
	}
}

func TestTemperatureStateSeedAndImmediate(t *testing.T) {
	now := time.Now()
	state := temperatureState{canSeed: true}
	seed := now.Add(-time.Hour)
	fire, since := state.observe(60, testTemperatureThreshold, testTemperatureDuration, now, seed)
	require.True(t, fire)
	require.Equal(t, seed, since)
	state.reset()
	fire, since = state.observe(60, testTemperatureThreshold, testTemperatureDuration, now, seed)
	require.False(t, fire)
	require.Equal(t, now, since)
	state = temperatureState{canSeed: true}
	fire, _ = state.observe(55, testTemperatureThreshold, 0, now, time.Time{})
	require.True(t, fire)
	state = temperatureState{canSeed: true}
	fire, since = state.observe(55, testTemperatureThreshold, testTemperatureDuration, now, now.Add(time.Hour))
	require.False(t, fire)
	require.Equal(t, now, since)
}

func TestTemperatureTrackerSettingChangesReset(t *testing.T) {
	now := time.Now()
	for _, change := range []struct {
		threshold int
		duration  time.Duration
	}{{65, testTemperatureDuration}, {55, time.Hour}} {
		state := temperatureState{canSeed: true}
		state.observe(60, testTemperatureThreshold, testTemperatureDuration, now, now.Add(-time.Hour))
		fire, since := state.observe(70, change.threshold, change.duration, now.Add(time.Hour), now)
		require.False(t, fire)
		require.Equal(t, now.Add(time.Hour), since)
	}
}

func TestTemperatureSeedOnlyBeforeFirstKnownReading(t *testing.T) {
	now := time.Now()
	for _, initial := range []int64{0, 40} {
		tracker := NewTemperatureTracker()
		seedCalls := 0
		sends := 0
		seed := func() time.Time { seedCalls++; return now.Add(-time.Hour) }
		send := func(time.Time) bool { sends++; return true }
		tracker.Evaluate("drive", initial, testTemperatureThreshold, testTemperatureDuration, now, seed, send)
		tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now, seed, send)
		if initial == 0 {
			require.Equal(t, 1, seedCalls)
			require.Equal(t, 1, sends)
		} else {
			require.Zero(t, seedCalls)
			require.Zero(t, sends)
		}
	}
}

func TestTemperatureEvaluationSerializesAndRetries(t *testing.T) {
	tracker := NewTemperatureTracker()
	now := time.Now()
	var seeds, sends atomic.Int32
	seed := func() time.Time { seeds.Add(1); return now.Add(-time.Hour) }
	send := func(time.Time) bool { return sends.Add(1) > 1 }
	const uploads = 20
	var workers sync.WaitGroup
	for range uploads {
		workers.Go(func() {
			tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now, seed, send)
		})
	}
	workers.Wait()
	require.EqualValues(t, 1, seeds.Load())
	require.EqualValues(t, 2, sends.Load(), "failed first attempt must retry once")
	tracker.Forget("drive")
	tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now, seed, send)
	require.EqualValues(t, 1, seeds.Load(), "resume must not reuse history")
	require.EqualValues(t, 2, sends.Load())
}

func TestTemperatureStalledDeliveryAllowsResetAndRetry(t *testing.T) {
	for _, action := range []string{"hot", "cold", "forget"} {
		t.Run(action, func(t *testing.T) {
			tracker := NewTemperatureTracker()
			now := time.Now()
			started, release, firstDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(firstDone)
				tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now,
					func() time.Time { return now.Add(-testTemperatureDuration) },
					func(time.Time) bool {
						close(started)
						select {
						case <-release:
						case <-time.After(transportTestAllowance):
						}
						return false
					})
			}()
			select {
			case <-started:
			case <-time.After(transportTestAllowance):
				t.Fatal("send never started")
			}
			// A different device must be able to finish before this send returns.
			otherDone := make(chan struct{})
			go func() {
				tracker.Evaluate("other", 40, testTemperatureThreshold, testTemperatureDuration, now, nil, nil)
				close(otherDone)
			}()
			select {
			case <-otherDone:
			case <-time.After(transportTestAllowance):
				t.Fatal("unrelated device blocked")
			}
			select {
			case <-firstDone:
				t.Fatal("stalled send returned before release")
			default:
			}
			var sent atomic.Int32
			send := func(time.Time) bool { sent.Add(1); return true }
			followingDone := make(chan struct{})
			go func() {
				defer close(followingDone)
				switch action {
				case "forget":
					tracker.Forget("drive")
				case "cold":
					tracker.Evaluate("drive", 40, testTemperatureThreshold, testTemperatureDuration, now, nil, send)
				case "hot":
					tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now, nil, send)
				}
			}()
			close(release)
			for _, done := range []chan struct{}{firstDone, followingDone} {
				select {
				case <-done:
				case <-time.After(transportTestAllowance):
					t.Fatal("device remained blocked after failed send")
				}
			}
			if action == "hot" {
				require.EqualValues(t, 1, sent.Load(), "failed hot send must retry")
			} else {
				tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now, func() time.Time { t.Error("reset must not seed again"); return now.Add(-time.Hour) }, send)
				require.Zero(t, sent.Load())
				tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now.Add(testTemperatureDuration), nil, send)
				require.EqualValues(t, 1, sent.Load(), "fresh excursion waits the full duration")
			}
			tracker.Evaluate("drive", 60, testTemperatureThreshold, testTemperatureDuration, now.Add(time.Hour), nil, send)
			require.EqualValues(t, 1, sent.Load(), "successful excursion must not repeat")
		})
	}
}

func TestExceededSince(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-time.Hour)
	middle := now.Add(-time.Minute)
	for _, tc := range []struct {
		name    string
		history []measurements.SmartTemperature
		want    time.Time
	}{
		{"empty", nil, now},
		{"cold", []measurements.SmartTemperature{{Date: old, Temp: 40}}, now},
		{"unsorted hot", []measurements.SmartTemperature{{Date: now, Temp: 60}, {Date: old, Temp: 55}}, old},
		{"cold boundary", []measurements.SmartTemperature{{Date: old, Temp: 60}, {Date: middle, Temp: 40}, {Date: now, Temp: 60}}, now},
		{"unknown ignored", []measurements.SmartTemperature{{Date: old, Temp: 60}, {Date: middle, Temp: 0}, {Date: now, Temp: 55}}, old},
		{"negative ignored", []measurements.SmartTemperature{{Date: old, Temp: 60}, {Date: now, Temp: -1}}, old},
		{"unknown only", []measurements.SmartTemperature{{Date: old, Temp: 0}}, now},
		{"future ignored", []measurements.SmartTemperature{{Date: now.Add(time.Hour), Temp: 60}}, now},
		{"invalid date", []measurements.SmartTemperature{{Temp: 60}}, now},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := append([]measurements.SmartTemperature(nil), tc.history...)
			require.Equal(t, tc.want, ExceededSince(tc.history, testTemperatureThreshold, now))
			require.Equal(t, original, tc.history, "must not reorder caller history")
		})
	}
}

func TestNewTemperatureNotify(t *testing.T) {
	cfg, err := config.Create()
	require.NoError(t, err)
	device := &models.Device{DeviceName: "/dev/sda", HostId: "nas", Label: "<hot>"}
	now := time.Now()
	for _, unit := range []string{"celsius", "fahrenheit", ""} {
		n := NewTemperatureNotify(logrus.New(), cfg, device, 60, testTemperatureThreshold, now.Add(-testTemperatureDuration), now, unit)
		require.Equal(t, NotifyFailureTypeTemperature, n.Payload.FailureType)
		require.Contains(t, n.Payload.Subject, "30m")
		require.Contains(t, n.Payload.Subject, "nas")
		require.Contains(t, n.Payload.Message, "/dev/sda")
		require.Contains(t, n.Payload.HTMLMessage, "HIGH TEMPERATURE")
		require.Contains(t, n.Payload.HTMLMessage, "&lt;hot&gt;")
		if unit == "fahrenheit" {
			require.Contains(t, n.Payload.Subject, "140°F >= 131°F")
		} else {
			require.Contains(t, n.Payload.Subject, "60°C >= 55°C")
		}
	}
}

func TestGateSharesTemperatureTracker(t *testing.T) {
	gate := NewNotificationGate(logrus.New())
	require.NotNil(t, gate.Temperature())
	require.Same(t, gate.Temperature(), gate.Temperature())
}

func TestTemperatureSubjectDeviceFormat(t *testing.T) {
	cfg, err := config.Create()
	require.NoError(t, err)
	now := time.Now()
	for _, tc := range []struct {
		label, host, suffix string
	}{
		{"", "", "device: /dev/sda"},
		{"Backup", "", "device: Backup (/dev/sda)"},
		{"", "nas", "device: /dev/sda (host: nas)"},
		{" Backup ", " nas ", "device: Backup (/dev/sda) (host: nas)"},
		{" ", " ", "device: /dev/sda"},
	} {
		t.Run(tc.suffix, func(t *testing.T) {
			device := &models.Device{DeviceName: "/dev/sda", Label: tc.label, HostId: tc.host}
			n := NewTemperatureNotify(logrus.New(), cfg, device, 60, testTemperatureThreshold, now, now, "celsius")
			require.Equal(t, "Scrutiny temperature alert (60°C >= 55°C for 0m) on "+tc.suffix, n.Payload.Subject)
		})
	}
}

func TestTemperaturePayloadPrecisionAndMissingLabels(t *testing.T) {
	require.Equal(t, "107.6°F", formatTemperature(42, "fahrenheit"))
	cfg, err := config.Create()
	require.NoError(t, err)
	now := time.Now()
	n := NewTemperatureNotify(logrus.New(), cfg, &models.Device{DeviceName: "/dev/sda"}, 60, testTemperatureThreshold, now.Add(time.Hour), now, "celsius")
	require.Contains(t, n.Payload.Subject, "for 0m")
	require.Contains(t, n.Payload.Subject, "on device: /dev/sda")
}

func TestTemperatureElapsedFormatting(t *testing.T) {
	cfg, err := config.Create()
	require.NoError(t, err)
	now := time.Now()
	for _, tc := range []struct {
		elapsed time.Duration
		want    string
	}{
		{-time.Minute, "0m"}, {0, "0m"}, {time.Second, "0m"},
		{time.Minute, "1m"}, {testTemperatureDuration + time.Second, "30m"},
		{time.Hour, "1h0m"}, {time.Hour + testTemperatureDuration, "1h30m"},
	} {
		t.Run(tc.elapsed.String(), func(t *testing.T) {
			n := NewTemperatureNotify(logrus.New(), cfg, &models.Device{}, testTemperatureThreshold, testTemperatureThreshold, now.Add(-tc.elapsed), now, "celsius")
			require.Contains(t, n.Payload.Subject, "for "+tc.want+")")
		})
	}
}
