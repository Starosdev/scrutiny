package m20260226000000

import "time"

type NotifyUrl struct {
	ID        uint      `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	URL       string `gorm:"not null"`
	Label     string
	Source    string `gorm:"default:'ui'"`
}

func (NotifyUrl) TableName() string {
	return "notify_urls"
}
