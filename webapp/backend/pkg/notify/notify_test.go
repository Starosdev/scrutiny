package notify

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, name)
	err := os.WriteFile(scriptPath, []byte(content), 0755)
	require.NoError(t, err)
	return scriptPath
}

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
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.True(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.True(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.True(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.True(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.True(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, false, "", &gin.Context{}, fakeDatabase, nil))
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
	require.True(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, false, "", &gin.Context{}, fakeDatabase, nil))
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
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, false, "", &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_WarnLevel_PassedDeviceWithWarnAttr(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusPassed,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusWarningScrutiny,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	//assert: warn level should trigger on a device with only warning attributes
	require.True(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelWarn, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
}

func TestShouldNotify_WarnLevel_PassedDeviceWithFailLevelSetting(t *testing.T) {
	t.Parallel()
	//setup
	device := models.Device{
		DeviceStatus: pkg.DeviceStatusPassed,
	}
	smartAttrs := measurements.Smart{Attributes: map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{
			Status: pkg.AttributeStatusWarningScrutiny,
		},
	}}
	statusThreshold := pkg.MetricsStatusThresholdBoth
	notifyFilterAttributes := pkg.MetricsStatusFilterAttributesAll
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeDatabase := mock_database.NewMockDeviceRepo(mockCtrl)
	//assert: fail level should not trigger on a device with only warning attributes
	require.False(t, ShouldNotify(logrus.StandardLogger(), &device, &smartAttrs, pkg.MetricsNotifyLevelFail, statusThreshold, notifyFilterAttributes, true, "", &gin.Context{}, fakeDatabase, nil))
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
	require.Contains(t, payload.HTMLMessage, "<!DOCTYPE html>")
	require.Contains(t, payload.HTMLMessage, "TEST NOTIFICATION")
	require.Contains(t, payload.HTMLMessage, "Scrutiny SMART error (EmailTest) detected on device: /dev/sda")
	require.Contains(t, payload.HTMLMessage, "Failure Type")
	require.Contains(t, payload.HTMLMessage, "EmailTest")
	require.Contains(t, payload.HTMLMessage, currentTime.Format(time.RFC3339))
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

// Missed Ping Notification Tests

func TestNewMissedPingPayload_Basic(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceName:   "/dev/sda",
	}
	lastSeen := time.Now().Add(-2 * time.Hour)
	timeoutMinutes := 60

	//test
	payload := NewMissedPingPayload(device, lastSeen, timeoutMinutes)

	//assert
	require.Equal(t, NotifyFailureTypeMissedPing, payload.FailureType)
	require.Equal(t, "0x5000cca264eb01d7", payload.DeviceWWN)
	require.Equal(t, "/dev/sda", payload.DeviceName)
	require.Equal(t, "FAKEWDDJ324KSO", payload.DeviceSerial)
	require.Equal(t, 60, payload.TimeoutMinutes)
	require.Equal(t, "Scrutiny collector missed ping on device: /dev/sda", payload.Subject)
	require.Contains(t, payload.Message, "Scrutiny has not received data from collector for device: /dev/sda")
	require.Contains(t, payload.Message, "Device WWN: 0x5000cca264eb01d7")
	require.Contains(t, payload.Message, "Timeout threshold: 60 minutes")
}

func TestNewMissedPingPayload_WithHostId(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceName:   "/dev/sda",
		HostId:       "nas-server-01",
	}
	lastSeen := time.Now().Add(-90 * time.Minute)
	timeoutMinutes := 60

	//test
	payload := NewMissedPingPayload(device, lastSeen, timeoutMinutes)

	//assert
	require.Equal(t, "nas-server-01", payload.HostId)
	require.Equal(t, "Scrutiny collector missed ping on [host]device: [nas-server-01]/dev/sda", payload.Subject)
	require.Contains(t, payload.Message, "Host Id: nas-server-01")
}

func TestNewMissedPingPayload_WithDeviceLabel(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceName:   "/dev/sda",
		Label:        "Parity Drive 1",
	}
	lastSeen := time.Now().Add(-90 * time.Minute)
	timeoutMinutes := 60

	//test
	payload := NewMissedPingPayload(device, lastSeen, timeoutMinutes)

	//assert
	require.Equal(t, "Parity Drive 1", payload.DeviceLabel)
	require.Equal(t, "Scrutiny collector missed ping on device: Parity Drive 1 (/dev/sda)", payload.Subject)
	require.Contains(t, payload.Message, "Device Label: Parity Drive 1")
}

func TestNewMissedPingPayload_WithHostIdAndLabel(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceName:   "/dev/sda",
		HostId:       "nas-server-01",
		Label:        "Parity Drive 1",
	}
	lastSeen := time.Now().Add(-90 * time.Minute)
	timeoutMinutes := 60

	//test
	payload := NewMissedPingPayload(device, lastSeen, timeoutMinutes)

	//assert
	require.Equal(t, "Scrutiny collector missed ping on [host]device: [nas-server-01]Parity Drive 1 (/dev/sda)", payload.Subject)
	require.Contains(t, payload.Message, "Host Id: nas-server-01")
	require.Contains(t, payload.Message, "Device Label: Parity Drive 1")
}

func TestMissedPingPayload_MessageContainsLastSeen(t *testing.T) {
	t.Parallel()

	//setup
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		SerialNumber: "FAKEWDDJ324KSO",
		DeviceName:   "/dev/sda",
	}
	lastSeen := time.Now().Add(-90 * time.Minute)
	timeoutMinutes := 60

	//test
	payload := NewMissedPingPayload(device, lastSeen, timeoutMinutes)

	//assert
	require.Contains(t, payload.Message, "Last seen:")
	require.Contains(t, payload.Message, lastSeen.Format(time.RFC3339))
	require.Contains(t, payload.Message, "Please check that the collector is running")
}

func TestNormalizeGotifyURL_Port8080_AddsDisableTLS(t *testing.T) {
	t.Parallel()
	result := normalizeGotifyURL("gotify://192.168.2.135:8080/A-iI4ewTwguZo_V")
	require.Equal(t, "gotify://192.168.2.135:8080/A-iI4ewTwguZo_V?disabletls=Yes", result)
}

func TestNormalizeGotifyURL_Port80_AddsDisableTLS(t *testing.T) {
	t.Parallel()
	result := normalizeGotifyURL("gotify://gotify-host:80/mytoken123456789")
	require.Equal(t, "gotify://gotify-host:80/mytoken123456789?disabletls=Yes", result)
}

func TestNormalizeGotifyURL_Port443_NoChange(t *testing.T) {
	t.Parallel()
	raw := "gotify://gotify-host:443/mytoken123456789"
	require.Equal(t, raw, normalizeGotifyURL(raw))
}

func TestNormalizeGotifyURL_NoPort_NoChange(t *testing.T) {
	t.Parallel()
	raw := "gotify://gotify-host/mytoken123456789"
	require.Equal(t, raw, normalizeGotifyURL(raw))
}

func TestNormalizeGotifyURL_AlreadyHasDisableTLS_NoChange(t *testing.T) {
	t.Parallel()
	raw := "gotify://192.168.2.135:8080/token?disabletls=No"
	require.Equal(t, raw, normalizeGotifyURL(raw))
}

func TestNormalizeGotifyURL_NonGotifyURL_NoChange(t *testing.T) {
	t.Parallel()
	raw := "slack://token-a/token-b/token-c"
	require.Equal(t, raw, normalizeGotifyURL(raw))
}

func TestMaskNotifyUrl_AppriseNestedURL(t *testing.T) {
	raw := "apprise+mailto://user:pass@example.com?to=alerts@example.com"
	require.Equal(t, "apprise+mailto://***:***@example.com?to=alerts%40example.com", MaskNotifyUrl(raw))
}

func TestMaskNotifyUrl_AppriseMailtoQueryCredentials(t *testing.T) {
	raw := "apprise+mailtos://example.com?smtp=smtp.example.com&from=alerts@example.com&to=admin@example.com&user=alerts@example.com&pass=secret"
	require.Equal(t, "apprise+mailtos://example.com?from=alerts%40example.com&pass=***&smtp=smtp.example.com&to=admin%40example.com&user=***", MaskNotifyUrl(raw))
}

func TestMaskNotifyUrl_AppriseDiscordWebhook(t *testing.T) {
	raw := "apprise+https://discord.com/api/webhooks/123456789/token-secret"
	require.Equal(t, "apprise+https://discord.com/api/webhooks/123456789/***", MaskNotifyUrl(raw))
}

func TestMaskNotifyUrl_AppriseSlackWebhook(t *testing.T) {
	raw := "apprise+https://hooks.slack.com/services/T000/B000/SECRET"
	require.Equal(t, "apprise+https://hooks.slack.com/services/***/***/***", MaskNotifyUrl(raw))
}

func TestMaskNotifyUrl_AppriseTelegram(t *testing.T) {
	raw := "apprise+tgram://123456789:ABCDEF/12345/"
	require.Equal(t, "apprise+tgram://***/12345/", MaskNotifyUrl(raw))
}

func TestSendAppriseNotification_Text(t *testing.T) {
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "args.txt")
	bodyPath := filepath.Join(tempDir, "body.txt")
	scriptPath := writeExecutable(t, tempDir, "mock-apprise.sh", "#!/bin/sh\nprintf '%s\n' \"$@\" > \"$SCRUTINY_ARGS_PATH\"\ncat > \"$SCRUTINY_BODY_PATH\"\n")

	originalExec := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, scriptPath, args...)
		cmd.Env = append(os.Environ(),
			"SCRUTINY_ARGS_PATH="+argsPath,
			"SCRUTINY_BODY_PATH="+bodyPath,
		)
		return cmd
	}
	t.Cleanup(func() {
		execCommandContext = originalExec
	})

	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: "Scrutiny test",
			Message: "plain text body",
		},
	}

	err := notify.SendAppriseNotification("apprise+mailto://example.com?to=alerts@example.com")
	require.NoError(t, err)

	argsData, err := os.ReadFile(argsPath)
	require.NoError(t, err)
	bodyData, err := os.ReadFile(bodyPath)
	require.NoError(t, err)

	require.Equal(t, "plain text body", string(bodyData))
	require.Contains(t, string(argsData), "--title")
	require.Contains(t, string(argsData), "Scrutiny test")
	require.Contains(t, string(argsData), "--input-format")
	require.Contains(t, string(argsData), "text")
	require.Contains(t, string(argsData), "mailto://example.com?to=alerts@example.com")
	require.NotContains(t, string(argsData), "apprise+mailto://")
}

func TestSendAppriseNotification_HTML(t *testing.T) {
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "args.txt")
	bodyPath := filepath.Join(tempDir, "body.txt")
	scriptPath := writeExecutable(t, tempDir, "mock-apprise.sh", "#!/bin/sh\nprintf '%s\n' \"$@\" > \"$SCRUTINY_ARGS_PATH\"\ncat > \"$SCRUTINY_BODY_PATH\"\n")

	originalExec := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, scriptPath, args...)
		cmd.Env = append(os.Environ(),
			"SCRUTINY_ARGS_PATH="+argsPath,
			"SCRUTINY_BODY_PATH="+bodyPath,
		)
		return cmd
	}
	t.Cleanup(func() {
		execCommandContext = originalExec
	})

	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject:     "Scrutiny HTML test",
			Message:     "plain text body",
			HTMLMessage: "<p>html body</p>",
		},
	}

	err := notify.SendAppriseNotification("apprise+discord://123/abc")
	require.NoError(t, err)

	argsData, err := os.ReadFile(argsPath)
	require.NoError(t, err)
	bodyData, err := os.ReadFile(bodyPath)
	require.NoError(t, err)

	require.Equal(t, "<p>html body</p>", string(bodyData))
	require.Contains(t, string(argsData), "html")
	require.Contains(t, string(argsData), "discord://123/abc")
}

func TestSendAppriseNotification_Failure(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := writeExecutable(t, tempDir, "mock-apprise-fail.sh", "#!/bin/sh\necho 'failed to send' >&2\nexit 1\n")

	originalExec := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, scriptPath, args...)
	}
	t.Cleanup(func() {
		execCommandContext = originalExec
	})

	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: "Scrutiny fail test",
			Message: "plain text body",
		},
	}

	err := notify.SendAppriseNotification("apprise+mailto://example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to send Apprise notification")
	require.Contains(t, err.Error(), "failed to send")
}

func TestSendToUrls_AppriseAndScript(t *testing.T) {
	tempDir := t.TempDir()
	scriptNotifyPath := writeExecutable(t, tempDir, "notify-script.sh", "#!/bin/sh\nexit 0\n")
	appriseLogPath := filepath.Join(tempDir, "apprise-targets.txt")
	appriseScriptPath := writeExecutable(t, tempDir, "mock-apprise.sh", "#!/bin/sh\nprintf '%s\n' \"$@\" > \"$SCRUTINY_ARGS_PATH\"\ncat >/dev/null\n")

	originalExec := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, appriseScriptPath, args...)
		cmd.Env = append(os.Environ(), "SCRUTINY_ARGS_PATH="+appriseLogPath)
		return cmd
	}
	t.Cleanup(func() {
		execCommandContext = originalExec
	})

	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject: "Scrutiny mix test",
			Message: "plain text body",
		},
	}

	err := notify.SendToUrls([]string{
		"apprise+mailto://example.com?to=alerts@example.com",
		"script://" + scriptNotifyPath,
	})
	require.NoError(t, err)

	argsData, err := os.ReadFile(appriseLogPath)
	require.NoError(t, err)
	require.Contains(t, string(argsData), "mailto://example.com?to=alerts@example.com")
	require.False(t, strings.Contains(string(argsData), "script://"))
}

func TestSendSMTPNotification_MultipartAlternative(t *testing.T) {
	server := newMockSMTPServer(t)
	defer server.Close()

	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject:     "Scrutiny HTML test",
			Message:     "plain text body",
			HTMLMessage: "<p>html body</p>",
		},
	}

	err := notify.SendSMTPNotification(server.URL())
	require.NoError(t, err)

	data := server.LastData()
	require.Contains(t, data, "Content-Type: multipart/alternative;")
	require.Contains(t, data, "Content-Type: text/plain; charset=UTF-8")
	require.Contains(t, data, "Content-Type: text/html; charset=UTF-8")
	require.Contains(t, data, "plain text body")
	require.Contains(t, data, "<p>html body</p>")
}

func TestSendToUrls_SMTPUsesNativeSender(t *testing.T) {
	server := newMockSMTPServer(t)
	defer server.Close()

	notify := &Notify{
		Logger: logrus.StandardLogger(),
		Payload: Payload{
			Subject:     "Scrutiny SMTP route test",
			Message:     "plain text body",
			HTMLMessage: "<p>html body</p>",
		},
	}

	err := notify.SendToUrls([]string{server.URL()})
	require.NoError(t, err)

	data := server.LastData()
	require.Contains(t, data, "Content-Type: multipart/alternative;")
	require.Contains(t, data, "plain text body")
	require.Contains(t, data, "<p>html body</p>")
}

type mockSMTPServer struct {
	listener net.Listener
	mu       sync.Mutex
	lastData string
}

func newMockSMTPServer(t *testing.T) *mockSMTPServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &mockSMTPServer{listener: listener}
	go server.serve(t)
	return server
}

func (s *mockSMTPServer) URL() string {
	return "smtp://" + s.listener.Addr().String() + "/?fromaddress=sender@example.com&toaddresses=recipient@example.com&usestarttls=No&auth=None"
}

func (s *mockSMTPServer) Close() {
	_ = s.listener.Close()
}

func (s *mockSMTPServer) LastData() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastData
}

func (s *mockSMTPServer) serve(t *testing.T) {
	t.Helper()

	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	writeSMTPLine(t, writer, "220 mock-smtp ESMTP")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			writeSMTPLine(t, writer, "250-mock-smtp")
			writeSMTPLine(t, writer, "250 OK")
		case strings.HasPrefix(line, "MAIL FROM:"):
			writeSMTPLine(t, writer, "250 OK")
		case strings.HasPrefix(line, "RCPT TO:"):
			writeSMTPLine(t, writer, "250 OK")
		case strings.HasPrefix(line, "DATA"):
			writeSMTPLine(t, writer, "354 End data with <CR><LF>.<CR><LF>")

			var data strings.Builder
			for {
				part, readErr := reader.ReadString('\n')
				if readErr != nil {
					return
				}
				if part == ".\r\n" {
					break
				}
				data.WriteString(part)
			}

			s.mu.Lock()
			s.lastData = data.String()
			s.mu.Unlock()

			writeSMTPLine(t, writer, "250 OK")
		case strings.HasPrefix(line, "QUIT"):
			writeSMTPLine(t, writer, "221 Bye")
			return
		default:
			writeSMTPLine(t, writer, "250 OK")
		}
	}
}

func writeSMTPLine(t *testing.T, writer *bufio.Writer, line string) {
	t.Helper()
	_, err := writer.WriteString(line + "\r\n")
	require.NoError(t, err)
	require.NoError(t, writer.Flush())
}
