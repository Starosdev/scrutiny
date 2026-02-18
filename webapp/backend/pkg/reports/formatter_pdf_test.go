package reports

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePDF_CreatesFile(t *testing.T) {
	report := &ReportData{
		GeneratedAt:   time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodStart:   time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:    "daily",
		TotalDevices:  2,
		PassedDevices: 1,
		FailedDevices: 1,
		Devices: []DeviceReport{
			{
				WWN: "0x5000cca264eb01d7", Name: "/dev/sda", Model: "WDC WD40EFRX",
				Serial: "WD-12345", Protocol: "ATA", Status: 0,
				TempCurrent: 35, TempMin: 30, TempMax: 40, TempAvg: 35.0,
				PowerOnHours: 25000, PowerCycleCount: 150,
				NewAlerts: []AlertEntry{}, ActiveFailures: []AlertEntry{},
			},
			{
				WWN: "0x5002538e40a22954", Name: "/dev/nvme0", Model: "Samsung 970 EVO",
				Serial: "S4EWNF0M", Protocol: "NVMe", Status: 1,
				TempCurrent: 45, TempMin: 40, TempMax: 50, TempAvg: 44.5,
				PowerOnHours: 8000,
				NewAlerts: []AlertEntry{},
				ActiveFailures: []AlertEntry{
					{AttributeID: "media_errors", AttributeName: "Media Errors", Status: "failed", Value: 3},
				},
			},
		},
		ZFSPools: []ZFSPoolReport{
			{Name: "tank", GUID: "1234", Health: "ONLINE", Capacity: 65.5},
		},
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-report.pdf")

	err := GeneratePDF(report, outputPath, "1.27.1")
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(100))
}

func TestGeneratePDF_EmptyReport(t *testing.T) {
	report := &ReportData{
		GeneratedAt: time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodStart: time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:  "daily",
		Devices:     []DeviceReport{},
		ZFSPools:    []ZFSPoolReport{},
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "empty-report.pdf")

	err := GeneratePDF(report, outputPath, "1.27.1")
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(100))
}

func TestGeneratePDF_InvalidPath(t *testing.T) {
	report := &ReportData{
		GeneratedAt: time.Now(),
		PeriodType:  "daily",
		Devices:     []DeviceReport{},
		ZFSPools:    []ZFSPoolReport{},
	}

	// Use a path under /dev/null which can't have subdirs
	err := GeneratePDF(report, "/dev/null/impossible/report.pdf", "1.27.1")
	assert.Error(t, err)
}

func TestPDFFilename(t *testing.T) {
	ts := time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC)
	assert.Equal(t, "daily-2026-02-17.pdf", PDFFilename("daily", ts))
	assert.Equal(t, "weekly-2026-02-17.pdf", PDFFilename("weekly", ts))
	assert.Equal(t, "monthly-2026-02-01.pdf", PDFFilename("monthly", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)))
}
