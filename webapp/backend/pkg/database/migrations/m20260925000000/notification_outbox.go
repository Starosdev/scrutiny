package m20260925000000

import "gorm.io/gorm"

// NotificationOutbox is frozen at the schema this migration creates.
type NotificationOutbox struct {
	ID               uint   `gorm:"primaryKey"`
	CreatedAtUnixMs  int64  `gorm:"not null;index"`
	State            string `gorm:"not null;index"`
	Body             string `gorm:"not null"`
	BypassQuietHours bool   `gorm:"not null;default:false"`
	DedupeKey        string
}

func (NotificationOutbox) TableName() string {
	return "notification_outbox"
}

// Migrate adds the notification_outbox table, which lets any replica record a notification
// for the leader replica to deliver (#880).
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&NotificationOutbox{})
}
