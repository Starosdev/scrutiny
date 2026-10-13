package database

import (
	"context"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func recalcSmart(attrs map[string]measurements.SmartAttribute) *measurements.Smart {
	return &measurements.Smart{Attributes: attrs}
}

func TestRecalculateStatusFromSmart_KeepsFailedSmart(t *testing.T) {
	device := &models.Device{DeviceProtocol: pkg.DeviceProtocolNvme}
	smart := recalcSmart(map[string]measurements.SmartAttribute{
		"critical_warning": &measurements.SmartNvmeAttribute{AttributeId: "critical_warning", Value: 1, Status: pkg.AttributeStatusFailedSmart | pkg.AttributeStatusFailedScrutiny},
		"media_errors":     &measurements.SmartNvmeAttribute{AttributeId: "media_errors", Value: 5, Status: pkg.AttributeStatusFailedScrutiny},
	})
	// An unrelated override must not drop the manufacturer failure.
	ov := []overrides.AttributeOverride{{Protocol: pkg.DeviceProtocolNvme, AttributeId: "media_errors", Action: overrides.AttributeOverrideActionIgnore}}

	status, _ := recalculateStatusFromSmart(nil, device, smart, ov)
	require.Equal(t, pkg.DeviceStatusFailedSmart|pkg.DeviceStatusFailedScrutiny, status)
}

func TestRecalculateStatusFromSmart_IgnoredAttributeDropsFailedSmart(t *testing.T) {
	device := &models.Device{DeviceProtocol: pkg.DeviceProtocolNvme, DeviceStatus: pkg.DeviceStatusFailedSmart | pkg.DeviceStatusFailedScrutiny}
	smart := recalcSmart(map[string]measurements.SmartAttribute{
		"critical_warning": &measurements.SmartNvmeAttribute{AttributeId: "critical_warning", Value: 1, Status: pkg.AttributeStatusFailedSmart | pkg.AttributeStatusFailedScrutiny},
	})
	ov := []overrides.AttributeOverride{{Protocol: pkg.DeviceProtocolNvme, AttributeId: "critical_warning", Action: overrides.AttributeOverrideActionIgnore}}

	status, _ := recalculateStatusFromSmart(nil, device, smart, ov)
	require.Equal(t, pkg.DeviceStatusPassed, status)
}

func TestRecalculateStatusFromSmart_KeepsSmartctlOverallFailure(t *testing.T) {
	// No attribute fails, but the device carries the smartctl smart_status failure bit.
	device := &models.Device{DeviceProtocol: pkg.DeviceProtocolAta, DeviceStatus: pkg.DeviceStatusFailedSmart}
	smart := recalcSmart(map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{AttributeId: 5},
	})
	status, _ := recalculateStatusFromSmart(nil, device, smart, nil)
	require.Equal(t, pkg.DeviceStatusFailedSmart, status)
}

func TestRecalculateStatusFromSmart_ScrutinyOnlyUnchanged(t *testing.T) {
	device := &models.Device{DeviceProtocol: pkg.DeviceProtocolAta}
	smart := recalcSmart(map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{AttributeId: 5, Status: pkg.AttributeStatusFailedScrutiny},
	})
	status, _ := recalculateStatusFromSmart(nil, device, smart, nil)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, status)
}

func TestRecalculateStatusFromSmart_HonorsTransientAtaList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := mock_config.NewMockInterface(ctrl)
	cfg.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()

	device := &models.Device{DeviceProtocol: pkg.DeviceProtocolAta}
	smart := recalcSmart(map[string]measurements.SmartAttribute{
		"195": &measurements.SmartAtaAttribute{AttributeId: 195, Status: pkg.AttributeStatusFailedSmart | pkg.AttributeStatusFailedScrutiny},
	})
	status, _ := recalculateStatusFromSmart(cfg, device, smart, nil)
	require.Equal(t, pkg.DeviceStatusPassed, status)
}

func TestPersistRecalculatedDeviceStatus(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()
	both := pkg.DeviceStatusFailedSmart | pkg.DeviceStatusFailedScrutiny
	require.NoError(t, repo.gormClient.Create(&models.Device{DeviceID: "d1", WWN: "w1", DeviceStatus: both}).Error)

	stored := func() pkg.DeviceStatus {
		var d models.Device
		require.NoError(t, repo.gormClient.Where("device_id = ?", "d1").First(&d).Error)
		return d.DeviceStatus
	}

	// Unchanged: no write, status stays as is.
	require.NoError(t, repo.persistRecalculatedDeviceStatus(ctx, "d1", &models.Device{DeviceStatus: both}, both))
	require.Equal(t, both, stored())

	// A bit can be cleared while another stays set.
	require.NoError(t, repo.persistRecalculatedDeviceStatus(ctx, "d1", &models.Device{DeviceStatus: both}, pkg.DeviceStatusFailedScrutiny))
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, stored())

	// Back to passed resets.
	require.NoError(t, repo.persistRecalculatedDeviceStatus(ctx, "d1", &models.Device{DeviceStatus: pkg.DeviceStatusFailedScrutiny}, pkg.DeviceStatusPassed))
	require.Equal(t, pkg.DeviceStatusPassed, stored())
}
