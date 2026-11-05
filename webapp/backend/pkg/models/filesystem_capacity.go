package models

import "time"

type FilesystemCapacity struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	HostID         string `json:"host_id" gorm:"primaryKey"`
	MountPoint     string `json:"mount_point" gorm:"primaryKey"`
	SourceDevice   string `json:"source_device"`
	FilesystemType string `json:"filesystem_type"`

	// BackingDevices names the whole disks this filesystem sits on, as the
	// collector resolved them from the kernel's holder tree. It is stored in
	// filesystem_devices rather than in a column here, because the relation is
	// many-to-many: one disk carries several filesystems, and one filesystem
	// (LVM, md) spans several disks.
	//
	// Empty means unattributed, not unbacked. A collector older than this
	// field, a non-Linux host, and a container without the host's sysfs all
	// report filesystems with no backing devices, and a drive's free space is
	// unknown in each case rather than zero.
	BackingDevices []string `json:"backing_devices,omitempty" gorm:"-"`

	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	AvailableBytes int64   `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type FilesystemSummaryUpload struct {
	Filesystems []FilesystemCapacity   `json:"filesystems"`
	Hosts       []FilesystemHostStatus `json:"hosts"`
}
