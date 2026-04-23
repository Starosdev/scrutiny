package collector

import "time"

// MDADMArray represents a discovered MDADM RAID array from the collector
type MDADMArray struct {
	UUID    string   `json:"uuid"`
	Name    string   `json:"name"`
	Level   string   `json:"level"`
	Devices []string `json:"devices,omitempty"`
}

// MDADMMetrics represents the status of an MDADM array from the collector
type MDADMMetrics struct {
	State          string  `json:"state"`
	ActiveDevices  int     `json:"active_devices"`
	WorkingDevices int     `json:"working_devices"`
	FailedDevices  int     `json:"failed_devices"`
	SpareDevices   int     `json:"spare_devices"`
	SyncProgress   float64   `json:"sync_progress,omitempty"`
	RawMdstat      string    `json:"raw_mdstat,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Storage sizes in bytes (KiB from mdadm --detail, converted to bytes by collector)
	ArraySize   int64 `json:"array_size,omitempty"`
	UsedDevSize int64 `json:"used_dev_size,omitempty"`
}
