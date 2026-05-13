package models

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/common"
	"time"
)

type DeviceWrapper struct {
	Errors  []error  `json:"errors"`
	Data    []Device `json:"data"`
	Success bool     `json:"success"`
}

type Device struct {
	UpdatedAt                 time.Time
	CreatedAt                 time.Time
	DeletedAt                 *time.Time
	InterfaceSpeed            string              `json:"interface_speed"`
	Firmware                  string              `json:"firmware"`
	DeviceID                  string              `json:"device_id" gorm:"column:device_id;primary_key"`
	WWN                       string              `json:"wwn"`
	DeviceName                string              `json:"device_name"`
	DeviceUUID                string              `json:"device_uuid"`
	DeviceSerialID            string              `json:"device_serial_id"`
	DeviceLabel               string              `json:"device_label"`
	Manufacturer              string              `json:"manufacturer"`
	ModelName                 string              `json:"model_name"`
	InterfaceType             string              `json:"interface_type"`
	SmartDisplayMode          string              `json:"smart_display_mode" gorm:"default:'scrutiny'"`
	SerialNumber              string              `json:"serial_number"`
	CollectorVersion          string              `json:"collector_version"`
	HostId                    string              `json:"host_id"`
	Label                     string              `json:"label"`
	FormFactor                string              `json:"form_factor"`
	SmartSupport              common.SmartSupport `json:"smart_support"`
	DeviceProtocol            string              `json:"device_protocol"`
	DeviceType                string              `json:"device_type"`
	Capacity                  int64               `json:"capacity"`
	RotationSpeed             int                 `json:"rotational_speed"`
	MissedPingTimeoutOverride int                 `json:"missed_ping_timeout_override" gorm:"default:0"`
	Muted                     bool                `json:"muted"`
	Archived                  bool                `json:"archived"`
	DeviceStatus              pkg.DeviceStatus    `json:"device_status"`
	HasForcedFailure          bool                `json:"has_forced_failure" gorm:"default:false"`
}

func (dv *Device) IsAta() bool {
	return dv.DeviceProtocol == pkg.DeviceProtocolAta
}

func (dv *Device) IsScsi() bool {
	return dv.DeviceProtocol == pkg.DeviceProtocolScsi
}

func (dv *Device) IsNvme() bool {
	return dv.DeviceProtocol == pkg.DeviceProtocolNvme
}

//
////This method requires a device with an array of SmartResults.
////It will remove all SmartResults other than the first (the latest one)
////All removed SmartResults, will be processed, grouping SmartAtaAttribute by attribute_id
//// and adding theme to an array called History.
//func (dv *Device) SquashHistory() error {
//	if len(dv.SmartResults) <= 1 {
//		return nil //no ataHistory found. ignore
//	}
//
//	latestSmartResultSlice := dv.SmartResults[0:1]
//	historicalSmartResultSlice := dv.SmartResults[1:]
//
//	//re-assign the latest slice to the SmartResults field
//	dv.SmartResults = latestSmartResultSlice
//
//	//process the historical slice for ATA data
//	if len(dv.SmartResults[0].AtaAttributes) > 0 {
//		ataHistory := map[int][]SmartAtaAttribute{}
//		for _, smartResult := range historicalSmartResultSlice {
//			for _, smartAttribute := range smartResult.AtaAttributes {
//				if _, ok := ataHistory[smartAttribute.AttributeId]; !ok {
//					ataHistory[smartAttribute.AttributeId] = []SmartAtaAttribute{}
//				}
//				ataHistory[smartAttribute.AttributeId] = append(ataHistory[smartAttribute.AttributeId], smartAttribute)
//			}
//		}
//
//		//now assign the historical slices to the AtaAttributes in the latest SmartResults
//		for sandx, smartAttribute := range dv.SmartResults[0].AtaAttributes {
//			if attributeHistory, ok := ataHistory[smartAttribute.AttributeId]; ok {
//				dv.SmartResults[0].AtaAttributes[sandx].History = attributeHistory
//			}
//		}
//	}
//
//	//process the historical slice for Nvme data
//	if len(dv.SmartResults[0].NvmeAttributes) > 0 {
//		nvmeHistory := map[string][]SmartNvmeAttribute{}
//		for _, smartResult := range historicalSmartResultSlice {
//			for _, smartAttribute := range smartResult.NvmeAttributes {
//				if _, ok := nvmeHistory[smartAttribute.AttributeId]; !ok {
//					nvmeHistory[smartAttribute.AttributeId] = []SmartNvmeAttribute{}
//				}
//				nvmeHistory[smartAttribute.AttributeId] = append(nvmeHistory[smartAttribute.AttributeId], smartAttribute)
//			}
//		}
//
//		//now assign the historical slices to the AtaAttributes in the latest SmartResults
//		for sandx, smartAttribute := range dv.SmartResults[0].NvmeAttributes {
//			if attributeHistory, ok := nvmeHistory[smartAttribute.AttributeId]; ok {
//				dv.SmartResults[0].NvmeAttributes[sandx].History = attributeHistory
//			}
//		}
//	}
//	//process the historical slice for Scsi data
//	if len(dv.SmartResults[0].ScsiAttributes) > 0 {
//		scsiHistory := map[string][]SmartScsiAttribute{}
//		for _, smartResult := range historicalSmartResultSlice {
//			for _, smartAttribute := range smartResult.ScsiAttributes {
//				if _, ok := scsiHistory[smartAttribute.AttributeId]; !ok {
//					scsiHistory[smartAttribute.AttributeId] = []SmartScsiAttribute{}
//				}
//				scsiHistory[smartAttribute.AttributeId] = append(scsiHistory[smartAttribute.AttributeId], smartAttribute)
//			}
//		}
//
//		//now assign the historical slices to the AtaAttributes in the latest SmartResults
//		for sandx, smartAttribute := range dv.SmartResults[0].ScsiAttributes {
//			if attributeHistory, ok := scsiHistory[smartAttribute.AttributeId]; ok {
//				dv.SmartResults[0].ScsiAttributes[sandx].History = attributeHistory
//			}
//		}
//	}
//	return nil
//}
//
//func (dv *Device) ApplyMetadataRules() error {
//
//	//embed metadata in the latest smart attributes object
//	if len(dv.SmartResults) > 0 {
//		for ndx, attr := range dv.SmartResults[0].AtaAttributes {
//			attr.PopulateAttributeStatus()
//			dv.SmartResults[0].AtaAttributes[ndx] = attr
//		}
//
//		for ndx, attr := range dv.SmartResults[0].NvmeAttributes {
//			attr.PopulateAttributeStatus()
//			dv.SmartResults[0].NvmeAttributes[ndx] = attr
//
//		}
//
//		for ndx, attr := range dv.SmartResults[0].ScsiAttributes {
//			attr.PopulateAttributeStatus()
//			dv.SmartResults[0].ScsiAttributes[ndx] = attr
//
//		}
//	}
//	return nil
//}

// This function is called every time the collector sends SMART data to the API.
// It can be used to update device data that can change over time.
func (dv *Device) UpdateFromCollectorSmartInfo(info collector.SmartInfo) error {
	dv.ModelName = info.ModelName
	dv.Firmware = info.FirmwareVersion
	dv.DeviceProtocol = info.Device.Protocol
	dv.SmartSupport = info.SmartSupport

	if !info.SmartStatus.Passed {
		dv.DeviceStatus = pkg.DeviceStatusSet(dv.DeviceStatus, pkg.DeviceStatusFailedSmart)
	} else {
		// Clear SMART failure flag when manufacturer status passes
		dv.DeviceStatus = pkg.DeviceStatusClear(dv.DeviceStatus, pkg.DeviceStatusFailedSmart)
	}

	return nil
}
