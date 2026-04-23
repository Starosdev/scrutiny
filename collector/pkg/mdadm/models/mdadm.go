package models

import (
	"time"
)

// MDADMArray represents a discovered MDADM RAID array
type MDADMArray struct {
	UUID    string   `json:"uuid"`
	Name    string   `json:"name"`
	Level   string   `json:"level"`
	Devices []string `json:"devices,omitempty"`
}

// MDADMMetrics represents the time-series status of an MDADM array
type MDADMMetrics struct {
	State          string  `json:"state"`
	ActiveDevices  int     `json:"active_devices"`
	WorkingDevices int     `json:"working_devices"`
	FailedDevices  int     `json:"failed_devices"`
	SpareDevices   int     `json:"spare_devices"`
	SyncProgress   float64   `json:"sync_progress,omitempty"`
	RawMdstat      string    `json:"raw_mdstat,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Storage sizes in bytes (parsed from mdadm --detail: Array Size / Used Dev Size * 1024)
	ArraySize   int64 `json:"array_size,omitempty"`
	UsedDevSize int64 `json:"used_dev_size,omitempty"`
}

// MDADMArrayWrapper wraps the response for MDADM array API calls
type MDADMArrayWrapper struct {
	Success bool         `json:"success"`
	Errors  []error      `json:"errors,omitempty"`
	Data    []MDADMArray `json:"data"`
}
