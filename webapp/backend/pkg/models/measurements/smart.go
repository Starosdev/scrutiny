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

// Custom threshold status reason strings (S1192: deduplicated string literals)
const statusReasonWithinThreshold = "Within custom threshold"
const statusReasonThresholdExceeded = "Custom threshold exceeded"

// applyOverrideResult applies a parsed override Result to an attribute's status fields.
// thresholdValue is the value compared against custom WarnAbove/FailAbove thresholds.
// It returns ignored=true when the attribute was marked ignored, and forcedFailure=true
// when the override explicitly forced a Scrutiny failure status. A nil result is a no-op.
func applyOverrideResult(result *overrides.Result, thresholdValue int64, status *pkg.AttributeStatus, statusReason *string) (ignored bool, forcedFailure bool) {
	if result == nil {
		return false, false
	}
	switch {
	case result.ShouldIgnore:
		*status = pkg.AttributeStatusPassed
		*statusReason = result.StatusReason
		return true, false
	case result.Status != nil:
		*status = *result.Status
		*statusReason = result.StatusReason
		return false, pkg.AttributeStatusHas(*result.Status, pkg.AttributeStatusFailedScrutiny)
	case result.WarnAbove != nil || result.FailAbove != nil:
		if thresholdStatus := overrides.ApplyThresholds(result, thresholdValue); thresholdStatus != nil {
			*status = *thresholdStatus // Replace status entirely with custom threshold result
			if *thresholdStatus == pkg.AttributeStatusPassed {
				*statusReason = statusReasonWithinThreshold
			} else {
				*statusReason = statusReasonThresholdExceeded
			}
		}
	}
	return false, false
}

type Smart struct {
	Date           time.Time `json:"date"`
	DeviceWWN      string    `json:"device_wwn"` //(tag)
	DeviceID       string    `json:"device_id"`  // (tag) deterministic UUIDv5
	DeviceProtocol string    `json:"device_protocol"`
	ModelFamily    string    `json:"model_family,omitempty"`
	ModelName      string    `json:"model_name,omitempty"`

	//Metrics (fields)
	Temp             int64 `json:"temp"`
	PowerOnHours     int64 `json:"power_on_hours"`
	PowerCycleCount  int64 `json:"power_cycle_count"`
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
		"device_id":       sm.DeviceID,
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

// parseInt64Field type-asserts an InfluxDB value to int64, logging a warning on mismatch.
func parseInt64Field(val interface{}, name string, logger logrus.FieldLogger) (int64, bool) {
	if intVal, ok := val.(int64); ok {
		return intVal, true
	}
	logger.Warnf("unable to parse %s information: %v", name, val)
	return 0, false
}

// coerceInt64 converts an int64/int/float64 InfluxDB value to int64, returning fallback otherwise.
func coerceInt64(val interface{}, fallback int64) int64 {
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return fallback
	}
}

// inflateInfluxAttribute groups an "attr.*" InfluxDB key into its sibling SmartAttribute,
// creating the attribute (by protocol) on first sight. Non-"attr." keys are ignored.
func (sm *Smart) inflateInfluxAttribute(key string, val interface{}) error {
	if !strings.HasPrefix(key, "attr.") {
		return nil
	}
	//this is a attribute, lets group it with its related "siblings", populating a SmartAttribute object
	attributeId := strings.Split(key, ".")[1]
	if _, ok := sm.Attributes[attributeId]; !ok {
		// init the attribute group
		attr, err := newAttributeForProtocol(sm.DeviceProtocol, attributeId)
		if err != nil {
			return err
		}
		sm.Attributes[attributeId] = attr
	}
	sm.Attributes[attributeId].Inflate(key, val)
	return nil
}

// newAttributeForProtocol constructs an empty SmartAttribute of the concrete type matching
// the device protocol (and, for ATA, the attribute ID prefix).
func newAttributeForProtocol(protocol, attributeId string) (SmartAttribute, error) {
	switch protocol {
	case pkg.DeviceProtocolAta:
		// Device statistics use string-based IDs like "devstat_7_8"
		switch {
		case strings.HasPrefix(attributeId, "devstat_"):
			return &SmartAtaDeviceStatAttribute{}, nil
		case strings.HasPrefix(attributeId, "farm_"):
			return &SmartFarmAttribute{}, nil
		default:
			return &SmartAtaAttribute{}, nil
		}
	case pkg.DeviceProtocolNvme:
		return &SmartNvmeAttribute{}, nil
	case pkg.DeviceProtocolScsi:
		return &SmartScsiAttribute{}, nil
	default:
		return nil, fmt.Errorf("Unknown Device Protocol: %s", protocol)
	}
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
			if intVal, ok := parseInt64Field(val, "temp", logger); ok {
				sm.Temp = intVal
			}
		case "power_on_hours":
			if intVal, ok := parseInt64Field(val, "power_on_hours", logger); ok {
				sm.PowerOnHours = intVal
			}
		case "power_cycle_count":
			if intVal, ok := parseInt64Field(val, "power_cycle_count", logger); ok {
				sm.PowerCycleCount = intVal
			}
		case "logical_block_size":
			sm.LogicalBlockSize = coerceInt64(val, sm.LogicalBlockSize)
		default:
			// this key is unknown; group "attr.*" keys into their SmartAttribute siblings.
			if err := sm.inflateInfluxAttribute(key, val); err != nil {
				return nil, err
			}
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
	sm.ModelFamily = info.ModelFamily
	sm.ModelName = info.ModelName

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
		sm.processAtaSmartInfoWithOverrides(cfg, info.ModelFamily, info.ModelName, info.AtaSmartAttributes.Table, mergedOverrides)
		// Also process ATA Device Statistics (GP Log 0x04) for enterprise SSD metrics
		if len(info.AtaDeviceStatistics.Pages) > 0 {
			sm.processAtaDeviceStatisticsWithOverrides(cfg, info, mergedOverrides)
		}
		// Process Seagate FARM data if present and supported
		if info.SeagateFarmLog != nil && info.SeagateFarmLog.Supported {
			sm.processFarmDataWithOverrides(cfg, info.SeagateFarmLog, mergedOverrides)
		}
	} else if sm.DeviceProtocol == pkg.DeviceProtocolNvme {
		sm.processNvmeSmartInfoWithOverrides(cfg, info.NvmeSmartHealthInformationLog, mergedOverrides)
	} else if sm.DeviceProtocol == pkg.DeviceProtocolScsi {
		sm.processScsiSmartInfoWithOverrides(cfg, info.ScsiGrownDefectList, info.ScsiErrorCounterLog, info.ScsiEnvironmentalReports, mergedOverrides)
	}

	return nil
}

// generate SmartAtaAttribute entries from Scrutiny Collector Smart data.
func (sm *Smart) ProcessAtaSmartInfo(cfg config.Interface, modelFamily string, modelName string, tableItems []collector.AtaSmartAttributesTableItem) {
	var profile *thresholds.ConsumerDriveProfile
	if consumerDriveProfilesEnabled(cfg) {
		profile, _ = thresholds.LookupConsumerDriveProfile(pkg.DeviceProtocolAta, modelFamily, modelName)
	}
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
		attrModel.PopulateAttributeStatus(profile)

		attrIdStr := strconv.Itoa(collectorAttr.ID)
		var ignored bool

		// Apply user-configured overrides
		if cfg != nil {
			result := overrides.Apply(cfg, pkg.DeviceProtocolAta, attrIdStr, sm.DeviceWWN)
			ignored, _ = applyOverrideResult(result, attrModel.RawValue, &attrModel.Status, &attrModel.StatusReason)
		}

		sm.Attributes[attrIdStr] = &attrModel

		transient := isTransientAtaAttribute(cfg, collectorAttr.ID)

		// Only propagate failure if not transient AND not ignored
		if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !transient && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

// isTransientAtaAttribute reports whether the given ATA attribute ID is configured as transient
// (failures.transient.ata), meaning its failure status should not propagate to the device.
func isTransientAtaAttribute(cfg config.Interface, attrID int) bool {
	if cfg == nil {
		return false
	}
	for _, id := range cfg.GetIntSlice("failures.transient.ata") {
		if attrID == id {
			return true
		}
	}
	return false
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
				result := overrides.Apply(cfg, pkg.DeviceProtocolAta, attrId, sm.DeviceWWN)
				ignored, _ = applyOverrideResult(result, attrModel.Value, &attrModel.Status, &attrModel.StatusReason)
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
	sm.Attributes = buildNvmeAttributes(&nvmeSmartHealthInformationLog)

	// Apply overrides and find analyzed attribute status
	for attrId, val := range sm.Attributes {
		nvmeAttr := val.(*SmartNvmeAttribute)
		ignored := applyNvmeOverrides(cfg, sm.DeviceWWN, attrId, nvmeAttr)
		if pkg.AttributeStatusHas(nvmeAttr.GetStatus(), pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

func buildNvmeAttributes(log *collector.NvmeSmartHealthInformationLog) map[string]SmartAttribute {
	return map[string]SmartAttribute{
		"critical_warning":     (&SmartNvmeAttribute{AttributeId: "critical_warning", Value: log.CriticalWarning, Threshold: 0}).PopulateAttributeStatus(),
		"temperature":          (&SmartNvmeAttribute{AttributeId: "temperature", Value: log.Temperature, Threshold: -1}).PopulateAttributeStatus(),
		"available_spare":      (&SmartNvmeAttribute{AttributeId: "available_spare", Value: log.AvailableSpare, Threshold: log.AvailableSpareThreshold}).PopulateAttributeStatus(),
		"percentage_used":      (&SmartNvmeAttribute{AttributeId: "percentage_used", Value: log.PercentageUsed, Threshold: 100}).PopulateAttributeStatus(),
		"data_units_read":      (&SmartNvmeAttribute{AttributeId: "data_units_read", Value: log.DataUnitsRead, Threshold: -1}).PopulateAttributeStatus(),
		"data_units_written":   (&SmartNvmeAttribute{AttributeId: "data_units_written", Value: log.DataUnitsWritten, Threshold: -1}).PopulateAttributeStatus(),
		"host_reads":           (&SmartNvmeAttribute{AttributeId: "host_reads", Value: log.HostReads, Threshold: -1}).PopulateAttributeStatus(),
		"host_writes":          (&SmartNvmeAttribute{AttributeId: "host_writes", Value: log.HostWrites, Threshold: -1}).PopulateAttributeStatus(),
		"controller_busy_time": (&SmartNvmeAttribute{AttributeId: "controller_busy_time", Value: log.ControllerBusyTime, Threshold: -1}).PopulateAttributeStatus(),
		"power_cycles":         (&SmartNvmeAttribute{AttributeId: "power_cycles", Value: log.PowerCycles, Threshold: -1}).PopulateAttributeStatus(),
		"power_on_hours":       (&SmartNvmeAttribute{AttributeId: "power_on_hours", Value: log.PowerOnHours, Threshold: -1}).PopulateAttributeStatus(),
		"unsafe_shutdowns":     (&SmartNvmeAttribute{AttributeId: "unsafe_shutdowns", Value: log.UnsafeShutdowns, Threshold: -1}).PopulateAttributeStatus(),
		"media_errors":         (&SmartNvmeAttribute{AttributeId: "media_errors", Value: log.MediaErrors, Threshold: 0}).PopulateAttributeStatus(),
		"num_err_log_entries":  (&SmartNvmeAttribute{AttributeId: "num_err_log_entries", Value: log.NumErrLogEntries, Threshold: -1}).PopulateAttributeStatus(),
		"warning_temp_time":    (&SmartNvmeAttribute{AttributeId: "warning_temp_time", Value: log.WarningTempTime, Threshold: -1}).PopulateAttributeStatus(),
		"critical_comp_time":   (&SmartNvmeAttribute{AttributeId: "critical_comp_time", Value: log.CriticalCompTime, Threshold: -1}).PopulateAttributeStatus(),
	}
}

func applyNvmeOverrides(cfg config.Interface, deviceWWN string, attrId string, nvmeAttr *SmartNvmeAttribute) bool {
	if cfg == nil {
		return false
	}
	result := overrides.Apply(cfg, pkg.DeviceProtocolNvme, attrId, deviceWWN)
	if result == nil {
		return false
	}
	if result.ShouldIgnore {
		nvmeAttr.Status = pkg.AttributeStatusPassed
		nvmeAttr.StatusReason = result.StatusReason
		return true
	}
	if result.Status != nil {
		nvmeAttr.Status = *result.Status
		nvmeAttr.StatusReason = result.StatusReason
		return false
	}
	if result.WarnAbove == nil && result.FailAbove == nil {
		return false
	}
	thresholdStatus := overrides.ApplyThresholds(result, nvmeAttr.Value)
	if thresholdStatus == nil {
		return false
	}
	nvmeAttr.Status = *thresholdStatus
	if *thresholdStatus == pkg.AttributeStatusPassed {
		nvmeAttr.StatusReason = statusReasonWithinThreshold
	} else {
		nvmeAttr.StatusReason = statusReasonThresholdExceeded
	}
	return false
}

// generate SmartScsiAttribute entries from Scrutiny Collector Smart data.
func (sm *Smart) ProcessScsiSmartInfo(cfg config.Interface, defectGrownList int64, scsiErrorCounterLog collector.ScsiErrorCounterLog, temperature map[string]collector.ScsiTemperatureData) {
	sm.Attributes = map[string]SmartAttribute{
		"temperature": (&SmartScsiAttribute{AttributeId: "temperature", Value: sm.Temp, Threshold: -1}).PopulateAttributeStatus(),

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

		// Get the value based on attribute type
		if scsiAttr, ok := val.(*SmartScsiAttribute); ok && cfg != nil {
			// Apply user-configured overrides
			result := overrides.Apply(cfg, pkg.DeviceProtocolScsi, attrId, sm.DeviceWWN)
			ignored, _ = applyOverrideResult(result, scsiAttr.Value, &scsiAttr.Status, &scsiAttr.StatusReason)
		}

		if pkg.AttributeStatusHas(val.GetStatus(), pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

// processAtaSmartInfoWithOverrides generates SmartAtaAttribute entries using pre-merged overrides.
func (sm *Smart) processAtaSmartInfoWithOverrides(cfg config.Interface, modelFamily string, modelName string, tableItems []collector.AtaSmartAttributesTableItem, mergedOverrides []overrides.AttributeOverride) {
	var profile *thresholds.ConsumerDriveProfile
	if consumerDriveProfilesEnabled(cfg) {
		profile, _ = thresholds.LookupConsumerDriveProfile(pkg.DeviceProtocolAta, modelFamily, modelName)
	}
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
		attrModel.PopulateAttributeStatus(profile)

		attrIdStr := strconv.Itoa(collectorAttr.ID)

		// Apply merged overrides (config + database)
		result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolAta, attrIdStr, sm.DeviceWWN)
		ignored, forcedFailure := applyOverrideResult(result, attrModel.RawValue, &attrModel.Status, &attrModel.StatusReason)
		if forcedFailure {
			sm.HasForcedFailure = true
		}

		sm.Attributes[attrIdStr] = &attrModel

		transient := isTransientAtaAttribute(cfg, collectorAttr.ID)

		// Only propagate failure if not transient AND not ignored
		if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !transient && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}

func consumerDriveProfilesEnabled(cfg config.Interface) bool {
	if cfg == nil {
		return true
	}
	key := config.DB_USER_SETTINGS_SUBKEY + ".metrics.consumer_drive_profiles_enabled"
	if !cfg.IsSet(key) {
		return true
	}
	return cfg.GetBool(key)
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

			// Apply merged overrides (config + database)
			result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolAta, attrId, sm.DeviceWWN)
			ignored, forcedFailure := applyOverrideResult(result, attrModel.Value, &attrModel.Status, &attrModel.StatusReason)
			if forcedFailure {
				sm.HasForcedFailure = true
			}

			sm.Attributes[attrId] = &attrModel

			if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !isDevstatIgnored(cfg, attrId) && !ignored {
				sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
			}
		}
	}
}

// buildFarmAttributeValues flattens the populated pages of a Seagate FARM log into a
// map of attribute ID -> value. Absent pages contribute no entries.
func buildFarmAttributeValues(farmLog *collector.SeagateFarmLog) map[string]int64 {
	farmAttrs := map[string]int64{}

	if farmLog.DriveInfo != nil {
		farmAttrs["farm_poh"] = farmLog.DriveInfo.Poh
		farmAttrs["farm_spoh"] = farmLog.DriveInfo.Spoh
		farmAttrs["farm_head_flight_hours"] = farmLog.DriveInfo.HeadFlightHours
		farmAttrs["farm_head_load_events"] = farmLog.DriveInfo.HeadLoadEvents
		farmAttrs["farm_power_cycle_count"] = farmLog.DriveInfo.PowerCycleCount
	}

	if farmLog.Workload != nil {
		farmAttrs["farm_total_read_commands"] = farmLog.Workload.TotalReadCommands
		farmAttrs["farm_total_write_commands"] = farmLog.Workload.TotalWriteCommands
		farmAttrs["farm_logical_sectors_written"] = farmLog.Workload.LogicalSectorsWritten
		farmAttrs["farm_logical_sectors_read"] = farmLog.Workload.LogicalSectorsRead
	}

	if farmLog.Errors != nil {
		farmAttrs["farm_unrecoverable_read_errors"] = farmLog.Errors.NumberOfUnrecoverableReadErrors
		farmAttrs["farm_unrecoverable_write_errors"] = farmLog.Errors.NumberOfUnrecoverableWriteErrors
		farmAttrs["farm_reallocated_sectors"] = farmLog.Errors.NumberOfReallocatedSectors
		farmAttrs["farm_reallocation_candidates"] = farmLog.Errors.NumberOfReallocatedCandidateSectors
		farmAttrs["farm_crc_errors"] = farmLog.Errors.TotalCrcErrors
		farmAttrs["farm_command_timeouts"] = farmLog.Errors.CommandTimeOutCountTotal
	}

	if farmLog.Environ != nil {
		farmAttrs["farm_current_temperature"] = farmLog.Environ.CurentTemp
		farmAttrs["farm_highest_temperature"] = farmLog.Environ.HighestTemp
		farmAttrs["farm_lowest_temperature"] = farmLog.Environ.LowestTemp
	}

	return farmAttrs
}

// processFarmDataWithOverrides extracts Seagate FARM attributes using pre-merged overrides.
func (sm *Smart) processFarmDataWithOverrides(cfg config.Interface, farmLog *collector.SeagateFarmLog, mergedOverrides []overrides.AttributeOverride) {
	farmAttrs := buildFarmAttributeValues(farmLog)

	for attrId, value := range farmAttrs {
		attrModel := SmartFarmAttribute{
			AttributeId: attrId,
			Value:       value,
		}

		attrModel.PopulateAttributeStatus()

		// Apply merged overrides (config + database)
		result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolAta, attrId, sm.DeviceWWN)
		ignored, forcedFailure := applyOverrideResult(result, attrModel.Value, &attrModel.Status, &attrModel.StatusReason)
		if forcedFailure {
			sm.HasForcedFailure = true
		}

		sm.Attributes[attrId] = &attrModel

		if pkg.AttributeStatusHas(attrModel.Status, pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
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

		// Apply merged overrides (config + database)
		result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolNvme, attrId, sm.DeviceWWN)
		ignored, forcedFailure := applyOverrideResult(result, nvmeAttr.Value, &nvmeAttr.Status, &nvmeAttr.StatusReason)
		if forcedFailure {
			sm.HasForcedFailure = true
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
		if suppressDeltaIfUnchanged(attr, previousValues) {
			deltaApplied = true
		}
	}

	// If we suppressed any attribute statuses, recalculate device status
	if deltaApplied {
		sm.recalculateDeviceStatus()
	}
}

// suppressDeltaIfUnchanged downgrades a warning/failing cumulative-counter ATA attribute to
// passed when its raw value is unchanged from the previous measurement. It returns true when a
// suppression was applied. Manufacturer SMART failures and non-delta attributes are left alone.
func suppressDeltaIfUnchanged(attr SmartAttribute, previousValues map[string]int64) bool {
	ataAttr, ok := attr.(*SmartAtaAttribute)
	if !ok {
		return false
	}

	metadata, ok := thresholds.AtaMetadata[ataAttr.AttributeId]
	if !ok || !metadata.UseDeltaEvaluation {
		return false
	}

	// Only suppress Scrutiny-evaluated warnings/failures, never manufacturer SMART failures
	if pkg.AttributeStatusHas(ataAttr.Status, pkg.AttributeStatusFailedSmart) {
		return false
	}

	// Only act on attributes that are currently warning or failing
	if ataAttr.Status == pkg.AttributeStatusPassed {
		return false
	}

	prevValue, hasPrevious := previousValues[strconv.Itoa(ataAttr.AttributeId)]
	if !hasPrevious || ataAttr.RawValue != prevValue {
		return false
	}

	// The raw value hasn't changed, suppress the warning
	ataAttr.Status = pkg.AttributeStatusPassed
	ataAttr.StatusReason = "Cumulative counter unchanged since last measurement"
	return true
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
		"temperature": (&SmartScsiAttribute{AttributeId: "temperature", Value: sm.Temp, Threshold: -1}).PopulateAttributeStatus(),

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

		if scsiAttr, ok := val.(*SmartScsiAttribute); ok {
			// Apply merged overrides (config + database)
			result := overrides.ApplyWithOverrides(mergedOverrides, pkg.DeviceProtocolScsi, attrId, sm.DeviceWWN)
			var forcedFailure bool
			ignored, forcedFailure = applyOverrideResult(result, scsiAttr.Value, &scsiAttr.Status, &scsiAttr.StatusReason)
			if forcedFailure {
				sm.HasForcedFailure = true
			}
		}

		if pkg.AttributeStatusHas(val.GetStatus(), pkg.AttributeStatusFailedScrutiny) && !ignored {
			sm.Status = pkg.DeviceStatusSet(sm.Status, pkg.DeviceStatusFailedScrutiny)
		}
	}
}
