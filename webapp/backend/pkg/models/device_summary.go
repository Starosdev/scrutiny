package models

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"time"
)

type DeviceSummaryWrapper struct {
	Data struct {
		Summary    map[string]*DeviceSummary `json:"summary"`
		Pagination *PaginationMetadata       `json:"pagination,omitempty"`
	} `json:"data"`
	Errors  []error `json:"errors"`
	Success bool    `json:"success"`
}

type PaginationMetadata struct {
	Page           int `json:"page"`
	PageSize       int `json:"page_size"`
	TotalItems     int `json:"total_items"`
	TotalPages     int `json:"total_pages"`
	AttentionCount int `json:"attention_count"`
}

type DeviceSummaryPageOptions struct {
	Sort        string
	Display     string
	HostSearch  string
	Page        int
	PageSize    int
	Archived    bool
	GroupByHost bool
}

type DeviceSummaryPage struct {
	Summary    map[string]*DeviceSummary
	Pagination PaginationMetadata
}

type TemperatureDeviceOption struct {
	DeviceID     string `json:"device_id"`
	HostID       string `json:"host_id"`
	Label        string `json:"label"`
	DeviceName   string `json:"device_name"`
	ModelName    string `json:"model_name"`
	SerialNumber string `json:"serial_number"`
}

type DeviceSummary struct {
	SmartResults *SmartSummary                   `json:"smart,omitempty"`
	TempHistory  []measurements.SmartTemperature `json:"temp_history,omitempty"`
	Device       Device                          `json:"device"`
}
type SmartSummary struct {
	CollectorDate  time.Time `json:"collector_date,omitempty"`
	PercentageUsed *int64    `json:"percentage_used,omitempty"`
	WearoutValue   *int64    `json:"wearout_value,omitempty"`
	RiskScore      *int      `json:"risk_score,omitempty"`
	RiskCategory   string    `json:"risk_category,omitempty"`
	// Temp is a pointer because a device with no temperature reading and one
	// reading 0C are different facts, same as PercentageUsed and WearoutValue
	// above. omitempty on a pointer omits only nil, so a genuine 0 still
	// serializes and the frontend renders it rather than "--".
	Temp         *int64 `json:"temp,omitempty"`
	PowerOnHours int64  `json:"power_on_hours,omitempty"`
}
