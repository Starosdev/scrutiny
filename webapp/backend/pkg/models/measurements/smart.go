package measurements

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/sirupsen/logrus"
)

type Smart struct {
	Date           time.Time `json:"date"`
	DeviceWWN      string    `json:"device_wwn"` //(tag)
	DeviceProtocol string    `json:"device_protocol"`

	//Metrics (fields)
	Temp            int64 `json:"temp"`
	PowerOnHours    int64 `json:"power_on_hours"`
	PowerCycleCount int64 `json:"power_cycle_count"`
	LogicalBlockSize int64 `json:"logical_block_size"` //logical block size in bytes (typically 512 or 4096)

	//Attributes (fields)
	Attributes map[string]SmartAttribute `json:"attrs"`

	//status
	Status           pkg.DeviceStatus
	HasForcedFailure bool // True when an override with action=force_status, status=failed was applied
}

func (sm *Smart) Flatten() (tags map[string]string, fields map[string]interface{}) {
	tags = map[string]string{
		"device_wwn":      sm.DeviceWWN,
		"device_protocol": sm.DeviceProtocol,
	}

	fields = map[string]interface{}{
		"temp":               sm.Temp,
		"power_on_hours":     sm.PowerOnHours,
		"power_cycle_count":  sm.PowerCycleCount,
		"logical_block_size": sm.LogicalBlockSize,
	}

	for _, attr := range sm.Attributes {
		for attrKey, attrVal := range attr.Flatten() {
			fields[attrKey] = attrVal
		}
	}

	return tags, fields
}

func NewSmartFromInfluxDB(attrs map[string]interface{}, logger logrus.FieldLogger) (*Smart, error) {
	//go though the massive map returned from influxdb. If a key is associated with the Smart struct, assign it. If it starts with "attr.*" group it by attributeId, and pass to attribute inflate.

	sm := Smart{
		//required fields
		Date:           attrs["_time"].(time.Time),
		DeviceWWN:      attrs["device_wwn"].(string),
		DeviceProtocol: attrs["device_protocol"].(string),

		Attributes: map[string]SmartAttribute{},
	}

	for key, val := range attrs {
		switch key {
		case "temp":
			if intVal, ok := val.(int64); ok {
				sm.Temp = intVal
			} else {
				logger.Warnf("unable to parse temp information: %v", val)
			}
		case "power_on_hours":
			if intVal, ok := val.(int64); ok {
				sm.PowerOnHours = intVal
			} else {
				logger.Warnf("unable to parse power_on_hours information: %v", val)
			}
		case "power_cycle_count":
			if intVal, ok := val.(int64); ok {
				sm.PowerCycleCount = intVal
			} else {
				logger.Warnf("unable to parse power_cycle_count information: %v", val)
			}
		case "logical_block_size":
			if intVal, ok := val.(int64); ok {
				sm.LogicalBlockSize = intVal
			} else if intVal, ok := val.(int); ok {
				sm.LogicalBlockSize = int64(intVal)
			} else if floatVal, ok := val.(float64); ok {
				sm.LogicalBlockSize = int64(floatVal)
			}
		default:
			// this key is unknown.
			if !strings.HasPrefix(key, "attr.") {
				continue
			}
			//this is a attribute, lets group it with its related "siblings", populating a SmartAttribute object
			keyParts := strings.Split(key, ".")
			attributeId := keyParts[1]
			if _, ok := sm.Attributes[attributeId]; !ok {
				// init the attribute group
				if sm.DeviceProtocol == pkg.DeviceProtocolAta {
					// Device statistics use string-based IDs like "devstat_7_8"
					if strings.HasPrefix(attributeId, "devstat_") {
						sm.Attributes[attributeId] = &SmartAtaDeviceStatAttribute{}
					} else {
						sm.Attributes[attributeId] = &SmartAtaAttribute{}
					}
				} else if sm.DeviceProtocol == pkg.DeviceProtocolNvme {
					sm.Attributes[attributeId] = &SmartNvmeAttribute{}
				} else if sm.DeviceProtocol == pkg.DeviceProtocolScsi {
					sm.Attributes[attributeId] = &SmartScsiAttribute{}
				} else {
					return nil, fmt.Errorf("Unknown Device Protocol: %s", sm.DeviceProtocol)
				}
			}

			sm.Attributes[attributeId].Inflate(key, val)
		}

	}

	logger.Debugf("Found Smart Device (%s) Attributes (%v)", sm.DeviceWWN, len(sm.Attributes))

	return &sm, nil
}

// Parse Collector SMART data results and create Smart object (and associated SmartAtaAttribute entries)
// This version uses config-based overrides only (for backwards compatibility)
func (sm *Smart) FromCollectorSmartInfo(cfg config.Interface, wwn string, info collector.SmartInfo) error {
	// Parse overrides from config and delegate to the full version
	configOverrides := overrides.ParseOverrides(cfg)
	return sm.FromCollectorSmartInfoWithOverrides(cfg, wwn, info, configOverrides)
}

// FromCollectorSmartInfoWithOverrides parses Collector SMART data with pre-merged overrides.
// Use this when you have database overrides merged with config overrides.
func (sm *Smart) FromCollectorSmartInfoWithOverrides(cfg config.Interface, wwn string, info collector.SmartInfo, mergedOverrides []overrides.AttributeOverride) error {
	sm.DeviceWWN = wwn
	sm.Date = time.Unix(info.LocalTime.TimeT, 0)

	//smart metrics
	sm.Temp = CorrectedTemperature(&info)
	sm.PowerCycleCount = info.PowerCycleCount
	sm.PowerOnHours = info.PowerOnTime.Hours
	// Store logical block size from smartctl (default to 512 if not provided)
	if info.LogicalBlockSize > 0 {
		sm.LogicalBlockSize = int64(info.LogicalBlockSize)
	} else {
		// Default to 512 bytes if not specified (standard for most HDDs)
		sm.LogicalBlockSize = int64(512)
	}
	if !info.SmartStatus.Passed {
		sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedSmart)
	}

	sm.DeviceProtocol = info.Device.Protocol
	// process ATA/NVME/SCSI protocol data
	sm.Attributes = map[string]SmartAttribute{}
	if sm.DeviceProtocol == pkg.DeviceProtocolAta {
		sm.processAtaSmartInfoWithOverrides(cfg, info.AtaSmartAttributes.Table, mergedOverrides)
		// Also process ATA Device Statistics (GP Log 0x04) for enterprise SSD metrics
		if len(info.AtaDeviceStatistics.Pages) > 0 {
			sm.processAtaDeviceStatisticsWithOverrides(cfg, info, mergedOverrides)
		}
	} else if sm.DeviceProtocol == pkg.DeviceProtocolNvme {
		sm.processNvmeSmartInfoWithOverrides(cfg, info.NvmeSmartHealthInformationLog, mergedOverrides)
	} else if sm.DeviceProtocol == pkg.DeviceProtocolScsi {
		sm.processScsiSmartInfoWithOverrides(cfg, info.ScsiGrownDefectList, info.ScsiErrorCounterLog, info.ScsiEnvironmentalReports, mergedOverrides)
	}

	return nil
}

// generate SmartAtaAttribute entries from Scrutiny Collector Smart data.
func (sm *Smart) ProcessAtaSmartInfo(cfg config.Interface, tableItems []collector.AtaSmartAttributesTableItem) {
	for _, collectorAttr := range tableItems {
		attrModel := SmartAtaAttribute{
			AttributeId: collectorAttr.ID,
			Name:        collectorAttr.Name,
			Value:       collectorAttr.Value,
			Worst:       collectorAttr.Worst,
			Threshold:   collectorAttr.Thresh,
			RawValue:    collectorAttr.Raw.Value,
			RawString:   collectorAttr.Raw.String,
			WhenFailed:  collectorAttr.WhenFailed,
		}

		//now that we've parsed the data from the smartctl response, lets match it against our metadata rules and add additional Scrutiny specific data.
		if smartMetadata, ok := thresholds.AtaMetadata[collectorAttr.ID]; ok {
			if smartMetadata.Transform != nil {
				attrModel.TransformedValue = smartMetadata.Transform(attrModel.Value, attrModel.RawValue, attrModel.RawString)
			}
		}
		attrModel.PopulateAttributeStatus()

		attrIdStr := strconv.Itoa(collectorAttr.ID)
		var ignored bool

		// Apply user-configured overrides
		if cfg != nil {
			if result := overrides.Apply(cfg, pkg.DeviceProtocolAta, attrIdStr, sm.DeviceWWN); result != nil {
				if result.ShouldIgnore {
					// Mark as ignored - clear any failure status
					attrModel.Status = pkg.AttributeStatusPassed
					attrModel.StatusReason = result.StatusReason
					ignored = true
				} else if result.Status != nil {
					// Force status to user-specified value
					attrModel.Status = *result.Status
					attrModel.StatusReason = result.StatusReason
				} else if result.WarnAbove != nil || result.FailAbove != nil {
					// Apply custom thresholds
					if thresholdStatus := overrides.ApplyThresholds(result, attrModel.RawValue); thresholdStatus != nil {
						attrModel.Status = *thresholdStatus // Replace status entirely with custom threshold result
						if *thresholdStatus == pkg.AttributeStatusPassed {
							attrModel.StatusReason = "Within custom threshold"
						} else {
							attrModel.StatusReason = "Custom threshold exceeded"
						}
					}
				}
			}
		}

		sm.Attributes[attrIdStr] = &attrModel

		var transient bool

		if cfg != nil {
			transients := cfg.GetIntSlice("failures.transient.ata")
			for i := range transients {
				if collectorAttr.ID == transients[i] {
					transient = true
					break
				}
			}
		}

		// Only propagate failure if not transient AND not ignored
		if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !transient && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

// isDevstatIgnored checks if an attribute ID is in the devstat ignore list
func isDevstatIgnored(cfg config.Interface, attrId string) bool {
	if cfg == nil {
		return false
	}
	for _, ignoredId := range cfg.GetStringSlice("failures.ignored.devstat") {
		if attrId == ignoredId {
			return true
		}
	}
	return false
}

// ProcessAtaDeviceStatistics extracts device statistics from GP Log 0x04
// This includes important SSD metrics like "Percentage Used Endurance Indicator" on Page 7
func (sm *Smart) ProcessAtaDeviceStatistics(cfg config.Interface, deviceStatistics collector.SmartInfo) {
	for _, page := range deviceStatistics.AtaDeviceStatistics.Pages {
		for _, stat := range page.Table {
			// Skip invalid entries
			if !stat.Flags.Valid {
				continue
			}

			// Create a unique attribute ID based on page number and offset
			// Format: "devstat_<page>_<offset>" e.g., "devstat_7_8" for Percentage Used
			attrId := fmt.Sprintf("devstat_%d_%d", page.Number, stat.Offset)

			attrModel := SmartAtaDeviceStatAttribute{
				AttributeId: attrId,
				Value:       stat.Value,
			}

			attrModel.PopulateAttributeStatus()

			var ignored bool

			// Apply user-configured overrides
			if cfg != nil {
				if result := overrides.Apply(cfg, pkg.DeviceProtocolAta, attrId, sm.DeviceWWN); result != nil {
					if result.ShouldIgnore {
						attrModel.Status = pkg.AttributeStatusPassed
						attrModel.StatusReason = result.StatusReason
						ignored = true
					} else if result.Status != nil {
						attrModel.Status = *result.Status
						attrModel.StatusReason = result.StatusReason
					} else if result.WarnAbove != nil || result.FailAbove != nil {
						if thresholdStatus := overrides.ApplyThresholds(result, attrModel.Value); thresholdStatus != nil {
							attrModel.Status = *thresholdStatus // Replace status entirely with custom threshold result
							if *thresholdStatus == pkg.AttributeStatusPassed {
								attrModel.StatusReason = "Within custom threshold"
							} else {
								attrModel.StatusReason = "Custom threshold exceeded"
							}
						}
					}
				}
			}

			sm.Attributes[attrId] = &attrModel

			// Propagate failure status to device (matching ProcessAtaSmartInfo behavior)
			// Skip attributes marked as invalid (corrupted data), ignored by config, or ignored by override
			if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !isDevstatIgnored(cfg, attrId) && !ignored {
				sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
			}
		}
	}
}

// generate SmartNvmeAttribute entries from Scrutiny Collector Smart data.
func (sm *Smart) ProcessNvmeSmartInfo(cfg config.Interface, nvmeSmartHealthInformationLog collector.NvmeSmartHealthInformationLog) {

	sm.Attributes = map[string]SmartAttribute{
		"critical_warning":     (&SmartNvmeAttribute{AttributeId: "critical_warning", Value: nvmeSmartHealthInformationLog.CriticalWarning, Threshold: 0}).PopulateAttributeStatus(),
		"temperature":          (&SmartNvmeAttribute{AttributeId: "temperature", Value: nvmeSmartHealthInformationLog.Temperature, Threshold: -1}).PopulateAttributeStatus(),
		"available_spare":      (&SmartNvmeAttribute{AttributeId: "available_spare", Value: nvmeSmartHealthInformationLog.AvailableSpare, Threshold: nvmeSmartHealthInformationLog.AvailableSpareThreshold}).PopulateAttributeStatus(),
		"percentage_used":      (&SmartNvmeAttribute{AttributeId: "percentage_used", Value: nvmeSmartHealthInformationLog.PercentageUsed, Threshold: 100}).PopulateAttributeStatus(),
		"data_units_read":      (&SmartNvmeAttribute{AttributeId: "data_units_read", Value: nvmeSmartHealthInformationLog.DataUnitsRead, Threshold: -1}).PopulateAttributeStatus(),
		"data_units_written":   (&SmartNvmeAttribute{AttributeId: "data_units_written", Value: nvmeSmartHealthInformationLog.DataUnitsWritten, Threshold: -1}).PopulateAttributeStatus(),
		"host_reads":           (&SmartNvmeAttribute{AttributeId: "host_reads", Value: nvmeSmartHealthInformationLog.HostReads, Threshold: -1}).PopulateAttributeStatus(),
		"host_writes":          (&SmartNvmeAttribute{AttributeId: "host_writes", Value: nvmeSmartHealthInformationLog.HostWrites, Threshold: -1}).PopulateAttributeStatus(),
		"controller_busy_time": (&SmartNvmeAttribute{AttributeId: "controller_busy_time", Value: nvmeSmartHealthInformationLog.ControllerBusyTime, Threshold: -1}).PopulateAttributeStatus(),
		"power_cycles":         (&SmartNvmeAttribute{AttributeId: "power_cycles", Value: nvmeSmartHealthInformationLog.PowerCycles, Threshold: -1}).PopulateAttributeStatus(),
		"power_on_hours":       (&SmartNvmeAttribute{AttributeId: "power_on_hours", Value: nvmeSmartHealthInformationLog.PowerOnHours, Threshold: -1}).PopulateAttributeStatus(),
		"unsafe_shutdowns":     (&SmartNvmeAttribute{AttributeId: "unsafe_shutdowns", Value: nvmeSmartHealthInformationLog.UnsafeShutdowns, Threshold: -1}).PopulateAttributeStatus(),
		"media_errors":         (&SmartNvmeAttribute{AttributeId: "media_errors", Value: nvmeSmartHealthInformationLog.MediaErrors, Threshold: 0}).PopulateAttributeStatus(),
		"num_err_log_entries":  (&SmartNvmeAttribute{AttributeId: "num_err_log_entries", Value: nvmeSmartHealthInformationLog.NumErrLogEntries, Threshold: -1}).PopulateAttributeStatus(),
		"warning_temp_time":    (&SmartNvmeAttribute{AttributeId: "warning_temp_time", Value: nvmeSmartHealthInformationLog.WarningTempTime, Threshold: -1}).PopulateAttributeStatus(),
		"critical_comp_time":   (&SmartNvmeAttribute{AttributeId: "critical_comp_time", Value: nvmeSmartHealthInformationLog.CriticalCompTime, Threshold: -1}).PopulateAttributeStatus(),
	}

	// Apply overrides and find analyzed attribute status
	for attrId, val := range sm.Attributes {
		nvmeAttr := val.(*SmartNvmeAttribute)
		var ignored bool

		// Apply user-configured overrides
		if cfg != nil {
			if result := overrides.Apply(cfg, pkg.DeviceProtocolNvme, attrId, sm.DeviceWWN); result != nil {
				if result.ShouldIgnore {
					nvmeAttr.Status = pkg.AttributeStatusPassed
					nvmeAttr.StatusReason = result.StatusReason
					ignored = true
				} else if result.Status != nil {
					nvmeAttr.Status = *result.Status
					nvmeAttr.StatusReason = result.StatusReason
				} else if result.WarnAbove != nil || result.FailAbove != nil {
					if thresholdStatus := overrides.ApplyThresholds(result, nvmeAttr.Value); thresholdStatus != nil {
						nvmeAttr.Status = *thresholdStatus // Replace status entirely with custom threshold result
						if *thresholdStatus == pkg.AttributeStatusPassed {
							nvmeAttr.StatusReason = "Within custom threshold"
						} else {
							nvmeAttr.StatusReason = "Custom threshold exceeded"
						}
					}
				}
			}
		}

		if pkg.AttributeStatusHas(nvmeAttr.GetStatus(), pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

// generate SmartScsiAttribute entries from Scrutiny Collector Smart data.
func (sm *Smart) ProcessScsiSmartInfo(cfg config.Interface, defectGrownList int64, scsiErrorCounterLog collector.ScsiErrorCounterLog, temperature map[string]collector.ScsiTemperatureData) {
	sm.Attributes = map[string]SmartAttribute{
		"temperature": (&SmartNvmeAttribute{AttributeId: "temperature", Value: getScsiTemperature(temperature), Threshold: -1}).PopulateAttributeStatus(),

		"scsi_grown_defect_list":                     (&SmartScsiAttribute{AttributeId: "scsi_grown_defect_list", Value: defectGrownList, Threshold: 0}).PopulateAttributeStatus(),
		"read_errors_corrected_by_eccfast":           (&SmartScsiAttribute{AttributeId: "read_errors_corrected_by_eccfast", Value: scsiErrorCounterLog.Read.ErrorsCorrectedByEccfast, Threshold: -1}).PopulateAttributeStatus(),
		"read_errors_corrected_by_eccdelayed":        (&SmartScsiAttribute{AttributeId: "read_errors_corrected_by_eccdelayed", Value: scsiErrorCounterLog.Read.ErrorsCorrectedByEccdelayed, Threshold: -1}).PopulateAttributeStatus(),
		"read_errors_corrected_by_rereads_rewrites":  (&SmartScsiAttribute{AttributeId: "read_errors_corrected_by_rereads_rewrites", Value: scsiErrorCounterLog.Read.ErrorsCorrectedByRereadsRewrites, Threshold: 0}).PopulateAttributeStatus(),
		"read_total_errors_corrected":                (&SmartScsiAttribute{AttributeId: "read_total_errors_corrected", Value: scsiErrorCounterLog.Read.TotalErrorsCorrected, Threshold: -1}).PopulateAttributeStatus(),
		"read_correction_algorithm_invocations":      (&SmartScsiAttribute{AttributeId: "read_correction_algorithm_invocations", Value: scsiErrorCounterLog.Read.CorrectionAlgorithmInvocations, Threshold: -1}).PopulateAttributeStatus(),
		"read_total_uncorrected_errors":              (&SmartScsiAttribute{AttributeId: "read_total_uncorrected_errors", Value: scsiErrorCounterLog.Read.TotalUncorrectedErrors, Threshold: 0}).PopulateAttributeStatus(),
		"write_errors_corrected_by_eccfast":          (&SmartScsiAttribute{AttributeId: "write_errors_corrected_by_eccfast", Value: scsiErrorCounterLog.Write.ErrorsCorrectedByEccfast, Threshold: -1}).PopulateAttributeStatus(),
		"write_errors_corrected_by_eccdelayed":       (&SmartScsiAttribute{AttributeId: "write_errors_corrected_by_eccdelayed", Value: scsiErrorCounterLog.Write.ErrorsCorrectedByEccdelayed, Threshold: -1}).PopulateAttributeStatus(),
		"write_errors_corrected_by_rereads_rewrites": (&SmartScsiAttribute{AttributeId: "write_errors_corrected_by_rereads_rewrites", Value: scsiErrorCounterLog.Write.ErrorsCorrectedByRereadsRewrites, Threshold: 0}).PopulateAttributeStatus(),
		"write_total_errors_corrected":               (&SmartScsiAttribute{AttributeId: "write_total_errors_corrected", Value: scsiErrorCounterLog.Write.TotalErrorsCorrected, Threshold: -1}).PopulateAttributeStatus(),
		"write_correction_algorithm_invocations":     (&SmartScsiAttribute{AttributeId: "write_correction_algorithm_invocations", Value: scsiErrorCounterLog.Write.CorrectionAlgorithmInvocations, Threshold: -1}).PopulateAttributeStatus(),
		"write_total_uncorrected_errors":             (&SmartScsiAttribute{AttributeId: "write_total_uncorrected_errors", Value: scsiErrorCounterLog.Write.TotalUncorrectedErrors, Threshold: 0}).PopulateAttributeStatus(),
	}

	// Apply overrides and find analyzed attribute status
	for attrId, val := range sm.Attributes {
		var ignored bool
		var attrValue int64

		// Get the value based on attribute type
		if scsiAttr, ok := val.(*SmartScsiAttribute); ok {
			attrValue = scsiAttr.Value

			// Apply user-configured overrides
			if cfg != nil {
				if result := overrides.Apply(cfg, pkg.DeviceProtocolScsi, attrId, sm.DeviceWWN); result != nil {
					if result.ShouldIgnore {
						scsiAttr.Status = pkg.AttributeStatusPassed
						scsiAttr.StatusReason = result.StatusReason
						ignored = true
					} else if result.Status != nil {
						scsiAttr.Status = *result.Status
						scsiAttr.StatusReason = result.StatusReason
					} else if result.WarnAbove != nil || result.FailAbove != nil {
						if thresholdStatus := overrides.ApplyThresholds(result, attrValue); thresholdStatus != nil {
							scsiAttr.Status = *thresholdStatus // Replace status entirely with custom threshold result
							if *thresholdStatus == pkg.AttributeStatusPassed {
								scsiAttr.StatusReason = "Within custom threshold"
							} else {
								scsiAttr.StatusReason = "Custom threshold exceeded"
							}
						}
					}
				}
			}
		}

		if pkg.AttributeStatusHas(val.GetStatus(), pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

func getScsiTemperature(s map[string]collector.ScsiTemperatureData) int64 {
	temp, ok := s["temperature_1"]
	if !ok {
		return 0
	}

	return temp.Current
}

// processAtaSmartInfoWithOverrides generates SmartAtaAttribute entries using pre-merged overrides.
func (sm *Smart) processAtaSmartInfoWithOverrides(cfg config.Interface, tableItems []collector.AtaSmartAttributesTableItem, mergedOverrides []overrides.AttributeOverride) {
	for _, collectorAttr := range tableItems {
		attrModel := SmartAtaAttribute{
			AttributeId: collectorAttr.ID,
			Name:        collectorAttr.Name,
			Value:       collectorAttr.Value,
			Worst:       collectorAttr.Worst,
			Threshold:   collectorAttr.Thresh,
			RawValue:    collectorAttr.Raw.Value,
			RawString:   collectorAttr.Raw.String,
			WhenFailed:  collectorAttr.WhenFailed,
		}

		// Apply metadata transforms
		if smartMetadata, ok := thresholds.AtaMetadata[collectorAttr.ID]; ok {
			if smartMetadata.Transform != nil {
				attrModel.TransformedValue = smartMetadata.Transform(attrModel.Value, attrModel.RawValue, attrModel.RawString)
			}
		}
		attrModel.PopulateAttributeStatus()

		attrIdStr := strconv.Itoa(collectorAttr.ID)
		var ignored bool

		// Apply merged overrides (config + database)
		if result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolAta, attrIdStr, sm.DeviceWWN); result != nil {
			if result.ShouldIgnore {
				attrModel.Status = pkg.AttributeStatusPassed
				attrModel.StatusReason = result.StatusReason
				ignored = true
			} else if result.Status != nil {
				attrModel.Status = *result.Status
				attrModel.StatusReason = result.StatusReason
				// Track if user explicitly forced a failure status
				if pkg.AttributeStatusHas(*result.Status, pkg.AttributeStatusFailedScrutiny) {
					sm.HasForcedFailure = true
				}
			} else if result.WarnAbove != nil || result.FailAbove != nil {
				if thresholdStatus := overrides.ApplyThresholds(result, attrModel.RawValue); thresholdStatus != nil {
					attrModel.Status = *thresholdStatus // Replace status entirely with custom threshold result
					if *thresholdStatus == pkg.AttributeStatusPassed {
						attrModel.StatusReason = "Within custom threshold"
					} else {
						attrModel.StatusReason = "Custom threshold exceeded"
					}
				}
			}
		}

		sm.Attributes[attrIdStr] = &attrModel

		var transient bool

		if cfg != nil {
			transients := cfg.GetIntSlice("failures.transient.ata")
			for i := range transients {
				if collectorAttr.ID == transients[i] {
					transient = true
					break
				}
			}
		}

		// Only propagate failure if not transient AND not ignored
		if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !transient && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

// processAtaDeviceStatisticsWithOverrides extracts device statistics using pre-merged overrides.
func (sm *Smart) processAtaDeviceStatisticsWithOverrides(cfg config.Interface, deviceStatistics collector.SmartInfo, mergedOverrides []overrides.AttributeOverride) {
	for _, page := range deviceStatistics.AtaDeviceStatistics.Pages {
		for _, stat := range page.Table {
			if !stat.Flags.Valid {
				continue
			}

			attrId := fmt.Sprintf("devstat_%d_%d", page.Number, stat.Offset)

			attrModel := SmartAtaDeviceStatAttribute{
				AttributeId: attrId,
				Value:       stat.Value,
			}

			attrModel.PopulateAttributeStatus()

			var ignored bool

			// Apply merged overrides (config + database)
			if result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolAta, attrId, sm.DeviceWWN); result != nil {
				if result.ShouldIgnore {
					attrModel.Status = pkg.AttributeStatusPassed
					attrModel.StatusReason = result.StatusReason
					ignored = true
				} else if result.Status != nil {
					attrModel.Status = *result.Status
					attrModel.StatusReason = result.StatusReason
					// Track if user explicitly forced a failure status
					if pkg.AttributeStatusHas(*result.Status, pkg.AttributeStatusFailedScrutiny) {
						sm.HasForcedFailure = true
					}
				} else if result.WarnAbove != nil || result.FailAbove != nil {
					if thresholdStatus := overrides.ApplyThresholds(result, attrModel.Value); thresholdStatus != nil {
						attrModel.Status = *thresholdStatus // Replace status entirely with custom threshold result
						if *thresholdStatus == pkg.AttributeStatusPassed {
							attrModel.StatusReason = "Within custom threshold"
						} else {
							attrModel.StatusReason = "Custom threshold exceeded"
						}
					}
				}
			}

			sm.Attributes[attrId] = &attrModel

			if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !isDevstatIgnored(cfg, attrId) && !ignored {
				sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
			}
		}
	}
}

// processNvmeSmartInfoWithOverrides generates SmartNvmeAttribute entries using pre-merged overrides.
func (sm *Smart) processNvmeSmartInfoWithOverrides(cfg config.Interface, nvmeSmartHealthInformationLog collector.NvmeSmartHealthInformationLog, mergedOverrides []overrides.AttributeOverride) {
	sm.Attributes = map[string]SmartAttribute{
		"critical_warning":     (&SmartNvmeAttribute{AttributeId: "critical_warning", Value: nvmeSmartHealthInformationLog.CriticalWarning, Threshold: 0}).PopulateAttributeStatus(),
		"temperature":          (&SmartNvmeAttribute{AttributeId: "temperature", Value: nvmeSmartHealthInformationLog.Temperature, Threshold: -1}).PopulateAttributeStatus(),
		"available_spare":      (&SmartNvmeAttribute{AttributeId: "available_spare", Value: nvmeSmartHealthInformationLog.AvailableSpare, Threshold: nvmeSmartHealthInformationLog.AvailableSpareThreshold}).PopulateAttributeStatus(),
		"percentage_used":      (&SmartNvmeAttribute{AttributeId: "percentage_used", Value: nvmeSmartHealthInformationLog.PercentageUsed, Threshold: 100}).PopulateAttributeStatus(),
		"data_units_read":      (&SmartNvmeAttribute{AttributeId: "data_units_read", Value: nvmeSmartHealthInformationLog.DataUnitsRead, Threshold: -1}).PopulateAttributeStatus(),
		"data_units_written":   (&SmartNvmeAttribute{AttributeId: "data_units_written", Value: nvmeSmartHealthInformationLog.DataUnitsWritten, Threshold: -1}).PopulateAttributeStatus(),
		"host_reads":           (&SmartNvmeAttribute{AttributeId: "host_reads", Value: nvmeSmartHealthInformationLog.HostReads, Threshold: -1}).PopulateAttributeStatus(),
		"host_writes":          (&SmartNvmeAttribute{AttributeId: "host_writes", Value: nvmeSmartHealthInformationLog.HostWrites, Threshold: -1}).PopulateAttributeStatus(),
		"controller_busy_time": (&SmartNvmeAttribute{AttributeId: "controller_busy_time", Value: nvmeSmartHealthInformationLog.ControllerBusyTime, Threshold: -1}).PopulateAttributeStatus(),
		"power_cycles":         (&SmartNvmeAttribute{AttributeId: "power_cycles", Value: nvmeSmartHealthInformationLog.PowerCycles, Threshold: -1}).PopulateAttributeStatus(),
		"power_on_hours":       (&SmartNvmeAttribute{AttributeId: "power_on_hours", Value: nvmeSmartHealthInformationLog.PowerOnHours, Threshold: -1}).PopulateAttributeStatus(),
		"unsafe_shutdowns":     (&SmartNvmeAttribute{AttributeId: "unsafe_shutdowns", Value: nvmeSmartHealthInformationLog.UnsafeShutdowns, Threshold: -1}).PopulateAttributeStatus(),
		"media_errors":         (&SmartNvmeAttribute{AttributeId: "media_errors", Value: nvmeSmartHealthInformationLog.MediaErrors, Threshold: 0}).PopulateAttributeStatus(),
		"num_err_log_entries":  (&SmartNvmeAttribute{AttributeId: "num_err_log_entries", Value: nvmeSmartHealthInformationLog.NumErrLogEntries, Threshold: -1}).PopulateAttributeStatus(),
		"warning_temp_time":    (&SmartNvmeAttribute{AttributeId: "warning_temp_time", Value: nvmeSmartHealthInformationLog.WarningTempTime, Threshold: -1}).PopulateAttributeStatus(),
		"critical_comp_time":   (&SmartNvmeAttribute{AttributeId: "critical_comp_time", Value: nvmeSmartHealthInformationLog.CriticalCompTime, Threshold: -1}).PopulateAttributeStatus(),
	}

	// Apply overrides and find analyzed attribute status
	for attrId, val := range sm.Attributes {
		nvmeAttr := val.(*SmartNvmeAttribute)
		var ignored bool

		// Apply merged overrides (config + database)
		if result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolNvme, attrId, sm.DeviceWWN); result != nil {
			if result.ShouldIgnore {
				nvmeAttr.Status = pkg.AttributeStatusPassed
				nvmeAttr.StatusReason = result.StatusReason
				ignored = true
			} else if result.Status != nil {
				nvmeAttr.Status = *result.Status
				nvmeAttr.StatusReason = result.StatusReason
				// Track if user explicitly forced a failure status
				if pkg.AttributeStatusHas(*result.Status, pkg.AttributeStatusFailedScrutiny) {
					sm.HasForcedFailure = true
				}
			} else if result.WarnAbove != nil || result.FailAbove != nil {
				if thresholdStatus := overrides.ApplyThresholds(result, nvmeAttr.Value); thresholdStatus != nil {
					nvmeAttr.Status = *thresholdStatus // Replace status entirely with custom threshold result
					if *thresholdStatus == pkg.AttributeStatusPassed {
						nvmeAttr.StatusReason = "Within custom threshold"
					} else {
						nvmeAttr.StatusReason = "Custom threshold exceeded"
					}
				}
			}
		}

		if pkg.AttributeStatusHas(nvmeAttr.GetStatus(), pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

// ApplyDeltaEvaluation suppresses warnings/failures for cumulative counter attributes
// (like UltraDMA CRC Error Count) when the value hasn't increased since the previous measurement.
// This prevents false positives from historical errors that are no longer actively occurring.
// previousValues maps attribute ID strings to their previous raw values.
// Only applies to ATA attributes with UseDeltaEvaluation=true in their metadata.
func (sm *Smart) ApplyDeltaEvaluation(previousValues map[string]int64) {
	if sm.DeviceProtocol != pkg.DeviceProtocolAta || len(previousValues) == 0 {
		return
	}

	deltaApplied := false

	for _, attr := range sm.Attributes {
		ataAttr, ok := attr.(*SmartAtaAttribute)
		if !ok {
			continue
		}

		metadata, ok := thresholds.AtaMetadata[ataAttr.AttributeId]
		if !ok || !metadata.UseDeltaEvaluation {
			continue
		}

		// Only suppress Scrutiny-evaluated warnings/failures, never manufacturer SMART failures
		if pkg.AttributeStatusHas(ataAttr.Status, pkg.AttributeStatusFailedSmart) {
			continue
		}

		// Only act on attributes that are currently warning or failing
		if ataAttr.Status == pkg.AttributeStatusPassed {
			continue
		}

		attrIdStr := strconv.Itoa(ataAttr.AttributeId)
		prevValue, hasPrevious := previousValues[attrIdStr]
		if !hasPrevious {
			continue
		}

		// If the raw value hasn't changed, suppress the warning
		if ataAttr.RawValue == prevValue {
			ataAttr.Status = pkg.AttributeStatusPassed
			ataAttr.StatusReason = "Cumulative counter unchanged since last measurement"
			deltaApplied = true
		}
	}

	// If we suppressed any attribute statuses, recalculate device status
	if deltaApplied {
		sm.recalculateDeviceStatus()
	}
}

// recalculateDeviceStatus re-aggregates device status from individual attribute statuses.
// Preserves manufacturer SMART failure status, only recalculates Scrutiny status.
func (sm *Smart) recalculateDeviceStatus() {
	// Preserve manufacturer SMART failure if set
	newStatus := pkg.DeviceStatusPassed
	if pkg.DeviceStatusHas(sm.Status, pkg.DeviceStatusFailedSmart) {
		newStatus = pkg.DeviceStatusSet(newStatus, pkg.DeviceStatusFailedSmart)
	}

	for _, attr := range sm.Attributes {
		if pkg.AttributeStatusHas(attr.GetStatus(), pkg.AttributeStatusFailedScrutiny) {
			newStatus = pkg.DeviceStatusSet(newStatus, pkg.DeviceStatusFailedScrutiny)
			break
		}
	}

	sm.Status = newStatus
}

// processScsiSmartInfoWithOverrides generates SmartScsiAttribute entries using pre-merged overrides.
func (sm *Smart) processScsiSmartInfoWithOverrides(cfg config.Interface, defectGrownList int64, scsiErrorCounterLog collector.ScsiErrorCounterLog, temperature map[string]collector.ScsiTemperatureData, mergedOverrides []overrides.AttributeOverride) {
	sm.Attributes = map[string]SmartAttribute{
		"temperature": (&SmartNvmeAttribute{AttributeId: "temperature", Value: getScsiTemperature(temperature), Threshold: -1}).PopulateAttributeStatus(),

		"scsi_grown_defect_list":                     (&SmartScsiAttribute{AttributeId: "scsi_grown_defect_list", Value: defectGrownList, Threshold: 0}).PopulateAttributeStatus(),
		"read_errors_corrected_by_eccfast":           (&SmartScsiAttribute{AttributeId: "read_errors_corrected_by_eccfast", Value: scsiErrorCounterLog.Read.ErrorsCorrectedByEccfast, Threshold: -1}).PopulateAttributeStatus(),
		"read_errors_corrected_by_eccdelayed":        (&SmartScsiAttribute{AttributeId: "read_errors_corrected_by_eccdelayed", Value: scsiErrorCounterLog.Read.ErrorsCorrectedByEccdelayed, Threshold: -1}).PopulateAttributeStatus(),
		"read_errors_corrected_by_rereads_rewrites":  (&SmartScsiAttribute{AttributeId: "read_errors_corrected_by_rereads_rewrites", Value: scsiErrorCounterLog.Read.ErrorsCorrectedByRereadsRewrites, Threshold: 0}).PopulateAttributeStatus(),
		"read_total_errors_corrected":                (&SmartScsiAttribute{AttributeId: "read_total_errors_corrected", Value: scsiErrorCounterLog.Read.TotalErrorsCorrected, Threshold: -1}).PopulateAttributeStatus(),
		"read_correction_algorithm_invocations":      (&SmartScsiAttribute{AttributeId: "read_correction_algorithm_invocations", Value: scsiErrorCounterLog.Read.CorrectionAlgorithmInvocations, Threshold: -1}).PopulateAttributeStatus(),
		"read_total_uncorrected_errors":              (&SmartScsiAttribute{AttributeId: "read_total_uncorrected_errors", Value: scsiErrorCounterLog.Read.TotalUncorrectedErrors, Threshold: 0}).PopulateAttributeStatus(),
		"write_errors_corrected_by_eccfast":          (&SmartScsiAttribute{AttributeId: "write_errors_corrected_by_eccfast", Value: scsiErrorCounterLog.Write.ErrorsCorrectedByEccfast, Threshold: -1}).PopulateAttributeStatus(),
		"write_errors_corrected_by_eccdelayed":       (&SmartScsiAttribute{AttributeId: "write_errors_corrected_by_eccdelayed", Value: scsiErrorCounterLog.Write.ErrorsCorrectedByEccdelayed, Threshold: -1}).PopulateAttributeStatus(),
		"write_errors_corrected_by_rereads_rewrites": (&SmartScsiAttribute{AttributeId: "write_errors_corrected_by_rereads_rewrites", Value: scsiErrorCounterLog.Write.ErrorsCorrectedByRereadsRewrites, Threshold: 0}).PopulateAttributeStatus(),
		"write_total_errors_corrected":               (&SmartScsiAttribute{AttributeId: "write_total_errors_corrected", Value: scsiErrorCounterLog.Write.TotalErrorsCorrected, Threshold: -1}).PopulateAttributeStatus(),
		"write_correction_algorithm_invocations":     (&SmartScsiAttribute{AttributeId: "write_correction_algorithm_invocations", Value: scsiErrorCounterLog.Write.CorrectionAlgorithmInvocations, Threshold: -1}).PopulateAttributeStatus(),
		"write_total_uncorrected_errors":             (&SmartScsiAttribute{AttributeId: "write_total_uncorrected_errors", Value: scsiErrorCounterLog.Write.TotalUncorrectedErrors, Threshold: 0}).PopulateAttributeStatus(),
	}

	// Apply overrides and find analyzed attribute status
	for attrId, val := range sm.Attributes {
		var ignored bool
		var attrValue int64

		if scsiAttr, ok := val.(*SmartScsiAttribute); ok {
			attrValue = scsiAttr.Value

			// Apply merged overrides (config + database)
			if result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolScsi, attrId, sm.DeviceWWN); result != nil {
				if result.ShouldIgnore {
					scsiAttr.Status = pkg.AttributeStatusPassed
					scsiAttr.StatusReason = result.StatusReason
					ignored = true
				} else if result.Status != nil {
					scsiAttr.Status = *result.Status
					scsiAttr.StatusReason = result.StatusReason
					// Track if user explicitly forced a failure status
					if pkg.AttributeStatusHas(*result.Status, pkg.AttributeStatusFailedScrutiny) {
						sm.HasForcedFailure = true
					}
				} else if result.WarnAbove != nil || result.FailAbove != nil {
					if thresholdStatus := overrides.ApplyThresholds(result, attrValue); thresholdStatus != nil {
						scsiAttr.Status = *thresholdStatus // Replace status entirely with custom threshold result
						if *thresholdStatus == pkg.AttributeStatusPassed {
							scsiAttr.StatusReason = "Within custom threshold"
						} else {
							scsiAttr.StatusReason = "Custom threshold exceeded"
						}
					}
				}
			}
		}

		if pkg.AttributeStatusHas(val.GetStatus(), pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}
