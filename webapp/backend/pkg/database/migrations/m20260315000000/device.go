package m20260315000000

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"time"
)

type Device struct {
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	DeletedAt                 *time.Time
	DeviceID                  string           `json:"device_id" gorm:"column:device_id;uniqueIndex"`
	FormFactor                string           `json:"form_factor"`
	DeviceType                string           `json:"device_type"`
	DeviceUUID                string           `json:"device_uuid"`
	DeviceSerialID            string           `json:"device_serial_id"`
	DeviceLabel               string           `json:"device_label"`
	Manufacturer              string           `json:"manufacturer"`
	ModelName                 string           `json:"model_name"`
	InterfaceType             string           `json:"interface_type"`
	InterfaceSpeed            string           `json:"interface_speed"`
	SerialNumber              string           `json:"serial_number"`
	Firmware                  string           `json:"firmware"`
	WWN                       string           `json:"wwn" gorm:"primary_key"`
	DeviceProtocol            string           `json:"device_protocol"`
	DeviceName                string           `json:"device_name"`
	Label                     string           `json:"label"`
	HostId                    string           `json:"host_id"`
	CollectorVersion          string           `json:"collector_version"`
	SmartDisplayMode          string           `json:"smart_display_mode" gorm:"default:'scrutiny'"`
	Capacity                  int64            `json:"capacity"`
	RotationSpeed             int              `json:"rotational_speed"`
	MissedPingTimeoutOverride int              `json:"missed_ping_timeout_override" gorm:"default:0"`
	DeviceStatus              pkg.DeviceStatus `json:"device_status"`
	Archived                  bool             `json:"archived"`
	Muted                     bool             `json:"muted"`
	SmartSupport              bool             `json:"smart_support"`
	HasForcedFailure          bool             `json:"has_forced_failure" gorm:"default:false"`
}
