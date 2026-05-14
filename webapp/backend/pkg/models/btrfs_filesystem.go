package models

import "time"

type BtrfsScrubState string

const (
	BtrfsScrubStateUnknown  BtrfsScrubState = "unknown"
	BtrfsScrubStateIdle     BtrfsScrubState = "idle"
	BtrfsScrubStateRunning  BtrfsScrubState = "running"
	BtrfsScrubStateFinished BtrfsScrubState = "finished"
	BtrfsScrubStateAborted  BtrfsScrubState = "aborted"
)

type BtrfsFilesystemStatus string

const (
	BtrfsFilesystemStatusOnline   BtrfsFilesystemStatus = "ONLINE"
	BtrfsFilesystemStatusDegraded BtrfsFilesystemStatus = "DEGRADED"
)

type BtrfsFilesystemWrapper struct {
	Errors  []error           `json:"errors,omitempty"`
	Data    []BtrfsFilesystem `json:"data"`
	Success bool              `json:"success"`
}

type BtrfsFilesystem struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	UUID       string                `json:"uuid" gorm:"primaryKey"`
	HostID     string                `json:"host_id"`
	Label      string                `json:"label,omitempty"`
	Status     BtrfsFilesystemStatus `json:"status"`
	MountPoint string                `json:"mount_point,omitempty"`

	DeviceCount       int     `json:"device_count"`
	DeviceSize        int64   `json:"device_size"`
	DeviceAllocated   int64   `json:"device_allocated"`
	DeviceUnallocated int64   `json:"device_unallocated"`
	DeviceMissing     int64   `json:"device_missing"`
	Used              int64   `json:"used"`
	FreeEstimated     int64   `json:"free_estimated"`
	FreeMin           int64   `json:"free_min"`
	FreeStatfs        int64   `json:"free_statfs"`
	DataRatio         float64 `json:"data_ratio"`
	MetadataRatio     float64 `json:"metadata_ratio"`
	MultipleProfiles  bool    `json:"multiple_profiles"`

	DataProfile     string `json:"data_profile,omitempty"`
	MetadataProfile string `json:"metadata_profile,omitempty"`
	SystemProfile   string `json:"system_profile,omitempty"`
	DataTotal       int64  `json:"data_total"`
	DataUsed        int64  `json:"data_used"`
	MetadataTotal   int64  `json:"metadata_total"`
	MetadataUsed    int64  `json:"metadata_used"`
	SystemTotal     int64  `json:"system_total"`
	SystemUsed      int64  `json:"system_used"`

	ScrubState         BtrfsScrubState `json:"scrub_state"`
	ScrubStartedAt     *time.Time      `json:"scrub_started_at,omitempty"`
	ScrubFinishedAt    *time.Time      `json:"scrub_finished_at,omitempty"`
	ScrubDuration      string          `json:"scrub_duration,omitempty"`
	ScrubTotalBytes    int64           `json:"scrub_total_bytes"`
	ScrubScrubbedBytes int64           `json:"scrub_scrubbed_bytes"`
	ScrubErrorSummary  string          `json:"scrub_error_summary,omitempty"`
	ScrubReadErrors    int64           `json:"scrub_read_errors"`
	ScrubCsumErrors    int64           `json:"scrub_csum_errors"`
	ScrubVerifyErrors  int64           `json:"scrub_verify_errors"`
	ScrubSuperErrors   int64           `json:"scrub_super_errors"`

	Archived bool          `json:"archived"`
	Muted    bool          `json:"muted"`
	Devices  []BtrfsDevice `json:"devices,omitempty" gorm:"foreignKey:FilesystemUUID;references:UUID"`
}

func (f *BtrfsFilesystem) IsHealthy() bool {
	return f.Status == BtrfsFilesystemStatusOnline
}

func (f *BtrfsFilesystem) HasErrors() bool {
	return f.ScrubReadErrors > 0 || f.ScrubCsumErrors > 0 || f.ScrubVerifyErrors > 0 || f.ScrubSuperErrors > 0
}
