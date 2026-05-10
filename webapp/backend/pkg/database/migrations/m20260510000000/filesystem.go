package m20260510000000

import "time"

type FilesystemCapacity struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	HostID         string `gorm:"primaryKey"`
	MountPoint     string `gorm:"primaryKey"`
	SourceDevice   string
	FilesystemType string
	TotalBytes     int64
	UsedBytes      int64
	AvailableBytes int64
	UsedPercent    float64
}

type FilesystemHostStatus struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	HostID          string `gorm:"primaryKey"`
	Status          string
	Reason          string
	FilesystemCount int
}
