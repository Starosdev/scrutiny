package models

import "time"

type FilesystemHostStatusValue string

const (
	FilesystemHostStatusAvailable   FilesystemHostStatusValue = "available"
	FilesystemHostStatusUnavailable FilesystemHostStatusValue = "unavailable"
)

type FilesystemHostStatus struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	HostID          string                    `json:"host_id" gorm:"primaryKey"`
	Status          FilesystemHostStatusValue `json:"status"`
	Reason          string                    `json:"reason,omitempty"`
	FilesystemCount int                       `json:"filesystem_count"`
}
