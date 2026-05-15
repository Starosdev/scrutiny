package m20260514000000

import "time"

type BtrfsScrubState string
type BtrfsFilesystemStatus string

//nolint:govet // Keep migration field order aligned with the runtime Btrfs model.
type BtrfsFilesystem struct {
	UUID               string `gorm:"primaryKey"`
	HostID             string
	Label              string
	MountPoint         string
	DataProfile        string
	MetadataProfile    string
	SystemProfile      string
	ScrubDuration      string
	ScrubErrorSummary  string
	DeletedAt          *time.Time
	ScrubStartedAt     *time.Time
	ScrubFinishedAt    *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeviceSize         int64
	DeviceAllocated    int64
	DeviceUnallocated  int64
	DeviceMissing      int64
	Used               int64
	FreeEstimated      int64
	FreeMin            int64
	FreeStatfs         int64
	DataRatio          float64
	MetadataRatio      float64
	DataTotal          int64
	DataUsed           int64
	MetadataTotal      int64
	MetadataUsed       int64
	SystemTotal        int64
	SystemUsed         int64
	ScrubTotalBytes    int64
	ScrubScrubbedBytes int64
	ScrubReadErrors    int64
	ScrubCsumErrors    int64
	ScrubVerifyErrors  int64
	ScrubSuperErrors   int64
	Status             BtrfsFilesystemStatus
	ScrubState         BtrfsScrubState
	DeviceCount        int
	MultipleProfiles   bool
	Archived           bool
	Muted              bool
}

type BtrfsDevice struct {
	FilesystemUUID string `gorm:"index;not null"`
	Path           string

	ReadIOErrors     int64
	WriteIOErrors    int64
	FlushIOErrors    int64
	CorruptionErrors int64
	GenerationErrors int64
	Size             int64
	ID               uint `gorm:"primary_key;autoIncrement"`
	DeviceID         int
	Missing          bool
}
