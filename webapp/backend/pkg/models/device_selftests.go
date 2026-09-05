package models

import "time"

type DeviceSelfTest struct {
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	DeletedAt               *time.Time `json:"deleted_at,omitempty"`
	DeviceIdentity          string     `json:"-" gorm:"index:idx_device_self_tests_history"`
	DeviceID                string     `json:"device_id" gorm:"index"`
	DeviceWWN               string     `json:"device_wwn" gorm:"index"`
	TypeString              string     `json:"type_string"`
	StatusString            string     `json:"status_string"`
	ID                      uint       `json:"id" gorm:"primaryKey"`
	TypeValue               int        `json:"type_value"`
	StatusValue             int        `json:"status_value"`
	LifetimeHours           int        `json:"lifetime_hours"`
	EffectiveLifetimeHours  *int64     `json:"effective_lifetime_hours"`
	ObservedAt              int64      `json:"-" gorm:"index:idx_device_self_tests_history"`
	LogIndex                int        `json:"-" gorm:"index:idx_device_self_tests_history"`
	ObservationPowerOnHours int64      `json:"-"`
	StatusPassed            bool       `json:"status_passed"`
}
