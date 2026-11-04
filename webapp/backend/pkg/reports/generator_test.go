package reports

import (
	"context"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
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

func (m *mockSummaryProvider) GetSmartAttributeHistory(ctx context.Context, deviceID string, durationKey string, selectEntries int, selectEntriesOffset int, attributes []string) ([]measurements.Smart, error) {
	if m.smartHistory != nil {
		if h, ok := m.smartHistory[deviceID]; ok {
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
			"devid-1": {
				Device: models.Device{
					DeviceID:       "devid-1",
					WWN:            "0x5000cca264eb01d7",
					DeviceName:     "/dev/sda",
					ModelName:      "WDC WD40EFRX",
					SerialNumber:   "WD-12345",
					DeviceProtocol: "ATA",
					DeviceStatus:   pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{
					Temp:         tempPtr(35),
					PowerOnHours: 25000,
				},
			},
			"devid-2": {
				Device: models.Device{
					DeviceID:       "devid-2",
					WWN:            "0x5002538e40a22954",
					DeviceName:     "/dev/nvme0",
					ModelName:      "Samsung 970 EVO",
					SerialNumber:   "S4EWNF0M",
					DeviceProtocol: "NVMe",
					DeviceStatus:   pkg.DeviceStatusFailedSmart,
				},
				SmartResults: &models.SmartSummary{
					Temp:         tempPtr(45),
					PowerOnHours: 8000,
				},
			},
		},
		tempHistory: map[string][]measurements.SmartTemperature{
			"devid-1": {
				{Date: now.Add(-12 * time.Hour), Temp: 30},
				{Date: now.Add(-6 * time.Hour), Temp: 40},
				{Date: now, Temp: 35},
			},
			"devid-2": {
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
			"devid-temp": {
				Device: models.Device{
					DeviceID:     "devid-temp",
					WWN:          "wwn1",
					DeviceName:   "/dev/sda",
					DeviceStatus: pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{Temp: tempPtr(35), PowerOnHours: 100},
			},
		},
		tempHistory: map[string][]measurements.SmartTemperature{
			"devid-temp": {
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
			"devid-active": {
				Device: models.Device{
					DeviceID: "devid-active", WWN: "wwn1", DeviceName: "/dev/sda", Archived: false,
					DeviceStatus: pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{Temp: tempPtr(35)},
			},
			"devid-archived": {
				Device: models.Device{
					DeviceID: "devid-archived", WWN: "wwn2", DeviceName: "/dev/sdb", Archived: true,
					DeviceStatus: pkg.DeviceStatusPassed,
				},
				SmartResults: &models.SmartSummary{Temp: tempPtr(30)},
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

func TestGenerateReport_AlertEntriesCarryNameValueAndThreshold(t *testing.T) {
	now := time.Now()
	mock := &mockSummaryProvider{
		summary: map[string]*models.DeviceSummary{
			"devid-1": {
				Device: models.Device{
					DeviceID: "devid-1", WWN: "wwn1", DeviceName: "/dev/sda",
					DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusFailedScrutiny,
				},
			},
			"devid-2": {
				Device: models.Device{
					DeviceID: "devid-2", WWN: "wwn2", DeviceName: "/dev/nvme0",
					DeviceProtocol: "NVMe", DeviceStatus: pkg.DeviceStatusFailedScrutiny,
				},
			},
		},
		tempHistory: map[string][]measurements.SmartTemperature{},
		smartHistory: map[string][]measurements.Smart{
			"devid-1": {{Attributes: map[string]measurements.SmartAttribute{
				// normalized display type: report Value, with the manufacturer threshold and the metadata name
				"3": &measurements.SmartAtaAttribute{
					AttributeId: 3, Name: "Spin_Up_Time", Value: 83, Threshold: 1, RawValue: 349,
					Status: pkg.AttributeStatusWarningScrutiny,
				},
				// raw display type: report RawValue; no Name from smartctl, so use the metadata name
				"4": &measurements.SmartAtaAttribute{
					AttributeId: 4, Value: 100, RawValue: 69,
					Status: pkg.AttributeStatusWarningScrutiny,
				},
			}}},
			"devid-2": {{Attributes: map[string]measurements.SmartAttribute{
				"media_errors": &measurements.SmartNvmeAttribute{
					AttributeId: "media_errors", Value: 7,
					Status: pkg.AttributeStatusFailedScrutiny,
				},
			}}},
		},
	}

	report, err := NewGenerator(mock).Generate(context.Background(), "weekly", now.Add(-7*24*time.Hour), now)
	require.NoError(t, err)

	byID := map[string]AlertEntry{}
	for _, dev := range report.Devices {
		for _, alert := range dev.ActiveFailures {
			byID[alert.AttributeID] = alert
		}
	}
	require.Len(t, byID, 3)

	assert.Equal(t, "Spin-Up Time", byID["3"].AttributeName)
	assert.Equal(t, int64(83), byID["3"].Value)
	assert.Equal(t, int64(1), byID["3"].Threshold)

	assert.Equal(t, "Start/Stop Count", byID["4"].AttributeName)
	assert.Equal(t, int64(69), byID["4"].Value)

	assert.NotEmpty(t, byID["media_errors"].AttributeName)
	assert.Equal(t, int64(7), byID["media_errors"].Value)
	assert.Equal(t, "failed", byID["media_errors"].Status)
}

func TestAlertAttributeDetails(t *testing.T) {
	// No built-in FARM entry has a metadata threshold, so add one for this test.
	thresholds.FarmMetadata["farm_test_threshold"] = thresholds.AtaDeviceStatisticsMetadata{
		DisplayName: "Test FARM", Threshold: 80,
	}
	defer delete(thresholds.FarmMetadata, "farm_test_threshold")

	tests := []struct {
		name          string
		attrID        string
		attr          measurements.SmartAttribute
		wantName      string
		wantValue     int64
		wantThreshold int64
	}{
		{
			name:   "ATA transformed attribute reports the transformed value, not the packed raw bytes",
			attrID: "194",
			attr: &measurements.SmartAtaAttribute{
				AttributeId: 194, Value: 60, Threshold: 0, RawValue: 65576, TransformedValue: 40,
			},
			wantName: "Temperature", wantValue: 40, wantThreshold: 0,
		},
		{
			name:   "ATA raw attribute drops the normalized-scale threshold",
			attrID: "10",
			attr: &measurements.SmartAtaAttribute{
				AttributeId: 10, Value: 100, Threshold: 97, RawValue: 2,
			},
			wantName: "Spin Retry Count", wantValue: 2, wantThreshold: 0,
		},
		{
			name:   "ATA attribute without metadata falls back to the normalized value, smartctl name and threshold",
			attrID: "999",
			attr: &measurements.SmartAtaAttribute{
				AttributeId: 999, Name: "Vendor_Thing", Value: 77, Threshold: 5, RawValue: 123456,
			},
			wantName: "Vendor_Thing", wantValue: 77, wantThreshold: 5,
		},
		{
			name:     "ATA attribute without metadata or name uses the attribute id",
			attrID:   "999",
			attr:     &measurements.SmartAtaAttribute{AttributeId: 999, Value: 77},
			wantName: "999", wantValue: 77, wantThreshold: 0,
		},
		{
			name:   "devstat uses the metadata threshold when the drive reports none",
			attrID: "devstat_7_8",
			attr: &measurements.SmartAtaDeviceStatAttribute{
				AttributeId: "devstat_7_8", Value: 100,
			},
			wantName: "Percentage Used Endurance Indicator", wantValue: 100, wantThreshold: 100,
		},
		{
			name:   "FARM uses the metadata threshold when the drive reports none",
			attrID: "farm_test_threshold",
			attr: &measurements.SmartFarmAttribute{
				AttributeId: "farm_test_threshold", Value: 90,
			},
			wantName: "Test FARM", wantValue: 90, wantThreshold: 80,
		},
		{
			name:   "NVMe no-threshold sentinel -1 becomes 0",
			attrID: "power_on_hours",
			attr: &measurements.SmartNvmeAttribute{
				AttributeId: "power_on_hours", Value: 500, Threshold: -1,
			},
			wantName: "Power on Hours", wantValue: 500, wantThreshold: 0,
		},
		{
			name:   "SCSI no-threshold sentinel -1 becomes 0",
			attrID: "scsi_grown_defect_list",
			attr: &measurements.SmartScsiAttribute{
				AttributeId: "scsi_grown_defect_list", Value: 12, Threshold: -1,
			},
			wantName: "Grown Defect List", wantValue: 12, wantThreshold: 0,
		},
		{
			name:   "SCSI reports Value and the drive threshold",
			attrID: "scsi_grown_defect_list",
			attr: &measurements.SmartScsiAttribute{
				AttributeId: "scsi_grown_defect_list", Value: 12, Threshold: 3,
			},
			wantName: "Grown Defect List", wantValue: 12, wantThreshold: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, value, threshold := alertAttributeDetails(tt.attrID, tt.attr)
			assert.Equal(t, tt.wantValue, value)
			assert.Equal(t, tt.wantThreshold, threshold)
			assert.Equal(t, tt.wantName, name)
		})
	}
}
