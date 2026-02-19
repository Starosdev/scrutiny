package reports

import (
	"context"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSummaryProvider is a minimal mock for testing the generator
type mockSummaryProvider struct {
	summary        map[string]*models.DeviceSummary
	tempHistory    map[string][]measurements.SmartTemperature
	smartHistory   map[string][]measurements.Smart
	zfsPoolSummary map[string]*models.ZFSPool
}

func (m *mockSummaryProvider) GetSummary(ctx context.Context) (map[string]*models.DeviceSummary, error) {
	return m.summary, nil
}

func (m *mockSummaryProvider) GetSmartTemperatureHistory(ctx context.Context, durationKey string) (map[string][]measurements.SmartTemperature, error) {
	return m.tempHistory, nil
}

func (m *mockSummaryProvider) GetSmartAttributeHistory(ctx context.Context, wwn string, durationKey string, selectEntries int, selectEntriesOffset int, attributes []string) ([]measurements.Smart, error) {
	if m.smartHistory != nil {
		if h, ok := m.smartHistory[wwn]; ok {
			return h, nil
		}
	}
	return nil, nil
}

func (m *mockSummaryProvider) GetZFSPoolsSummary(ctx context.Context) (map[string]*models.ZFSPool, error) {
	if m.zfsPoolSummary != nil {
		return m.zfsPoolSummary, nil
	}
	return map[string]*models.ZFSPool{}, nil
}

func TestGenerateReport_BasicSummary(t *testing.T) {
	now := time.Now()
	mock := &mockSummaryProvider{
		summary: map[string]*models.DeviceSummary{
			"0x5000cca264eb01d7": {
				Device: models.Device{
					WWN:            "0x5000cca264eb01d7",
					DeviceName:     "/dev/sda",
					ModelName:      "WDC WD40EFRX",
					SerialNumber:   "WD-12345",
					DeviceProtocol: "ATA",
					DeviceStatus:   pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{
					Temp:         35,
					PowerOnHours: 25000,
				},
			},
			"0x5002538e40a22954": {
				Device: models.Device{
					WWN:            "0x5002538e40a22954",
					DeviceName:     "/dev/nvme0",
					ModelName:      "Samsung 970 EVO",
					SerialNumber:   "S4EWNF0M",
					DeviceProtocol: "NVMe",
					DeviceStatus:   pkg.DeviceStatusFailedSmart,
				},
				SmartResults: &models.SmartSummary{
					Temp:         45,
					PowerOnHours: 8000,
				},
			},
		},
		tempHistory: map[string][]measurements.SmartTemperature{
			"0x5000cca264eb01d7": {
				{Date: now.Add(-12 * time.Hour), Temp: 30},
				{Date: now.Add(-6 * time.Hour), Temp: 40},
				{Date: now, Temp: 35},
			},
			"0x5002538e40a22954": {
				{Date: now.Add(-12 * time.Hour), Temp: 40},
				{Date: now, Temp: 45},
			},
		},
	}

	gen := NewGenerator(mock)
	report, err := gen.Generate(context.Background(), "daily", now.Add(-24*time.Hour), now)
	require.NoError(t, err)

	assert.Equal(t, "daily", report.PeriodType)
	assert.Equal(t, 2, report.TotalDevices)
	assert.Equal(t, 1, report.PassedDevices)
	assert.Equal(t, 1, report.FailedDevices)
	assert.Len(t, report.Devices, 2)
}

func TestGenerateReport_TempAggregation(t *testing.T) {
	now := time.Now()
	mock := &mockSummaryProvider{
		summary: map[string]*models.DeviceSummary{
			"wwn1": {
				Device: models.Device{
					WWN:          "wwn1",
					DeviceName:   "/dev/sda",
					DeviceStatus: pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{Temp: 35, PowerOnHours: 100},
			},
		},
		tempHistory: map[string][]measurements.SmartTemperature{
			"wwn1": {
				{Date: now.Add(-8 * time.Hour), Temp: 20},
				{Date: now.Add(-4 * time.Hour), Temp: 50},
				{Date: now, Temp: 35},
			},
		},
	}

	gen := NewGenerator(mock)
	report, err := gen.Generate(context.Background(), "daily", now.Add(-24*time.Hour), now)
	require.NoError(t, err)

	require.Len(t, report.Devices, 1)
	dev := report.Devices[0]
	assert.Equal(t, int64(20), dev.TempMin)
	assert.Equal(t, int64(50), dev.TempMax)
	assert.InDelta(t, 35.0, dev.TempAvg, 0.1)
}

func TestGenerateReport_ArchivedDevicesExcluded(t *testing.T) {
	now := time.Now()
	mock := &mockSummaryProvider{
		summary: map[string]*models.DeviceSummary{
			"wwn1": {
				Device: models.Device{
					WWN: "wwn1", DeviceName: "/dev/sda", Archived: false,
					DeviceStatus: pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{Temp: 35},
			},
			"wwn2": {
				Device: models.Device{
					WWN: "wwn2", DeviceName: "/dev/sdb", Archived: true,
					DeviceStatus: pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{Temp: 30},
			},
		},
		tempHistory: map[string][]measurements.SmartTemperature{},
	}

	gen := NewGenerator(mock)
	report, err := gen.Generate(context.Background(), "daily", now.Add(-24*time.Hour), now)
	require.NoError(t, err)

	assert.Equal(t, 1, report.TotalDevices)
	assert.Equal(t, 1, report.ArchivedDevices)
	assert.Len(t, report.Devices, 1)
}

func TestGenerateReport_ZFSPoolsPopulated(t *testing.T) {
	now := time.Now()
	scrubEnd := now.Add(-24 * time.Hour)
	mock := &mockSummaryProvider{
		summary:     map[string]*models.DeviceSummary{},
		tempHistory: map[string][]measurements.SmartTemperature{},
		zfsPoolSummary: map[string]*models.ZFSPool{
			"guid1": {
				GUID:                "guid1",
				Name:                "tank",
				Health:              "ONLINE",
				CapacityPercent:     65.5,
				TotalReadErrors:     0,
				TotalWriteErrors:    0,
				TotalChecksumErrors: 0,
				ScrubState:          "completed",
				ScrubEndTime:        &scrubEnd,
			},
		},
	}

	gen := NewGenerator(mock)
	report, err := gen.Generate(context.Background(), "daily", now.Add(-24*time.Hour), now)
	require.NoError(t, err)

	require.Len(t, report.ZFSPools, 1)
	pool := report.ZFSPools[0]
	assert.Equal(t, "tank", pool.Name)
	assert.Equal(t, "ONLINE", pool.Health)
	assert.Equal(t, 65.5, pool.Capacity)
	assert.Equal(t, "completed", pool.ScrubStatus)
	assert.NotNil(t, pool.LastScrubDate)
}
