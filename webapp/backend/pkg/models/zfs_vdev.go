package models

// ZFSVdevType represents the type of a ZFS vdev
type ZFSVdevType string

const (
	ZFSVdevTypeDisk    ZFSVdevType = "disk"
	ZFSVdevTypeFile    ZFSVdevType = "file"
	ZFSVdevTypeMirror  ZFSVdevType = "mirror"
	ZFSVdevTypeRaidz1  ZFSVdevType = "raidz1"
	ZFSVdevTypeRaidz2  ZFSVdevType = "raidz2"
	ZFSVdevTypeRaidz3  ZFSVdevType = "raidz3"
	ZFSVdevTypeDraid1  ZFSVdevType = "draid1"
	ZFSVdevTypeDraid2  ZFSVdevType = "draid2"
	ZFSVdevTypeDraid3  ZFSVdevType = "draid3"
	ZFSVdevTypeSpare   ZFSVdevType = "spare"
	ZFSVdevTypeLog     ZFSVdevType = "log"
	ZFSVdevTypeCache   ZFSVdevType = "cache"
	ZFSVdevTypeSpecial ZFSVdevType = "special"
	ZFSVdevTypeDedup   ZFSVdevType = "dedup"
	ZFSVdevTypeRoot    ZFSVdevType = "root" // Virtual root node for the pool
)

// ZFSVdev represents a virtual device in a ZFS pool
type ZFSVdev struct {
	ParentID       *uint         `json:"parent_id,omitempty" gorm:"index"`
	GUID           string        `json:"guid,omitempty"`
	PoolGUID       string        `json:"pool_guid" gorm:"index;not null"`
	Name           string        `json:"name"`
	Type           ZFSVdevType   `json:"type"`
	Status         ZFSPoolStatus `json:"status"`
	Path           string        `json:"path,omitempty"`
	Children       []ZFSVdev     `json:"children,omitempty" gorm:"foreignKey:ParentID;references:ID"`
	ID             uint          `json:"id" gorm:"primary_key;autoIncrement"`
	ReadErrors     int64         `json:"read_errors"`
	WriteErrors    int64         `json:"write_errors"`
	ChecksumErrors int64         `json:"checksum_errors"`
	Size           int64         `json:"size,omitempty"`
	Allocated      int64         `json:"allocated,omitempty"`
}

// IsLeaf returns true if this vdev has no children (is a disk or file)
func (v *ZFSVdev) IsLeaf() bool {
	return v.Type == ZFSVdevTypeDisk || v.Type == ZFSVdevTypeFile
}

// IsHealthy returns true if the vdev status is ONLINE
func (v *ZFSVdev) IsHealthy() bool {
	return v.Status == ZFSPoolStatusOnline
}

// HasErrors returns true if the vdev has any read, write, or checksum errors
func (v *ZFSVdev) HasErrors() bool {
	return v.ReadErrors > 0 || v.WriteErrors > 0 || v.ChecksumErrors > 0
}

// IsDataVdev returns true if this is a data vdev (not spare, log, cache, special, or dedup)
func (v *ZFSVdev) IsDataVdev() bool {
	switch v.Type {
	case ZFSVdevTypeSpare, ZFSVdevTypeLog, ZFSVdevTypeCache, ZFSVdevTypeSpecial, ZFSVdevTypeDedup:
		return false
	default:
		return true
	}
}

// IsRedundant returns true if this vdev type provides redundancy
func (v *ZFSVdev) IsRedundant() bool {
	switch v.Type {
	case ZFSVdevTypeMirror, ZFSVdevTypeRaidz1, ZFSVdevTypeRaidz2, ZFSVdevTypeRaidz3,
		ZFSVdevTypeDraid1, ZFSVdevTypeDraid2, ZFSVdevTypeDraid3:
		return true
	default:
		return false
	}
}
