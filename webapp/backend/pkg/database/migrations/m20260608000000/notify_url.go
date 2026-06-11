package m20260608000000

import "time"

type NotifyUrl struct {
	CreatedAt        time.Time
	UpdatedAt        time.Time
	URL              string `gorm:"not null"`
	Label            string
	Source           string `gorm:"default:'ui'"`
	HeartbeatEnabled bool   `gorm:"default:true"`
	ID               uint   `gorm:"primaryKey"`
}

func (NotifyUrl) TableName() string {
	return "notify_urls"
}
