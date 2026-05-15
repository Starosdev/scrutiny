package models

import "time"

type FilesystemCapacity struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	HostID         string  `json:"host_id" gorm:"primaryKey"`
	MountPoint     string  `json:"mount_point" gorm:"primaryKey"`
	SourceDevice   string  `json:"source_device"`
	FilesystemType string  `json:"filesystem_type"`
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	AvailableBytes int64   `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type FilesystemSummaryUpload struct {
	Filesystems []FilesystemCapacity   `json:"filesystems"`
	Hosts       []FilesystemHostStatus `json:"hosts"`
}
