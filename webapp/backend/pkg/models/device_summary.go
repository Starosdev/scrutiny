package models

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"time"
)

type DeviceSummaryWrapper struct {
	Data struct {
		Summary map[string]*DeviceSummary `json:"summary"`
	} `json:"data"`
	Errors  []error `json:"errors"`
	Success bool    `json:"success"`
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
	Temp           int64     `json:"temp"`
	PowerOnHours   int64     `json:"power_on_hours,omitempty"`
}
