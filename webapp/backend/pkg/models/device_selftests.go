package models

import "time"

type DeviceSelfTest struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	DeviceID       string `json:"device_id" gorm:"index"`
	DeviceWWN      string `json:"device_wwn" gorm:"index"`
	DeviceIdentity string `json:"-" gorm:"index:idx_device_self_tests_identity,unique;index:idx_device_self_tests_history"`

	TypeValue     int    `json:"type_value" gorm:"index:idx_device_self_tests_identity,unique"`
	TypeString    string `json:"type_string"`
	StatusValue   int    `json:"status_value"`
	StatusString  string `json:"status_string"`
	StatusPassed  bool   `json:"status_passed"`
	LifetimeHours int    `json:"lifetime_hours" gorm:"index:idx_device_self_tests_identity,unique;index:idx_device_self_tests_history"`
}
