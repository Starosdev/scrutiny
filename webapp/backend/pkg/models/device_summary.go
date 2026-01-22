package models

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"time"
)

type DeviceSummaryWrapper struct {
	Success bool    `json:"success"`
	Errors  []error `json:"errors"`
	Data    struct {
		Summary map[string]*DeviceSummary `json:"summary"`
	} `json:"data"`
}

type DeviceSummary struct {
	Device Device `json:"device"`

	SmartResults *SmartSummary                   `json:"smart,omitempty"`
	TempHistory  []measurements.SmartTemperature `json:"temp_history,omitempty"`
}
type SmartSummary struct {
	// Collector Summary Data
	CollectorDate time.Time `json:"collector_date,omitempty"`
	Temp          int64     `json:"temp,omitempty"`
	PowerOnHours  int64     `json:"power_on_hours,omitempty"`

	// SSD Health Metrics (nullable - only present for SSDs)
	// PercentageUsed: NVMe percentage_used or ATA devstat_7_8 (0-100%, higher = more worn)
	PercentageUsed *int64 `json:"percentage_used,omitempty"`
	// WearoutValue: ATA attributes 177, 233, 231, 232 (0-100%, higher = healthier)
	WearoutValue *int64 `json:"wearout_value,omitempty"`
}
