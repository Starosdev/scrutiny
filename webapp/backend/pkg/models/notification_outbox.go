package models

// Notification outbox row states.
const (
	// NotificationPending rows wait for the leader to deliver them.
	NotificationPending = "pending"
	// NotificationClaimed rows are being delivered by one leader.
	NotificationClaimed = "claimed"
	// NotificationQueued rows arrived during quiet hours and wait for the digest.
	NotificationQueued = "queued"
	// NotificationDigesting rows are in a digest that one leader is sending.
	NotificationDigesting = "digesting"
)

// NotificationOutbox is a notification waiting for the leader replica to deliver it (#880).
// Any replica can write a row; only the replica holding the scheduler lease reads and sends,
// so rate limits, quiet hours, and duplicate suppression apply once for the whole deployment.
type NotificationOutbox struct {
	ID              uint   `gorm:"primaryKey"`
	CreatedAtUnixMs int64  `gorm:"not null;index"`
	State           string `gorm:"not null;index"`
	// Body is the JSON-encoded notification (payload, HTML body, and database URLs).
	Body             string `gorm:"not null"`
	BypassQuietHours bool   `gorm:"not null;default:false"`
	// DedupeKey is set for collector errors, which are sent once per identity, type, and message.
	DedupeKey string
}

func (NotificationOutbox) TableName() string {
	return "notification_outbox"
}
