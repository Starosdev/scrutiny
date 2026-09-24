package models

import "time"

// NotifyUrl represents a user-configured notification endpoint stored in the database.
// Only UI-sourced URLs are persisted here. Config/env URLs are read from Viper at runtime.
type NotifyUrl struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url" gorm:"not null"`
	Label     string    `json:"label"`
	Source    string    `json:"source" gorm:"default:'ui'"`
	// No default tag: GORM would replace a false value with the default on create (#888).
	// Callers always set it. SQLite columns keep the database default of true that migration
	// m20260608000000 used to turn heartbeats on for URLs that existed before the setting.
	HeartbeatEnabled bool `json:"heartbeat_enabled"`
	ID               uint `json:"id" gorm:"primaryKey"`
}

func (NotifyUrl) TableName() string {
	return "notify_urls"
}
