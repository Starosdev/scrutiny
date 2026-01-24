package notify

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestShouldNotify_MustSkipPassingDevices(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusPassed,
	}
	smartAttrs := measurements.Smart{}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	//assert
	require.False(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MustSkipMutedDevices(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
		Muted:        true,
	}
	smartAttrs := measurements.Smart{}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	//assert
	require.False(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusThresholdBoth_FailingSmartDevice(t *testing.T) {
	t.Parallel()
	//setupD
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	//assert
	require.True(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusThresholdSmart_FailingSmartDevice(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{}
	statusThreshold := pkg.MetricsStatusThresholdSmart
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	//assert
	require.True(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusThresholdScrutiny_FailingSmartDevice(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{}
	statusThreshold := pkg.MetricsStatusThresholdScrutiny
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	//assert
	require.False(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusFilterAttributesCritical_WithCriticalAttrs(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusFailedSmart,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesCritical
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)

	//assert
	require.True(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusFilterAttributesCritical_WithMultipleCriticalAttrs(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusPassed,
		},
		"10": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusFailedScrutiny,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesCritical
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)

	//assert
	require.True(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusFilterAttributesCritical_WithNoCriticalAttrs(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"1": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusFailedSmart,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesCritical
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)

	//assert
	require.False(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusFilterAttributesCritical_WithNoFailingCriticalAttrs(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusPassed,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesCritical
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)

	//assert
	require.False(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_MetricsStatusFilterAttributesCritical_MetricsStatusThresholdSmart_WithCriticalAttrsFailingScrutiny(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusPassed,
		},
		"10": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusFailedScrutiny,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdSmart
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesCritical
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)

	//assert
	require.False(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, true, &gin.Context{}, fakeDatabase, nil))
}
func TestShouldNotify_NoRepeat_DatabaseFailure(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusFailedScrutiny,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	fakeDatabase.EXPECT().GetPreviousSmartSubmission(&gin.Context{}, "").Return([]measurements.Smart{}, errors.New("")).Times(1)

	//assert
	require.True(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, false, &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_NoRepeat_NoDatabaseData(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusFailedScrutiny,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	fakeDatabase.EXPECT().GetPreviousSmartSubmission(&gin.Context{}, "").Return([]measurements.Smart{}, nil).Times(1)

	//assert
	require.True(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, false, &gin.Context{}, fakeDatabase, nil))
}
func TestShouldNotify_NoRepeat(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status:           pkg.AttributeStatusFailedScrutiny,
			TransformedValue: 0,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	fakeDatabase.EXPECT().GetPreviousSmartSubmission(&gin.Context{}, "").Return([]measurements.Smart{smartAttrs}, nil).Times(1)

	//assert
	require.False(t, ShouldNotify(logrus.StandardLogger(), device, smartAttrs, statusThreshold, notifyFilterAttributes, false, &gin.Context{}, fakeDatabase, nil))
}

func TestNewPayload(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceType:   pkg.DeviceProtocolAta,
		DeviceName:   "/dev/sda",
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
	}
	currentTime := time.Now()
	//test

	payload := NewPayload(device, false, currentTime)

	//assert
	require.Equal(t, "Scrutiny SMART error (ScrutinyFailure) detected on device: /dev/sda", payload.Subject)
	require.Equal(t, fmt.Sprintf(`Scrutiny SMART error notification for device: /dev/sda
Failure Type: ScrutinyFailure
Device Name: /dev/sda
Device Serial: FAKEWDDJ324KSO
Device Type: ATA

Date: %s`, currentTime.Format(time.RFC3339)), payload.Message)
}

func TestNewPayload_TestMode(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceType:   pkg.DeviceProtocolAta,
		DeviceName:   "/dev/sda",
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
	}
	currentTime := time.Now()
	//test

	payload := NewPayload(device, true, currentTime)

	//assert
	require.Equal(t, "Scrutiny SMART error (EmailTest) detected on device: /dev/sda", payload.Subject)
	require.Equal(t, fmt.Sprintf(`TEST NOTIFICATION:
Scrutiny SMART error notification for device: /dev/sda
Failure Type: EmailTest
Device Name: /dev/sda
Device Serial: FAKEWDDJ324KSO
Device Type: ATA

Date: %s`, currentTime.Format(time.RFC3339)), payload.Message)
}

func TestNewPayload_WithHostId(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceType:   pkg.DeviceProtocolAta,
		DeviceName:   "/dev/sda",
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
		HostId:       "custom-host",
	}
	currentTime := time.Now()
	//test

	payload := NewPayload(device, false, currentTime)

	//assert
	require.Equal(t, "Scrutiny SMART error (ScrutinyFailure) detected on [host]device: [custom-host]/dev/sda", payload.Subject)
	require.Equal(t, fmt.Sprintf(`Scrutiny SMART error notification for device: /dev/sda
Host Id: custom-host
Failure Type: ScrutinyFailure
Device Name: /dev/sda
Device Serial: FAKEWDDJ324KSO
Device Type: ATA

Date: %s`, currentTime.Format(time.RFC3339)), payload.Message)
}

func TestNewPayload_WithDeviceLabel(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceType:   pkg.DeviceProtocolAta,
		DeviceName:   "/dev/sda",
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
		Label:        "Parity Drive 1",
	}
	currentTime := time.Now()
	//test

	payload := NewPayload(device, false, currentTime)

	//assert
	require.Equal(t, "FAKEWDDJ324KSO", payload.DeviceSerial)
	require.Equal(t, "Parity Drive 1", payload.DeviceLabel)
	require.Equal(t, "Scrutiny SMART error (ScrutinyFailure) detected on device: Parity Drive 1 (/dev/sda)", payload.Subject)
	require.Equal(t, fmt.Sprintf(`Scrutiny SMART error notification for device: /dev/sda
Failure Type: ScrutinyFailure
Device Name: /dev/sda
Device Serial: FAKEWDDJ324KSO
Device Type: ATA
Device Label: Parity Drive 1

Date: %s`, currentTime.Format(time.RFC3339)), payload.Message)
}

func TestGenShoutrrrNotificationParams_Zulip_ShortSubject(t *testing.T) {
	t.Parallel()

	//setup
	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: "Short subject under 60 chars",
		},
	}

	//test
	serviceName, params, err := notify.GenShoutrrrNotificationParams("zulip://bot@example.com:token@zulip.example.com:443/?stream=alerts")

	//assert
	require.NoError(t, err)
	require.Equal(t, "zulip", serviceName)
	require.Equal(t, "Short subject under 60 chars", (*params)["topic"])
}

func TestGenShoutrrrNotificationParams_Zulip_LongSubjectTruncation(t *testing.T) {
	t.Parallel()

	//setup - subject is 67 characters, should be truncated to 60
	longSubject := "Scrutiny SMART error (ScrutinyFailure) detected on device: /dev/sda"
	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: longSubject,
		},
	}

	//test
	serviceName, params, err := notify.GenShoutrrrNotificationParams("zulip://bot@example.com:token@zulip.example.com:443/?stream=alerts")

	//assert
	require.NoError(t, err)
	require.Equal(t, "zulip", serviceName)
	require.Equal(t, 60, len((*params)["topic"]))
	require.Equal(t, longSubject[:60], (*params)["topic"])
}

func TestGenShoutrrrNotificationParams_Zulip_ForceTopic(t *testing.T) {
	t.Parallel()

	//setup
	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: "Scrutiny SMART error (ScrutinyFailure) detected on device: /dev/sda",
		},
	}

	//test - force_topic should override the subject
	serviceName, params, err := notify.GenShoutrrrNotificationParams("zulip://bot@example.com:token@zulip.example.com:443/?stream=alerts&force_topic=scrutiny")

	//assert
	require.NoError(t, err)
	require.Equal(t, "zulip", serviceName)
	require.Equal(t, "scrutiny", (*params)["topic"])
}

func TestGenShoutrrrNotificationParams_Zulip_EmptyForceTopic(t *testing.T) {
	t.Parallel()

	//setup
	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: "Short subject",
		},
	}

	//test - empty force_topic should fall back to subject
	serviceName, params, err := notify.GenShoutrrrNotificationParams("zulip://bot@example.com:token@zulip.example.com:443/?stream=alerts&force_topic=")

	//assert
	require.NoError(t, err)
	require.Equal(t, "zulip", serviceName)
	require.Equal(t, "Short subject", (*params)["topic"])
}

func TestGenShoutrrrNotificationParams_Zulip_ForceTopicTruncation(t *testing.T) {
	t.Parallel()

	//setup
	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: "Short subject",
		},
	}
	longForceTopic := "this-is-a-very-long-force-topic-that-exceeds-sixty-characters-limit"

	//test - force_topic over 60 chars should also be truncated
	serviceName, params, err := notify.GenShoutrrrNotificationParams("zulip://bot@example.com:token@zulip.example.com:443/?stream=alerts&force_topic=" + longForceTopic)

	//assert
	require.NoError(t, err)
	require.Equal(t, "zulip", serviceName)
	require.Equal(t, 60, len((*params)["topic"]))
	require.Equal(t, longForceTopic[:60], (*params)["topic"])
}
