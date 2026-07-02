package handler

import (
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestConsumerDriveProfileDenylistParsesConfiguredFamilies(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	cfg := mock_config.NewMockInterface(mockCtrl)
	key := config.DB_USER_SETTINGS_SUBKEY + ".metrics.consumer_drive_profiles_denylist"
	cfg.EXPECT().GetString(key).Return("Samsung based SSDs, WDC Red Plus")

	denied := consumerDriveProfileDenylist(cfg)
	require.Len(t, denied, 2)
	require.Contains(t, denied, "samsung_based_ssds")
	require.Contains(t, denied, "wdc_red_plus")
}

func TestConsumerDriveProfileDenylistEmptyWhenUnset(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	cfg := mock_config.NewMockInterface(mockCtrl)
	key := config.DB_USER_SETTINGS_SUBKEY + ".metrics.consumer_drive_profiles_denylist"
	cfg.EXPECT().GetString(key).Return("")

	require.Nil(t, consumerDriveProfileDenylist(cfg))
}

func TestPopulateDriveProfileMatchApplied(t *testing.T) {
	inspection := models.DriveProfileInspection{DeviceProtocol: pkg.DeviceProtocolAta}
	match := thresholds.MatchConsumerDriveProfile(pkg.DeviceProtocolAta, "Samsung based SSDs", "", nil)
	require.NotNil(t, match)

	populateDriveProfileMatch(&inspection, match, true)

	require.True(t, inspection.Matched)
	require.True(t, inspection.Applied)
	require.Equal(t, "model_family", inspection.MatchMethod)
	require.Equal(t, "Samsung based SSDs", inspection.ProfileFamily)
	require.True(t, inspection.ConfidenceMet)
	require.NotEmpty(t, inspection.ProfileSource)
	require.NotEmpty(t, inspection.CounterSeverityAttributes)
	require.Empty(t, inspection.FallbackReason)
}

func TestPopulateDriveProfileMatchDenylisted(t *testing.T) {
	inspection := models.DriveProfileInspection{DeviceProtocol: pkg.DeviceProtocolAta}
	denied := thresholds.ParseConsumerDriveProfileDenylist("Samsung based SSDs")
	match := thresholds.MatchConsumerDriveProfile(pkg.DeviceProtocolAta, "Samsung based SSDs", "", denied)
	require.NotNil(t, match)

	populateDriveProfileMatch(&inspection, match, true)

	require.True(t, inspection.Matched)
	require.False(t, inspection.Applied)
	require.Contains(t, inspection.FallbackReason, "denylisted")
}

func TestPopulateDriveProfileMatchGloballyDisabled(t *testing.T) {
	inspection := models.DriveProfileInspection{DeviceProtocol: pkg.DeviceProtocolAta}
	match := thresholds.MatchConsumerDriveProfile(pkg.DeviceProtocolAta, "Samsung based SSDs", "", nil)
	require.NotNil(t, match)

	populateDriveProfileMatch(&inspection, match, false)

	require.True(t, inspection.Matched)
	require.False(t, inspection.Applied)
	require.Contains(t, inspection.FallbackReason, "disabled globally")
}

func TestPopulateDriveProfileMatchNoMatch(t *testing.T) {
	inspection := models.DriveProfileInspection{DeviceProtocol: pkg.DeviceProtocolAta}

	populateDriveProfileMatch(&inspection, nil, true)

	require.False(t, inspection.Matched)
	require.False(t, inspection.Applied)
	require.Contains(t, inspection.FallbackReason, "No profile in the bundled catalog")
}

func TestPopulateDriveProfileMatchNonAtaProtocol(t *testing.T) {
	inspection := models.DriveProfileInspection{DeviceProtocol: pkg.DeviceProtocolNvme}

	populateDriveProfileMatch(&inspection, nil, true)

	require.False(t, inspection.Matched)
	require.Contains(t, inspection.FallbackReason, "not ATA")
}
