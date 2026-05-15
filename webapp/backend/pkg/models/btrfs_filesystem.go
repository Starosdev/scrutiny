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
	Data    []BtrfsFilesystem `json:"data"`
	Errors  []error           `json:"errors,omitempty"`
	Success bool              `json:"success"`
}

//nolint:govet // Keep the API/DB Btrfs model grouped by payload semantics instead of field packing.
type BtrfsFilesystem struct {
	UUID               string                `json:"uuid" gorm:"primaryKey"`
	HostID             string                `json:"host_id"`
	Label              string                `json:"label,omitempty"`
	MountPoint         string                `json:"mount_point,omitempty"`
	DataProfile        string                `json:"data_profile,omitempty"`
	MetadataProfile    string                `json:"metadata_profile,omitempty"`
	SystemProfile      string                `json:"system_profile,omitempty"`
	ScrubDuration      string                `json:"scrub_duration,omitempty"`
	ScrubErrorSummary  string                `json:"scrub_error_summary,omitempty"`
	DeletedAt          *time.Time            `json:"deleted_at,omitempty"`
	ScrubStartedAt     *time.Time            `json:"scrub_started_at,omitempty"`
	ScrubFinishedAt    *time.Time            `json:"scrub_finished_at,omitempty"`
	Devices            []BtrfsDevice         `json:"devices,omitempty" gorm:"foreignKey:FilesystemUUID;references:UUID"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
	DeviceSize         int64                 `json:"device_size"`
	DeviceAllocated    int64                 `json:"device_allocated"`
	DeviceUnallocated  int64                 `json:"device_unallocated"`
	DeviceMissing      int64                 `json:"device_missing"`
	Used               int64                 `json:"used"`
	FreeEstimated      int64                 `json:"free_estimated"`
	FreeMin            int64                 `json:"free_min"`
	FreeStatfs         int64                 `json:"free_statfs"`
	DataRatio          float64               `json:"data_ratio"`
	MetadataRatio      float64               `json:"metadata_ratio"`
	DataTotal          int64                 `json:"data_total"`
	DataUsed           int64                 `json:"data_used"`
	MetadataTotal      int64                 `json:"metadata_total"`
	MetadataUsed       int64                 `json:"metadata_used"`
	SystemTotal        int64                 `json:"system_total"`
	SystemUsed         int64                 `json:"system_used"`
	ScrubTotalBytes    int64                 `json:"scrub_total_bytes"`
	ScrubScrubbedBytes int64                 `json:"scrub_scrubbed_bytes"`
	ScrubReadErrors    int64                 `json:"scrub_read_errors"`
	ScrubCsumErrors    int64                 `json:"scrub_csum_errors"`
	ScrubVerifyErrors  int64                 `json:"scrub_verify_errors"`
	ScrubSuperErrors   int64                 `json:"scrub_super_errors"`
	Status             BtrfsFilesystemStatus `json:"status"`
	ScrubState         BtrfsScrubState       `json:"scrub_state"`
	DeviceCount        int                   `json:"device_count"`
	MultipleProfiles   bool                  `json:"multiple_profiles"`
	Archived           bool                  `json:"archived"`
	Muted              bool                  `json:"muted"`
}

func (f *BtrfsFilesystem) IsHealthy() bool {
	return f.Status == BtrfsFilesystemStatusOnline
}

func (f *BtrfsFilesystem) HasErrors() bool {
	return f.ScrubReadErrors > 0 || f.ScrubCsumErrors > 0 || f.ScrubVerifyErrors > 0 || f.ScrubSuperErrors > 0
}
