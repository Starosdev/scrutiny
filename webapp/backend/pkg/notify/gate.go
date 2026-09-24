package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

// OutboxMaxAge is how long an undelivered notification waits for a leader before it is
// discarded, so an outage does not end in a burst of stale alerts.
const OutboxMaxAge = time.Hour

const outboxBatchSize = 100

// OutboxStore persists notifications for the leader replica. database.DeviceRepo implements it.
type OutboxStore interface {
	EnqueueNotification(ctx context.Context, row *models.NotificationOutbox) error
	ListNotifications(ctx context.Context, state string, limit int) ([]models.NotificationOutbox, error)
	TransitionNotification(ctx context.Context, id uint, from string, to string) (bool, error)
	DeleteNotifications(ctx context.Context, ids []uint) error
	DeleteStaleNotifications(ctx context.Context, states []string, createdBeforeUnixMs int64) (int64, error)
}

// outboxBody is the stored form of a Notify. Logger and Config are not stored; the leader
// supplies its own when it delivers.
type outboxBody struct {
	Payload      Payload  `json:"payload"`
	HTMLMessage  string   `json:"html_message,omitempty"`
	DatabaseUrls []string `json:"database_urls,omitempty"`
}

type deliveryOutcome int

const (
	outcomeSent deliveryOutcome = iota
	outcomeQueued
	outcomeDropped
)

// NotificationGate centralizes rate limiting, cooldown, and quiet hours logic.
// All notification dispatch should pass through Gate.TrySend() instead of
// directly calling Notify.Send().
type NotificationGate struct {
	logger            logrus.FieldLogger
	sentTimestamps    []time.Time          // sliding window for rate limiting
	quietQueue        []QueuedNotification // queued during quiet hours
	collectorError    map[string]time.Time // dedupe map for collector-side errors
	mu                sync.Mutex
	temperature       *TemperatureTracker
	flushMu           sync.Mutex // Serializes digest flushes without blocking notification enqueue.
	noEndpointsWarned bool       // Reset after successful delivery; retries remain enabled.

	// outbox, when set, makes TrySend record notifications for the leader replica instead of
	// sending them. wake signals the local outbox worker that a row was added.
	outbox OutboxStore
	wake   chan struct{}
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
		logger:         logger,
		collectorError: map[string]time.Time{},
		temperature:    NewTemperatureTracker(),
		wake:           make(chan struct{}, 1),
	}
}

// UseOutbox routes TrySend and TrySendCollectorError through the outbox, so the leader replica
// applies rate limits, quiet hours, and duplicate suppression once for the whole deployment.
// Call it before the gate is shared.
func (g *NotificationGate) UseOutbox(store OutboxStore) {
	g.outbox = store
}

// Wake receives a value whenever this process adds a row to the outbox.
func (g *NotificationGate) Wake() <-chan struct{} {
	return g.wake
}

func (g *NotificationGate) Temperature() *TemperatureTracker { return g.temperature }

// TrySend checks rate limiting and quiet hours before dispatching a notification.
// If quiet hours are active, the notification summary is queued for digest delivery.
// If rate limit is exceeded, the notification is dropped (logged).
// If bypassQuietHours is true, quiet hours are ignored (used for heartbeats).
// Returns true if sent or queued, false if dropped.
//
// With an outbox, the notification is recorded for the leader replica and TrySend returns true
// once it is stored; the leader applies the checks above when it delivers.
func (g *NotificationGate) TrySend(n *Notify, settings *models.Settings, bypassQuietHours bool) bool {
	if g.outbox != nil && g.enqueue(n, bypassQuietHours, "") {
		return true
	}
	return g.settle(n, g.deliver(n, settings, bypassQuietHours))
}

func (g *NotificationGate) TrySendCollectorError(identity, errorType, errorMessage string, n *Notify, settings *models.Settings) bool {
	key := collectorErrorKey(identity, errorType, errorMessage)
	if g.outbox != nil && g.enqueue(n, false, key) {
		return true
	}
	return g.settle(n, g.deliverCollectorError(key, n, settings))
}

// deliver applies quiet hours and the rate limit, then sends. It never queues; the caller
// decides where a quiet-hours notification waits.
func (g *NotificationGate) deliver(n *Notify, settings *models.Settings, bypassQuietHours bool) deliveryOutcome {
	if !bypassQuietHours && g.isQuietHours(settings) {
		return outcomeQueued
	}

	if g.isRateLimited(settings) {
		g.logger.Warnf("Notification dropped due to rate limit (%d/hour): %s",
			settings.Metrics.NotificationRateLimit, n.Payload.Subject)
		return outcomeDropped
	}

	if err := n.Send(); err != nil {
		g.logSendError(n.Payload.Subject, err)
		return outcomeDropped
	}

	g.recordSent()
	return outcomeSent
}

// deliverCollectorError sends a collector error once per key unless repeat notifications are on.
func (g *NotificationGate) deliverCollectorError(key string, n *Notify, settings *models.Settings) deliveryOutcome {
	if settings.Metrics.RepeatNotifications {
		return g.deliver(n, settings, false)
	}

	g.mu.Lock()
	if _, exists := g.collectorError[key]; exists {
		g.mu.Unlock()
		g.logger.Infof("Skipping duplicate collector error notification: %s", n.Payload.Subject)
		return outcomeDropped
	}
	g.collectorError[key] = time.Now()
	g.mu.Unlock()

	outcome := g.deliver(n, settings, false)
	if outcome == outcomeDropped {
		g.mu.Lock()
		delete(g.collectorError, key)
		g.mu.Unlock()
	}
	return outcome
}

// settle holds a quiet-hours notification in memory and reports whether it was sent or queued.
func (g *NotificationGate) settle(n *Notify, outcome deliveryOutcome) bool {
	if outcome == outcomeQueued {
		g.mu.Lock()
		g.quietQueue = append(g.quietQueue, QueuedNotification{
			Subject:  n.Payload.Subject,
			Message:  n.Payload.Message,
			QueuedAt: time.Now(),
		})
		g.mu.Unlock()
		g.logger.Infof("Notification queued during quiet hours: %s", n.Payload.Subject)
	}
	return outcome != outcomeDropped
}

// enqueue records n in the outbox. It returns false when the row cannot be stored, and the
// caller then delivers directly rather than losing the notification.
func (g *NotificationGate) enqueue(n *Notify, bypassQuietHours bool, dedupeKey string) bool {
	body, err := json.Marshal(outboxBody{
		Payload:      n.Payload,
		HTMLMessage:  n.Payload.HTMLMessage,
		DatabaseUrls: n.DatabaseUrls,
	})
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = g.outbox.EnqueueNotification(ctx, &models.NotificationOutbox{
			CreatedAtUnixMs:  time.Now().UnixMilli(),
			State:            models.NotificationPending,
			Body:             string(body),
			BypassQuietHours: bypassQuietHours,
			DedupeKey:        dedupeKey,
		})
	}
	if err != nil {
		g.logger.Warnf("Failed to store notification %q in the outbox, sending directly: %v", n.Payload.Subject, err)
		return false
	}
	select {
	case g.wake <- struct{}{}:
	default:
	}
	return true
}

// DrainOutbox delivers pending outbox rows. Only the leader replica calls it. Each row is claimed
// before it is sent, so a row is delivered at most once even if two replicas briefly both lead.
// Rows that arrive during quiet hours move to the queued state and wait for FlushQuietQueue.
func (g *NotificationGate) DrainOutbox(ctx context.Context, appConfig config.Interface, settings *models.Settings) error {
	cutoff := time.Now().Add(-OutboxMaxAge).UnixMilli()
	stale, err := g.outbox.DeleteStaleNotifications(ctx,
		[]string{models.NotificationPending, models.NotificationClaimed}, cutoff)
	if err != nil {
		return err
	}
	if stale > 0 {
		g.logger.Warnf("Discarded %d notification(s) that waited more than %s for delivery", stale, OutboxMaxAge)
	}

	rows, err := g.outbox.ListNotifications(ctx, models.NotificationPending, outboxBatchSize)
	if err != nil {
		return err
	}
	for _, row := range rows {
		claimed, err := g.outbox.TransitionNotification(ctx, row.ID, models.NotificationPending, models.NotificationClaimed)
		if err != nil {
			return err
		}
		if !claimed {
			continue
		}

		n, err := g.decodeOutboxRow(row, appConfig)
		if err != nil {
			g.logger.Warnf("Discarding unreadable outbox notification %d: %v", row.ID, err)
			if err := g.outbox.DeleteNotifications(ctx, []uint{row.ID}); err != nil {
				return err
			}
			continue
		}

		var outcome deliveryOutcome
		if row.DedupeKey != "" {
			outcome = g.deliverCollectorError(row.DedupeKey, n, settings)
		} else {
			outcome = g.deliver(n, settings, row.BypassQuietHours)
		}

		if outcome == outcomeQueued {
			if _, err := g.outbox.TransitionNotification(ctx, row.ID, models.NotificationClaimed, models.NotificationQueued); err != nil {
				return err
			}
			g.logger.Infof("Notification queued during quiet hours: %s", n.Payload.Subject)
			continue
		}
		if err := g.outbox.DeleteNotifications(ctx, []uint{row.ID}); err != nil {
			return err
		}
	}
	return nil
}

func (g *NotificationGate) decodeOutboxRow(row models.NotificationOutbox, appConfig config.Interface) (*Notify, error) {
	var body outboxBody
	if err := json.Unmarshal([]byte(row.Body), &body); err != nil {
		return nil, err
	}
	body.Payload.HTMLMessage = body.HTMLMessage
	return &Notify{
		Logger:       g.logger,
		Config:       appConfig,
		Payload:      body.Payload,
		DatabaseUrls: body.DatabaseUrls,
	}, nil
}

func (g *NotificationGate) ClearCollectorErrorState(identity string) {
	prefix := strings.ToLower(strings.TrimSpace(identity)) + "|"
	if prefix == "|" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	for key := range g.collectorError {
		if strings.HasPrefix(key, prefix) {
			delete(g.collectorError, key)
		}
	}
}

// FlushQuietQueue checks if quiet hours have ended and sends a digest of all
// queued notifications. Failed or rate-limited digests remain queued for the
// next periodic check, regardless of whether missed ping alerts are enabled.
func (g *NotificationGate) FlushQuietQueue(n *Notify, settings *models.Settings) {
	g.flushMu.Lock()
	defer g.flushMu.Unlock()

	if g.isQuietHours(settings) {
		return
	}
	if g.outbox != nil {
		g.flushOutboxQueue(n, settings)
		return
	}

	g.mu.Lock()
	if len(g.quietQueue) == 0 {
		g.mu.Unlock()
		return
	}
	queued := make([]QueuedNotification, len(g.quietQueue))
	copy(queued, g.quietQueue)
	g.mu.Unlock()

	n.Payload = quietDigestPayload(queued)

	if g.isRateLimited(settings) {
		g.logger.Warnf("Quiet hours digest deferred due to rate limit")
		return
	}

	if err := n.Send(); err != nil {
		g.logSendError(n.Payload.Subject, err)
		return
	}
	// Only remove the delivered batch; new notifications may have been queued
	// while Send was running. flushMu prevents another flush removing it first.
	g.mu.Lock()
	g.quietQueue = append([]QueuedNotification(nil), g.quietQueue[len(queued):]...)
	g.mu.Unlock()
	g.recordSent()
	g.logger.Infof("Sent quiet hours digest with %d queued notification(s)", len(queued))
}

// flushOutboxQueue sends the digest from queued outbox rows. Rows are claimed first, so two
// replicas cannot send the same digest, and are returned to the queue if the digest fails.
func (g *NotificationGate) flushOutboxQueue(n *Notify, settings *models.Settings) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := g.outbox.ListNotifications(ctx, models.NotificationQueued, 1000)
	if err != nil {
		g.logger.Warnf("Failed to read queued notifications: %v", err)
		return
	}

	var ids []uint
	var queued []QueuedNotification
	for _, row := range rows {
		claimed, err := g.outbox.TransitionNotification(ctx, row.ID, models.NotificationQueued, models.NotificationDigesting)
		if err != nil {
			g.logger.Warnf("Failed to claim queued notification %d: %v", row.ID, err)
			continue
		}
		if !claimed {
			continue
		}
		ids = append(ids, row.ID)
		entry := QueuedNotification{QueuedAt: time.UnixMilli(row.CreatedAtUnixMs)}
		var body outboxBody
		if err := json.Unmarshal([]byte(row.Body), &body); err == nil {
			entry.Subject = body.Payload.Subject
			entry.Message = body.Payload.Message
		}
		queued = append(queued, entry)
	}
	if len(ids) == 0 {
		return
	}

	requeue := func() {
		for _, id := range ids {
			if _, err := g.outbox.TransitionNotification(ctx, id, models.NotificationDigesting, models.NotificationQueued); err != nil {
				g.logger.Warnf("Failed to return notification %d to the quiet hours queue: %v", id, err)
			}
		}
	}

	n.Payload = quietDigestPayload(queued)

	if g.isRateLimited(settings) {
		g.logger.Warnf("Quiet hours digest deferred due to rate limit")
		requeue()
		return
	}
	if err := n.Send(); err != nil {
		g.logSendError(n.Payload.Subject, err)
		requeue()
		return
	}
	g.recordSent()
	if err := g.outbox.DeleteNotifications(ctx, ids); err != nil {
		g.logger.Warnf("Failed to remove delivered digest notifications: %v", err)
	}
	g.logger.Infof("Sent quiet hours digest with %d queued notification(s)", len(queued))
}

func quietDigestPayload(queued []QueuedNotification) Payload {
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

	return Payload{
		FailureType: NotifyFailureTypeMissedPing,
		Subject:     subject,
		Message:     strings.Join(parts, "\n"),
	}
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
	g.noEndpointsWarned = false
}

// logSendError warns once about missing endpoints across all devices and digests.
// Other delivery failures are always logged, and no retry state is changed.
func (g *NotificationGate) logSendError(subject string, err error) {
	if errors.Is(err, errNoNotificationEndpoints) {
		g.mu.Lock()
		alreadyWarned := g.noEndpointsWarned
		g.noEndpointsWarned = true
		g.mu.Unlock()
		if alreadyWarned {
			return
		}
	}
	g.logger.Warnf("Failed to send notification %q: %v", subject, err)
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
	if g.outbox != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rows, err := g.outbox.ListNotifications(ctx, models.NotificationQueued, 1000)
		if err != nil {
			g.logger.Warnf("Failed to count queued notifications: %v", err)
			return 0
		}
		return len(rows)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.quietQueue)
}

func collectorErrorKey(identity, errorType, errorMessage string) string {
	return strings.ToLower(strings.TrimSpace(identity)) + "|" +
		strings.ToLower(strings.TrimSpace(errorType)) + "|" +
		strings.ToLower(strings.TrimSpace(errorMessage))
}
