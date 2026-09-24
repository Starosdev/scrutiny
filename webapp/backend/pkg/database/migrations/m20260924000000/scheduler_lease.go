package m20260924000000

import "gorm.io/gorm"

// SchedulerLease is frozen at the schema this migration creates.
type SchedulerLease struct {
	Name            string `gorm:"primaryKey"`
	Holder          string `gorm:"not null"`
	ExpiresAtUnixMs int64  `gorm:"not null"`
}

func (SchedulerLease) TableName() string {
	return "scheduler_leases"
}

// Migrate adds the scheduler_leases table used to elect one replica to run the
// background monitors and the report scheduler (#880).
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&SchedulerLease{})
}
