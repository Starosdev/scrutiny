package m20260514000000

import "time"

type BtrfsScrubState string
type BtrfsFilesystemStatus string

type BtrfsFilesystem struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	UUID       string `gorm:"primaryKey"`
	HostID     string
	Label      string
	Status     BtrfsFilesystemStatus
	MountPoint string

	DeviceCount       int
	DeviceSize        int64
	DeviceAllocated   int64
	DeviceUnallocated int64
	DeviceMissing     int64
	Used              int64
	FreeEstimated     int64
	FreeMin           int64
	FreeStatfs        int64
	DataRatio         float64
	MetadataRatio     float64
	MultipleProfiles  bool

	DataProfile     string
	MetadataProfile string
	SystemProfile   string
	DataTotal       int64
	DataUsed        int64
	MetadataTotal   int64
	MetadataUsed    int64
	SystemTotal     int64
	SystemUsed      int64

	ScrubState         BtrfsScrubState
	ScrubStartedAt     *time.Time
	ScrubFinishedAt    *time.Time
	ScrubDuration      string
	ScrubTotalBytes    int64
	ScrubScrubbedBytes int64
	ScrubErrorSummary  string
	ScrubReadErrors    int64
	ScrubCsumErrors    int64
	ScrubVerifyErrors  int64
	ScrubSuperErrors   int64

	Archived bool
	Muted    bool
}

type BtrfsDevice struct {
	ID             uint   `gorm:"primary_key;autoIncrement"`
	FilesystemUUID string `gorm:"index;not null"`

	DeviceID int
	Path     string
	Size     int64
	Missing  bool

	ReadIOErrors     int64
	WriteIOErrors    int64
	FlushIOErrors    int64
	CorruptionErrors int64
	GenerationErrors int64
}
