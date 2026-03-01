package notify

import (
	"fmt"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestParseTimeOfDay_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int
	}{
		{"00:00", 0},
		{"07:30", 450},
		{"12:00", 720},
		{"22:00", 1320},
		{"23:59", 1439},
	}

	for _, tc := range tests {
		result, err := parseTimeOfDay(tc.input)
		require.NoError(t, err, "input: %s", tc.input)
		require.Equal(t, tc.expected, result, "input: %s", tc.input)
	}
}

func TestParseTimeOfDay_Invalid(t *testing.T) {
	t.Parallel()

	inputs := []string{"", "abc", "25:00", "12:60", "12", "-1:00"}
	for _, input := range inputs {
		_, err := parseTimeOfDay(input)
		require.Error(t, err, "input: %s", input)
	}
}

func TestGate_NoLimits_SendsImmediately(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))
	settings := &models.Settings{}
	settings.Metrics.NotificationRateLimit = 0
	settings.Metrics.NotificationQuietStart = ""
	settings.Metrics.NotificationQuietEnd = ""

	// Verify isQuietHours returns false when not configured
	require.False(t, gate.isQuietHours(settings))

	// Verify isRateLimited returns false when limit is 0
	require.False(t, gate.isRateLimited(settings))
}

func TestGate_RateLimit_DropsWhenExceeded(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))
	settings := &models.Settings{}
	settings.Metrics.NotificationRateLimit = 2

	// Simulate 2 sent notifications
	gate.mu.Lock()
	gate.sentTimestamps = []time.Time{
		time.Now().Add(-30 * time.Minute),
		time.Now().Add(-10 * time.Minute),
	}
	gate.mu.Unlock()

	require.True(t, gate.isRateLimited(settings))
}

func TestGate_RateLimit_SlidingWindowExpiry(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))
	settings := &models.Settings{}
	settings.Metrics.NotificationRateLimit = 2

	// Simulate 2 sent notifications, both older than 1 hour
	gate.mu.Lock()
	gate.sentTimestamps = []time.Time{
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-90 * time.Minute),
	}
	gate.mu.Unlock()

	// Old timestamps should be pruned, so not rate limited
	require.False(t, gate.isRateLimited(settings))
}

func TestGate_QuietHours_SameDayWindow(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))

	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	// Set quiet hours around the current time (current - 30min to current + 30min)
	startMin := currentMinutes - 30
	if startMin < 0 {
		startMin += 1440
	}
	endMin := currentMinutes + 30
	if endMin >= 1440 {
		endMin -= 1440
	}

	settings := &models.Settings{}
	settings.Metrics.NotificationQuietStart = minutesToHHMM(startMin)
	settings.Metrics.NotificationQuietEnd = minutesToHHMM(endMin)

	// Same-day window only works when start < end
	if startMin < endMin {
		require.True(t, gate.isQuietHours(settings))
	}
}

func TestGate_QuietHours_OvernightSpan(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))

	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	// Overnight window: start after current time + 60 to end before current time - 60
	// This means current time is NOT in quiet hours
	startAfter := currentMinutes + 60
	if startAfter >= 1440 {
		startAfter -= 1440
	}
	endBefore := currentMinutes - 60
	if endBefore < 0 {
		endBefore += 1440
	}

	settings := &models.Settings{}
	settings.Metrics.NotificationQuietStart = minutesToHHMM(startAfter)
	settings.Metrics.NotificationQuietEnd = minutesToHHMM(endBefore)

	// In overnight mode (start > end), quiet if now >= start OR now < end
	// Since we set start = now+60 and end = now-60, now is NOT in quiet
	if startAfter > endBefore {
		require.False(t, gate.isQuietHours(settings))
	}
}

func TestGate_QuietHours_Disabled(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))

	// Empty strings = disabled
	settings := &models.Settings{}
	settings.Metrics.NotificationQuietStart = ""
	settings.Metrics.NotificationQuietEnd = ""
	require.False(t, gate.isQuietHours(settings))

	// Only start set = disabled
	settings.Metrics.NotificationQuietStart = "22:00"
	settings.Metrics.NotificationQuietEnd = ""
	require.False(t, gate.isQuietHours(settings))

	// Only end set = disabled
	settings.Metrics.NotificationQuietStart = ""
	settings.Metrics.NotificationQuietEnd = "07:00"
	require.False(t, gate.isQuietHours(settings))
}

func TestGate_QueueLength(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))
	require.Equal(t, 0, gate.QueueLength())

	gate.mu.Lock()
	gate.quietQueue = append(gate.quietQueue, QueuedNotification{
		Subject:  "Test",
		Message:  "Message",
		QueuedAt: time.Now(),
	})
	gate.mu.Unlock()

	require.Equal(t, 1, gate.QueueLength())
}

func TestGate_PruneOldTimestamps(t *testing.T) {
	t.Parallel()

	gate := NewNotificationGate(logrus.NewEntry(logrus.StandardLogger()))

	gate.mu.Lock()
	gate.sentTimestamps = []time.Time{
		time.Now().Add(-2 * time.Hour),   // should be pruned
		time.Now().Add(-90 * time.Minute), // should be pruned
		time.Now().Add(-30 * time.Minute), // should be kept
		time.Now().Add(-5 * time.Minute),  // should be kept
	}
	gate.pruneOldTimestamps()
	require.Equal(t, 2, len(gate.sentTimestamps))
	gate.mu.Unlock()
}

// minutesToHHMM converts minutes since midnight to "HH:MM" format.
func minutesToHHMM(minutes int) string {
	if minutes < 0 {
		minutes += 1440
	}
	if minutes >= 1440 {
		minutes -= 1440
	}
	h := minutes / 60
	m := minutes % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}
