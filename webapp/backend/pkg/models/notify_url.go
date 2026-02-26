package models

import "time"

// NotifyUrl represents a user-configured notification endpoint stored in the database.
// Only UI-sourced URLs are persisted here. Config/env URLs are read from Viper at runtime.
type NotifyUrl struct {
	// The full Shoutrrr-compatible URL (e.g. "smtp://...", "discord://...", "https://...")
	URL string `json:"url" gorm:"not null"`

	// Optional user-friendly name for display
	Label string `json:"label"`

	// Source is always "ui" for database-stored entries; read-only display values
	// ("config", "env") are synthesized at query time and never stored.
	Source string `json:"source" gorm:"default:'ui'"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ID        uint      `json:"id" gorm:"primaryKey"`
}

func (NotifyUrl) TableName() string {
	return "notify_urls"
}
