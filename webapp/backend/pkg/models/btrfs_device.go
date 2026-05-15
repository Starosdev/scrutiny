package models

type BtrfsDevice struct {
	ID             uint   `json:"row_id" gorm:"primary_key;autoIncrement"`
	FilesystemUUID string `json:"filesystem_uuid" gorm:"index;not null"`

	DeviceID int    `json:"id"`
	Path     string `json:"path,omitempty"`
	Size     int64  `json:"size"`
	Missing  bool   `json:"missing"`

	ReadIOErrors     int64 `json:"read_io_errors"`
	WriteIOErrors    int64 `json:"write_io_errors"`
	FlushIOErrors    int64 `json:"flush_io_errors"`
	CorruptionErrors int64 `json:"corruption_errors"`
	GenerationErrors int64 `json:"generation_errors"`
}

func (d *BtrfsDevice) HasErrors() bool {
	return d.ReadIOErrors > 0 || d.WriteIOErrors > 0 || d.FlushIOErrors > 0 || d.CorruptionErrors > 0 || d.GenerationErrors > 0
}
