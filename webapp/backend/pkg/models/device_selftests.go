package models

import "time"

type DeviceSelfTestStatus string

const (
	DeviceSelfTestStatusUnknown DeviceSelfTestStatus = "unknown"
	DeviceSelfTestStatusPassed  DeviceSelfTestStatus = "passed"
	DeviceSelfTestStatusFailed  DeviceSelfTestStatus = "failed"
)

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

// DeviceSelfTestHealth summarizes self-test history ordered newest first.
//
//nolint:govet // Keep nullable timestamp and status fields in API payload order.
type DeviceSelfTestHealth struct {
	Status           DeviceSelfTestStatus `json:"status"`
	HasResult        bool                 `json:"has_result"`
	LatestObservedAt *int64               `json:"latest_observed_at"`
	HasFailures      bool                 `json:"has_failures"`
}

func SummarizeDeviceSelfTests(selfTests []DeviceSelfTest) DeviceSelfTestHealth {
	health := DeviceSelfTestHealth{Status: DeviceSelfTestStatusUnknown}
	if len(selfTests) == 0 {
		return health
	}

	health.HasResult = true
	if selfTests[0].StatusPassed {
		health.Status = DeviceSelfTestStatusPassed
	} else {
		health.Status = DeviceSelfTestStatusFailed
	}
	if selfTests[0].ObservedAt > 0 {
		observedAt := selfTests[0].ObservedAt
		health.LatestObservedAt = &observedAt
	}
	for index := range selfTests {
		if !selfTests[index].StatusPassed {
			health.HasFailures = true
			break
		}
	}
	return health
}
