package notify

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

// NotificationGate centralizes rate limiting, cooldown, and quiet hours logic.
// All notification dispatch should pass through Gate.TrySend() instead of
// directly calling Notify.Send().
type NotificationGate struct {
	logger         logrus.FieldLogger
	sentTimestamps []time.Time          // sliding window for rate limiting
	quietQueue     []QueuedNotification // queued during quiet hours
	mu             sync.Mutex
}

// QueuedNotification holds a notification that was deferred during quiet hours.
type QueuedNotification struct {
	QueuedAt time.Time
	Subject  string
	Message  string
}

// NewNotificationGate creates a new gate instance. Should be created once
// in AppEngine and shared across all notification paths.
func NewNotificationGate(logger logrus.FieldLogger) *NotificationGate {
	return &NotificationGate{
		logger: logger,
	}
}

// TrySend checks rate limiting and quiet hours before dispatching a notification.
// If quiet hours are active, the notification summary is queued for digest delivery.
// If rate limit is exceeded, the notification is dropped (logged).
// If bypassQuietHours is true, quiet hours are ignored (used for heartbeats).
// Returns true if sent or queued, false if dropped.
func (g *NotificationGate) TrySend(n *Notify, settings *models.Settings, bypassQuietHours bool) bool {
	if !bypassQuietHours && g.isQuietHours(settings) {
		subject := n.Payload.Subject
		message := n.Payload.Message
		g.mu.Lock()
		g.quietQueue = append(g.quietQueue, QueuedNotification{
			Subject:  subject,
			Message:  message,
			QueuedAt: time.Now(),
		})
		g.mu.Unlock()
		g.logger.Infof("Notification queued during quiet hours: %s", subject)
		return true
	}

	if g.isRateLimited(settings) {
		g.logger.Warnf("Notification dropped due to rate limit (%d/hour): %s",
			settings.Metrics.NotificationRateLimit, n.Payload.Subject)
		return false
	}

	if err := n.Send(); err != nil {
		g.logger.Warnf("Failed to send notification: %v", err)
		return false
	}

	g.recordSent()
	return true
}

// FlushQuietQueue checks if quiet hours have ended and sends a digest of all
// queued notifications. Should be called periodically (e.g., at the start of
// each missed ping check cycle).
func (g *NotificationGate) FlushQuietQueue(n *Notify, settings *models.Settings) {
	if g.isQuietHours(settings) {
		return
	}

	g.mu.Lock()
	if len(g.quietQueue) == 0 {
		g.mu.Unlock()
		return
	}
	queued := make([]QueuedNotification, len(g.quietQueue))
	copy(queued, g.quietQueue)
	g.quietQueue = nil
	g.mu.Unlock()

	subject := fmt.Sprintf("Scrutiny: %d notification(s) during quiet hours", len(queued))
	var parts []string
	parts = append(parts,
		fmt.Sprintf("The following %d notification(s) were queued during quiet hours:", len(queued)),
		"",
	)
	for _, q := range queued {
		parts = append(parts, fmt.Sprintf("  [%s] %s", q.QueuedAt.Format("15:04"), q.Subject))
		if q.Message != "" {
			// Include first line of message for context
			lines := strings.SplitN(q.Message, "\n", 2)
			parts = append(parts, fmt.Sprintf("    %s", lines[0]))
		}
		parts = append(parts, "")
	}

	n.Payload = Payload{
		FailureType: NotifyFailureTypeMissedPing,
		Subject:     subject,
		Message:     strings.Join(parts, "\n"),
	}

	if g.isRateLimited(settings) {
		g.logger.Warnf("Quiet hours digest dropped due to rate limit")
		return
	}

	if err := n.Send(); err != nil {
		g.logger.Warnf("Failed to send quiet hours digest: %v", err)
		return
	}
	g.recordSent()
	g.logger.Infof("Sent quiet hours digest with %d queued notification(s)", len(queued))
}

// isQuietHours checks if the current time falls within the configured quiet window.
// Returns false if quiet hours are not configured (empty strings).
func (g *NotificationGate) isQuietHours(settings *models.Settings) bool {
	startStr := settings.Metrics.NotificationQuietStart
	endStr := settings.Metrics.NotificationQuietEnd

	if startStr == "" || endStr == "" {
		return false
	}

	start, err := parseTimeOfDay(startStr)
	if err != nil {
		g.logger.Warnf("Invalid notification_quiet_start '%s': %v", startStr, err)
		return false
	}
	end, err := parseTimeOfDay(endStr)
	if err != nil {
		g.logger.Warnf("Invalid notification_quiet_end '%s': %v", endStr, err)
		return false
	}

	now := time.Now()
	nowMinutes := now.Hour()*60 + now.Minute()

	if start <= end {
		// Same-day window (e.g., 08:00-17:00)
		return nowMinutes >= start && nowMinutes < end
	}
	// Overnight window (e.g., 22:00-07:00)
	return nowMinutes >= start || nowMinutes < end
}

// isRateLimited checks if sending another notification would exceed the hourly limit.
// Returns false if rate limiting is disabled (limit == 0).
func (g *NotificationGate) isRateLimited(settings *models.Settings) bool {
	limit := settings.Metrics.NotificationRateLimit
	if limit <= 0 {
		return false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.pruneOldTimestamps()
	return len(g.sentTimestamps) >= limit
}

// recordSent adds the current time to the sliding window.
func (g *NotificationGate) recordSent() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sentTimestamps = append(g.sentTimestamps, time.Now())
}

// pruneOldTimestamps removes entries older than 1 hour from the sliding window.
// Must be called with g.mu held.
func (g *NotificationGate) pruneOldTimestamps() {
	cutoff := time.Now().Add(-1 * time.Hour)
	n := 0
	for _, ts := range g.sentTimestamps {
		if ts.After(cutoff) {
			g.sentTimestamps[n] = ts
			n++
		}
	}
	g.sentTimestamps = g.sentTimestamps[:n]
}

// parseTimeOfDay parses a "HH:MM" string and returns total minutes since midnight.
func parseTimeOfDay(s string) (int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM format, got %q", s)
	}
	var hour, min int
	if _, err := fmt.Sscanf(parts[0], "%d", &hour); err != nil {
		return 0, fmt.Errorf("invalid hour: %w", err)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &min); err != nil {
		return 0, fmt.Errorf("invalid minute: %w", err)
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, fmt.Errorf("time out of range: %02d:%02d", hour, min)
	}
	return hour*60 + min, nil
}

// QueueLength returns the number of notifications currently queued during quiet hours.
func (g *NotificationGate) QueueLength() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.quietQueue)
}
