package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	colmodels "github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/sirupsen/logrus"
)

const NotifyFailureTypeMDADMDegraded = "MDADMDegraded"

type MDADMPayload struct {
	ArrayUUID      string
	ArrayName      string
	ArrayLevel     string
	State          string
	ActiveDevices  int
	WorkingDevices int
	FailedDevices  int
	SpareDevices   int

	Date        string
	FailureType string
	Subject     string
	Message     string
}

func NewMDADMDegradedPayload(array models.MDADMArray, metrics colmodels.MDADMMetrics) MDADMPayload {
	payload := MDADMPayload{
		ArrayUUID:      array.UUID,
		ArrayName:      array.Name,
		ArrayLevel:     array.Level,
		State:          metrics.State,
		ActiveDevices:  metrics.ActiveDevices,
		WorkingDevices: metrics.WorkingDevices,
		FailedDevices:  metrics.FailedDevices,
		SpareDevices:   metrics.SpareDevices,
		Date:           time.Now().Format(time.RFC3339),
		FailureType:    NotifyFailureTypeMDADMDegraded,
	}

	payload.Subject = payload.generateSubject()
	payload.Message = payload.generateMessage()
	return payload
}

func (p *MDADMPayload) generateSubject() string {
	return fmt.Sprintf("Scrutiny RAID Degradation (%s) detected on array: %s", p.FailureType, p.ArrayName)
}

func (p *MDADMPayload) generateMessage() string {
	messageParts := []string{
		fmt.Sprintf("Scrutiny RAID Degradation notification for array: %s", p.ArrayName),
		fmt.Sprintf("Array UUID: %s", p.ArrayUUID),
		fmt.Sprintf("RAID Level: %s", p.ArrayLevel),
		fmt.Sprintf("Array State: %s", p.State),
		fmt.Sprintf("Active Devices: %d", p.ActiveDevices),
		fmt.Sprintf("Working Devices: %d", p.WorkingDevices),
		fmt.Sprintf("Failed Devices: %d", p.FailedDevices),
		fmt.Sprintf("Spare Devices: %d", p.SpareDevices),
		"",
		fmt.Sprintf(fmtDate, p.Date),
	}

	return strings.Join(messageParts, "\n")
}

func NewMDADMNotify(logger logrus.FieldLogger, appconfig config.Interface, array models.MDADMArray, metrics colmodels.MDADMMetrics) Notify {
	mdadmPayload := NewMDADMDegradedPayload(array, metrics)

	// Convert to standard Payload structure for Send() functionality
	// It uses DeviceType/DeviceName to house Array details to avoid altering the base payload.
	payload := Payload{
		DeviceType:   "MDADM",
		DeviceName:   array.Name,
		DeviceSerial: array.UUID,
		DeviceLabel:  fmt.Sprintf("RAID %s", array.Level),
		Test:         false,
		Date:         mdadmPayload.Date,
		FailureType:  mdadmPayload.FailureType,
		Subject:      mdadmPayload.Subject,
		Message:      mdadmPayload.Message,
	}

	return Notify{
		Logger:  logger,
		Config:  appconfig,
		Payload: payload,
	}
}
