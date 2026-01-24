package database

import (
	"context"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
)

// Create mock using:
// mockgen -source=webapp/backend/pkg/database/interface.go -destination=webapp/backend/pkg/database/mock/mock_database.go
type DeviceRepo interface {
	Close() error
	HealthCheck(ctx context.Context) error

	RegisterDevice(ctx context.Context, dev models.Device) error
	GetDevices(ctx context.Context) ([]models.Device, error)
	UpdateDevice(ctx context.Context, wwn string, collectorSmartData collector.SmartInfo) (models.Device, error)
	UpdateDeviceStatus(ctx context.Context, wwn string, status pkg.DeviceStatus) (models.Device, error)
	ResetDeviceStatus(ctx context.Context, wwn string) (models.Device, error)
	GetDeviceDetails(ctx context.Context, wwn string) (models.Device, error)
	UpdateDeviceArchived(ctx context.Context, wwn string, archived bool) error
	UpdateDeviceMuted(ctx context.Context, wwn string, muted bool) error
	UpdateDeviceLabel(ctx context.Context, wwn string, label string) error
	DeleteDevice(ctx context.Context, wwn string) error

	SaveSmartAttributes(ctx context.Context, wwn string, collectorSmartData collector.SmartInfo) (measurements.Smart, error)
	GetSmartAttributeHistory(ctx context.Context, wwn string, durationKey string, selectEntries int, selectEntriesOffset int, attributes []string) ([]measurements.Smart, error)
	// GetPreviousSmartSubmission returns the previous raw SMART submission (without daily aggregation)
	// for use in repeat notification detection. Returns the submission before the most recent one.
	GetPreviousSmartSubmission(ctx context.Context, wwn string) ([]measurements.Smart, error)

	SaveSmartTemperature(ctx context.Context, wwn string, deviceProtocol string, collectorSmartData collector.SmartInfo, retrieveSCTTemperatureHistory bool) error

	GetSummary(ctx context.Context) (map[string]*models.DeviceSummary, error)
	GetSmartTemperatureHistory(ctx context.Context, durationKey string) (map[string][]measurements.SmartTemperature, error)

	LoadSettings(ctx context.Context) (*models.Settings, error)
	SaveSettings(ctx context.Context, settings models.Settings) error

	// ZFS Pool operations
	RegisterZFSPool(ctx context.Context, pool models.ZFSPool) error
	GetZFSPools(ctx context.Context) ([]models.ZFSPool, error)
	GetZFSPoolDetails(ctx context.Context, guid string) (models.ZFSPool, error)
	UpdateZFSPoolArchived(ctx context.Context, guid string, archived bool) error
	UpdateZFSPoolMuted(ctx context.Context, guid string, muted bool) error
	UpdateZFSPoolLabel(ctx context.Context, guid string, label string) error
	DeleteZFSPool(ctx context.Context, guid string) error
	GetZFSPoolsSummary(ctx context.Context) (map[string]*models.ZFSPool, error)

	// ZFS Pool metrics
	SaveZFSPoolMetrics(ctx context.Context, pool models.ZFSPool) error
	GetZFSPoolMetricsHistory(ctx context.Context, guid string, durationKey string) ([]measurements.ZFSPoolMetrics, error)

	// Attribute Override operations
	GetAttributeOverrides(ctx context.Context) ([]models.AttributeOverride, error)
	SaveAttributeOverride(ctx context.Context, override models.AttributeOverride) error
	DeleteAttributeOverride(ctx context.Context, id uint) error
	GetMergedOverrides(ctx context.Context) []overrides.AttributeOverride
}
