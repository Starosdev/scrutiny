package btrfs

import "time"

type ScrubState string

const (
	ScrubStateUnknown  ScrubState = "unknown"
	ScrubStateIdle     ScrubState = "idle"
	ScrubStateRunning  ScrubState = "running"
	ScrubStateFinished ScrubState = "finished"
	ScrubStateAborted  ScrubState = "aborted"
)

type FilesystemStatus string

const (
	FilesystemStatusOnline   FilesystemStatus = "ONLINE"
	FilesystemStatusDegraded FilesystemStatus = "DEGRADED"
)

type FilesystemWrapper struct {
	Data    []Filesystem `json:"data"`
	Errors  []error      `json:"errors,omitempty"`
	Success bool         `json:"success"`
}

//nolint:govet // Keep the Btrfs payload fields grouped for stable JSON contract readability.
type Filesystem struct {
	UUID               string           `json:"uuid"`
	Label              string           `json:"label,omitempty"`
	HostID             string           `json:"host_id"`
	MountPoint         string           `json:"mount_point,omitempty"`
	DataProfile        string           `json:"data_profile,omitempty"`
	MetadataProfile    string           `json:"metadata_profile,omitempty"`
	SystemProfile      string           `json:"system_profile,omitempty"`
	ScrubDuration      string           `json:"scrub_duration,omitempty"`
	ScrubErrorSummary  string           `json:"scrub_error_summary,omitempty"`
	ScrubStartedAt     *time.Time       `json:"scrub_started_at,omitempty"`
	ScrubFinishedAt    *time.Time       `json:"scrub_finished_at,omitempty"`
	Devices            []Device         `json:"devices,omitempty"`
	DeviceSize         int64            `json:"device_size"`
	DeviceAllocated    int64            `json:"device_allocated"`
	DeviceUnallocated  int64            `json:"device_unallocated"`
	DeviceMissing      int64            `json:"device_missing"`
	Used               int64            `json:"used"`
	FreeEstimated      int64            `json:"free_estimated"`
	FreeMin            int64            `json:"free_min"`
	FreeStatfs         int64            `json:"free_statfs"`
	DataRatio          float64          `json:"data_ratio"`
	MetadataRatio      float64          `json:"metadata_ratio"`
	DataTotal          int64            `json:"data_total"`
	DataUsed           int64            `json:"data_used"`
	MetadataTotal      int64            `json:"metadata_total"`
	MetadataUsed       int64            `json:"metadata_used"`
	SystemTotal        int64            `json:"system_total"`
	SystemUsed         int64            `json:"system_used"`
	ScrubTotalBytes    int64            `json:"scrub_total_bytes"`
	ScrubScrubbedBytes int64            `json:"scrub_scrubbed_bytes"`
	ScrubReadErrors    int64            `json:"scrub_read_errors"`
	ScrubCsumErrors    int64            `json:"scrub_csum_errors"`
	ScrubVerifyErrors  int64            `json:"scrub_verify_errors"`
	ScrubSuperErrors   int64            `json:"scrub_super_errors"`
	Status             FilesystemStatus `json:"status"`
	ScrubState         ScrubState       `json:"scrub_state"`
	DeviceCount        int              `json:"device_count"`
	MultipleProfiles   bool             `json:"multiple_profiles"`
}

type Device struct {
	Path             string `json:"path,omitempty"`
	ReadIOErrors     int64  `json:"read_io_errors"`
	WriteIOErrors    int64  `json:"write_io_errors"`
	FlushIOErrors    int64  `json:"flush_io_errors"`
	CorruptionErrors int64  `json:"corruption_errors"`
	GenerationErrors int64  `json:"generation_errors"`
	Size             int64  `json:"size"`
	ID               int    `json:"id"`
	Missing          bool   `json:"missing"`
}
