package models

// FilesystemDevice links a filesystem to one whole disk that backs it.
//
// The link is a row rather than a column on either side because the relation is
// many-to-many: a disk carries several filesystems, and an LVM or md filesystem
// spans several disks. Storing it this way also keeps the join cheap in both
// directions: a device detail page asks for the filesystems on one drive, and
// the dashboard asks for every drive's filesystems at once.
type FilesystemDevice struct {
	HostID     string `json:"host_id" gorm:"primaryKey"`
	MountPoint string `json:"mount_point" gorm:"primaryKey"`
	DeviceName string `json:"device_name" gorm:"primaryKey;index"`
}

func (FilesystemDevice) TableName() string {
	return "filesystem_devices"
}
