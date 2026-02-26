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
	ID        uint      `json:"id" gorm:"primaryKey"`
}

func (NotifyUrl) TableName() string {
	return "notify_urls"
}
