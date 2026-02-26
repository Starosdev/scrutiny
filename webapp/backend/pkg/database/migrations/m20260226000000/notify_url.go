package m20260226000000

import "time"

type NotifyUrl struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	URL       string `gorm:"not null"`
	Label     string
	Source    string `gorm:"default:'ui'"`
	ID        uint   `gorm:"primaryKey"`
}

func (NotifyUrl) TableName() string {
	return "notify_urls"
}
