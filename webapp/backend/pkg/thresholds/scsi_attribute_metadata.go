package thresholds

import (
	"fmt"
	"regexp"
)

type ScsiAttributeMetadata struct {
	Transform          func(int64, int64, string) int64 `json:"-"` //this should be a method to extract/tranform the normalized or raw data to a chartable format. Str
	ID                 string                           `json:"-"`
	DisplayName        string                           `json:"display_name"`
	Ideal              string                           `json:"ideal"`
	Description        string                           `json:"description"`
	TransformValueUnit string                           `json:"transform_value_unit,omitempty"`
	DisplayType        string                           `json:"display_type"` //"raw" "normalized" or "transformed"
	Critical           bool                             `json:"critical"`

	// UseDeltaEvaluation indicates this attribute is a cumulative counter where the trend
	// matters more than the absolute value. When true, Scrutiny suppresses a warning if the
	// value has not increased since the last measurement. See the ATA metadata field of the
	// same name; on SAS this covers the PHY transport-error counters and the verify error
	// counters, which stay nonzero long after the cable or backplane fault that caused them.
	UseDeltaEvaluation bool `json:"use_delta_evaluation,omitempty"`
}

var ScsiMetadata = map[string]ScsiAttributeMetadata{
	"scsi_grown_defect_list": {
		ID:          "scsi_grown_defect_list",
		DisplayName: "Grown Defect List",
		DisplayType: "",
		Ideal:       "low",
		Critical:    true,
		Description: "The grown defect count shows the amount of swapped (defective) blocks since the drive was shipped by it's vendor. Each additional defective block increases the count by one.",
	},
	"read_errors_corrected_by_eccfast": {
		ID:          "read_errors_corrected_by_eccfast",
		DisplayName: "Read Errors Corrected by ECC Fast",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "An error correction was applied to get perfect data (a.k.a. ECC on-the-fly). \"Without substantial delay\" means the correction did not postpone reading of later sectors (e.g. a revolution was not lost). The counter is incremented once for each logical block that requires correction. Two different blocks corrected during the same command are counted as two events.",
	},
	"read_errors_corrected_by_eccdelayed": {
		ID:          "read_errors_corrected_by_eccdelayed",
		DisplayName: "Read Errors Corrected by ECC Delayed",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "An error code or algorithm (e.g. ECC, checksum) is applied in order to get perfect data with substantial delay. \"With possible delay\" means the correction took longer than a sector time so that reading/writing of subsequent sectors was delayed (e.g. a lost revolution). The counter is incremented once for each logical block that requires correction. A block with a double error that is correctable counts as one event and two different blocks corrected during the same command count as two events. ",
	},
	"read_errors_corrected_by_rereads_rewrites": {
		ID:          "read_errors_corrected_by_rereads_rewrites",
		DisplayName: "Read Errors Corrected by ReReads/ReWrites",
		DisplayType: "",
		Ideal:       "low",
		Critical:    true,
		Description: "This parameter code specifies the counter counting the number of errors that are corrected by applying retries. This counts errors recovered, not the number of retries. If five retries were required to recover one block of data, the counter increments by one, not five. The counter is incremented once for each logical block that is recovered using retries. If an error is not recoverable while applying retries and is recovered by ECC, it isn't counted by this counter; it will be counted by the counter specified by parameter code 01h - Errors Corrected With Possible Delays. ",
	},
	"read_total_errors_corrected": {
		ID:          "read_total_errors_corrected",
		DisplayName: "Read Total Errors Corrected",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "This counter counts the total of parameter code errors 00h, 01h and 02h (i.e. error corrected by ECC: fast and delayed plus errors corrected by rereads and rewrites). There is no \"double counting\" of data errors among these three counters. The sum of all correctable errors can be reached by adding parameter code 01h and 02h errors, not by using this total.",
	},
	"read_correction_algorithm_invocations": {
		ID:          "read_correction_algorithm_invocations",
		DisplayName: "Read Correction Algorithm Invocations",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "This parameter code specifies the counter that counts the total number of retries, or \"times the retry algorithm is invoked\". If after five attempts a counter 02h type error is recovered, then five is added to this counter. If three retries are required to get stable ECC syndrome before a counter 01h type error is corrected, then those three retries are also counted here. The number of retries applied to unsuccessfully recover an error (counter 06h type error) are also counted by this counter. ",
	},
	"read_total_uncorrected_errors": {
		ID:          "read_total_uncorrected_errors",
		DisplayName: "Read Total Uncorrected Errors",
		DisplayType: "",
		Ideal:       "low",
		Critical:    true,
		Description: "This parameter code specifies the counter that contains the total number of blocks for which an uncorrected data error has occurred. ",
	},
	"write_errors_corrected_by_eccfast": {
		ID:          "write_errors_corrected_by_eccfast",
		DisplayName: "Write Errors Corrected by ECC Fast",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "An error correction was applied to get perfect data (a.k.a. ECC on-the-fly). \"Without substantial delay\" means the correction did not postpone reading of later sectors (e.g. a revolution was not lost). The counter is incremented once for each logical block that requires correction. Two different blocks corrected during the same command are counted as two events. ",
	},
	"write_errors_corrected_by_eccdelayed": {
		ID:          "write_errors_corrected_by_eccdelayed",
		DisplayName: "Write Errors Corrected by ECC Delayed",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "An error code or algorithm (e.g. ECC, checksum) is applied in order to get perfect data with substantial delay. \"With possible delay\" means the correction took longer than a sector time so that reading/writing of subsequent sectors was delayed (e.g. a lost revolution). The counter is incremented once for each logical block that requires correction. A block with a double error that is correctable counts as one event and two different blocks corrected during the same command count as two events. ",
	},
	"write_errors_corrected_by_rereads_rewrites": {
		ID:          "write_errors_corrected_by_rereads_rewrites",
		DisplayName: "Write Errors Corrected by ReReads/ReWrites",
		DisplayType: "",
		Ideal:       "low",
		Critical:    true,
		Description: "This parameter code specifies the counter counting the number of errors that are corrected by applying retries. This counts errors recovered, not the number of retries. If five retries were required to recover one block of data, the counter increments by one, not five. The counter is incremented once for each logical block that is recovered using retries. If an error is not recoverable while applying retries and is recovered by ECC, it isn't counted by this counter; it will be counted by the counter specified by parameter code 01h - Errors Corrected With Possible Delays.",
	},
	"write_total_errors_corrected": {
		ID:          "write_total_errors_corrected",
		DisplayName: "Write Total Errors Corrected",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "This counter counts the total of parameter code errors 00h, 01h and 02h (i.e. error corrected by ECC: fast and delayed plus errors corrected by rereads and rewrites). There is no \"double counting\" of data errors among these three counters. The sum of all correctable errors can be reached by adding parameter code 01h and 02h errors, not by using this total.",
	},
	"write_correction_algorithm_invocations": {
		ID:          "write_correction_algorithm_invocations",
		DisplayName: "Write Correction Algorithm Invocations",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "This parameter code specifies the counter that counts the total number of retries, or \"times the retry algorithm is invoked\". If after five attempts a counter 02h type error is recovered, then five is added to this counter. If three retries are required to get stable ECC syndrome before a counter 01h type error is corrected, then those three retries are also counted here. The number of retries applied to unsuccessfully recover an error (counter 06h type error) are also counted by this counter. ",
	},
	"write_total_uncorrected_errors": {
		ID:          "write_total_uncorrected_errors",
		DisplayName: "Write Total Uncorrected Errors",
		DisplayType: "",
		Ideal:       "low",
		Critical:    true,
		Description: " This parameter code specifies the counter that contains the total number of blocks for which an uncorrected data error has occurred.",
	},
	"temperature": {
		ID:          "temperature",
		DisplayName: "Temperature",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Current drive temperature in Celsius.",
	},
	"read_gigabytes_processed": {
		ID:          "read_gigabytes_processed",
		DisplayName: "Data Read",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Cumulative amount of data processed by read commands, taken from the SCSI Read Error Counter log page's \"gigabytes processed\" field. This is the SAS/SCSI equivalent of ATA LBAs Read or NVMe Data Units Read, and is the basis for workload read-rate estimates on SAS drives. Note this counts all data processed (including retries), not just unique host data, and is reported at ~1 GB granularity.",
	},
	"write_gigabytes_processed": {
		ID:          "write_gigabytes_processed",
		DisplayName: "Data Written",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Cumulative amount of data processed by write commands, taken from the SCSI Write Error Counter log page's \"gigabytes processed\" field. This is the SAS/SCSI equivalent of ATA LBAs Written or NVMe Data Units Written, and is the basis for workload write-rate and endurance (TBW) estimates on SAS drives. Note this counts all data processed (including retries), not just unique host data, and is reported at ~1 GB granularity.",
	},
	"verify_errors_corrected_by_eccfast": {
		ID:          "verify_errors_corrected_by_eccfast",
		DisplayName: "Verify Errors Corrected by ECC Fast",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "An error correction was applied during a verify operation to get perfect data (a.k.a. ECC on-the-fly). \"Without substantial delay\" means the correction did not postpone reading of later sectors. The counter is incremented once for each logical block that requires correction. Verify counters come from the Verify Error Counter log page (0x05), which records errors during VERIFY operations: media checks that read a block back without transferring it to the host. Some drives also fold their background medium scan activity into this page, so the counters can move with no host I/O at all.",
	},
	"verify_errors_corrected_by_eccdelayed": {
		ID:          "verify_errors_corrected_by_eccdelayed",
		DisplayName: "Verify Errors Corrected by ECC Delayed",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "An error code or algorithm (e.g. ECC, checksum) was applied during a verify operation to get perfect data with substantial delay. \"With possible delay\" means the correction took longer than a sector time so that reading of subsequent sectors was delayed (e.g. a lost revolution). The counter is incremented once for each logical block that requires correction.",
	},
	"verify_errors_corrected_by_rereads_rewrites": {
		ID:                 "verify_errors_corrected_by_rereads_rewrites",
		UseDeltaEvaluation: true,
		DisplayName:        "Verify Errors Corrected by ReReads/ReWrites",
		DisplayType:        "",
		Ideal:              "low",
		Critical:           true,
		Description:        "The number of verify errors recovered by applying retries. This counts errors recovered, not the number of retries: if five retries were required to recover one block, the counter increments by one, not five. A rising count means the drive needed more than one attempt to read a block back, so the media is degrading where verify operations reach - on drives that fold background medium scanning into this page, that includes areas the host never reads.",
	},
	"verify_total_errors_corrected": {
		ID:          "verify_total_errors_corrected",
		DisplayName: "Verify Total Errors Corrected",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "The total of the ECC fast, ECC delayed, and rereads/rewrites verify counters (parameter codes 00h, 01h and 02h). There is no \"double counting\" of data errors among those three counters.",
	},
	"verify_correction_algorithm_invocations": {
		ID:          "verify_correction_algorithm_invocations",
		DisplayName: "Verify Correction Algorithm Invocations",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "The total number of times the retry algorithm was invoked during verify operations, counting every attempt rather than every recovered block. Retries applied to errors that were never recovered are counted here as well.",
	},
	"verify_total_uncorrected_errors": {
		ID:          "verify_total_uncorrected_errors",
		DisplayName: "Verify Total Uncorrected Errors",
		DisplayType: "",
		Ideal:       "low",
		Critical:    true,
		Description: "The total number of blocks that failed a verify operation with an uncorrected data error. These are unreadable blocks the drive found on its own, so a nonzero count can precede a host-visible read failure.",
	},
	"verify_gigabytes_processed": {
		ID:          "verify_gigabytes_processed",
		DisplayName: "Data Verified",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Cumulative amount of data processed by verify commands, taken from the SCSI Verify Error Counter log page's \"gigabytes processed\" field. It typically covers only host-issued verify commands, so a drive can report 0 here while its verify error counters still move, and VERIFY is rare in normal use. It is reported at ~1 GB granularity and is not part of host workload (read/write) totals.",
	},
	"background_scans_performed": {
		ID:          "background_scans_performed",
		DisplayName: "Background Scans Performed",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Number of background scans the drive has completed, from the Background Medium Scan status (log page 0x15). The drive runs this scan on its own schedule, independently of Scrutiny's self-tests, so this count is how you see whether it is still checking its own media. A count that stops rising means background scanning is disabled or never completing. The errors a scan finds are counted by the verify error counters, not here.",
	},
	"background_medium_scans_performed": {
		ID:          "background_medium_scans_performed",
		DisplayName: "Background Medium Scans Performed",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Number of background medium scans the drive has completed, from the Background Medium Scan status (log page 0x15). A medium scan reads the media itself, as opposed to the pre-scan the drive performs after a format, so on most drives this count tracks the total scan count closely.",
	},
	"background_scan_status": {
		ID:          "background_scan_status",
		DisplayName: "Background Scan Status",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Current state of the drive's background medium scan as a numeric status code from log page 0x15 (for example 8, waiting for the scan interval timer to expire). It is a state, not a measurement: a change in value means the drive moved between waiting, scanning, suspended and halted.",
	},
	"background_scan_progress_percent": {
		ID:                 "background_scan_progress_percent",
		DisplayName:        "Background Scan Progress",
		DisplayType:        "",
		Ideal:              "",
		Critical:           false,
		TransformValueUnit: "%",
		Description:        "How far the drive is through its current background medium scan, in whole percent. It only means anything while a scan is running; between scans the drive reports 0.",
	},
	"start_stop_cycles": {
		ID:          "start_stop_cycles",
		DisplayName: "Start/Stop Cycles",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Number of times the drive's motor has spun up and down, from the Start-Stop Cycle Counter log page (0x0e). The threshold shown beside it is the manufacturer's rated count over the drive's lifetime, read from the same log page, so the pair is a mechanical wear figure. Passing the rating is not a failure and is not reported as one: a drive regularly outlives its rated cycle count, and the ATA equivalents of this counter are not failed either. Set a fail_above override on this attribute to be told when a drive passes a count you choose.",
	},
	"load_unload_cycles": {
		ID:          "load_unload_cycles",
		DisplayName: "Load/Unload Cycles",
		DisplayType: "",
		Ideal:       "",
		Critical:    false,
		Description: "Number of times the drive's heads have been loaded onto and unloaded from the platters, from the Start-Stop Cycle Counter log page (0x0e). Head loading happens far more often than a full spin-up, which is why drives are rated for an order of magnitude more of these cycles. The threshold shown beside it is the manufacturer's rated count over the drive's lifetime; as with start/stop cycles, passing it is not reported as a failure.",
	},
	"percentage_used": {
		ID:          "percentage_used",
		DisplayName: "Percentage Used",
		DisplayType: "",
		Ideal:       "low",
		Critical:    true,
		Description: "SAS SSD wear indicator from the Solid State Media log page's \"Percentage used endurance indicator\". Reaches 100% at the manufacturer's rated endurance limit, but the drive may continue to operate beyond that point. Only reported by solid-state SAS drives.",
	},
}

// scsiSasPhyMetadataTemplates holds the display metadata of each per-PHY SAS attribute,
// keyed by the attribute ID suffix. The full attribute ID carries the port and PHY numbers
// of the drive that reported them (e.g. "sas_port_1_phy_0_invalid_dword_count"), which are
// only known at collection time, so ScsiMetadataFor fills the numbers in rather than this
// package enumerating every port and PHY a drive might have.
var scsiSasPhyMetadataTemplates = map[string]ScsiAttributeMetadata{
	"invalid_dword_count": {
		DisplayName:        "Invalid DWORDs",
		Ideal:              "low",
		Critical:           true,
		UseDeltaEvaluation: true,
		Description:        "Count of DWORDs received on this PHY that were not valid, from the SAS Protocol log page. Invalid DWORDs point at the link rather than the media: a marginal cable, a dirty or worn connector, a backplane slot, or an HBA port. The counter is cumulative and never resets, so a nonzero value may be years old; what matters is whether it climbs between collections.",
	},
	"running_disparity_error_count": {
		DisplayName:        "Running Disparity Errors",
		Ideal:              "low",
		Critical:           true,
		UseDeltaEvaluation: true,
		Description:        "Count of 8b/10b running-disparity errors on this PHY, from the SAS Protocol log page. Like invalid DWORDs these are signal-integrity errors on the link, not media errors, and they are cumulative: a rising count indicates an active cabling, backplane or HBA problem.",
	},
	"loss_of_dword_synchronization_count": {
		DisplayName:        "Loss of DWORD Synchronization",
		Ideal:              "low",
		Critical:           true,
		UseDeltaEvaluation: true,
		Description:        "Count of times this PHY lost DWORD synchronization and had to re-establish it, from the SAS Protocol log page. Each event interrupts the link briefly. A few are normal across a drive's life, often from power cycles and reseats; a rising count means the link is dropping out now.",
	},
	"phy_reset_problem_count": {
		DisplayName:        "PHY Reset Problems",
		Ideal:              "low",
		Critical:           true,
		UseDeltaEvaluation: true,
		Description:        "Count of times this PHY failed to complete a reset sequence, from the SAS Protocol log page. The counter is cumulative, so a nonzero value can date from installation; a rising count means the PHY is struggling to train the link.",
	},
	"negotiated_link_rate_mbps": {
		DisplayName:        "Negotiated Link Rate, Mbps",
		Ideal:              "high",
		Critical:           false,
		TransformValueUnit: "Mbps",
		Description:        "Speed this PHY negotiated with whatever is attached to it, in megabits per second (12000 Mbps = 12 Gbps). It is reported for history rather than judged against a threshold, because the ceiling depends on the drive, the HBA and the cabling. A drop from a previously higher rate is the signal: a 12 Gbps link renegotiating at 6 Gbps usually means a marginal cable or connector. PHYs that report no speed, such as a port with nothing attached, get no attribute.",
	},
}

// scsiSasPhyAttributePattern matches a per-PHY SAS attribute ID, capturing the port number,
// the PHY number and the counter suffix.
var scsiSasPhyAttributePattern = regexp.MustCompile(`^sas_port_([0-9]+)_phy_([0-9]+)_(.+)$`)

// scsiEnvironmentalMetadataTemplates holds the display metadata of the per-sensor lifetime
// temperature attributes, keyed by the attribute ID suffix. The full attribute ID carries the
// sensor key smartctl reported ("temperature_1_lifetime_maximum"), and a drive can report
// several sensors, so ScsiMetadataFor fills the sensor number in.
var scsiEnvironmentalMetadataTemplates = map[string]ScsiAttributeMetadata{
	"lifetime_maximum": {
		DisplayName: "Lifetime Maximum Temperature",
		Description: "Highest temperature this sensor has ever recorded, from the Environmental Reports log page. The drive tracks it itself, so it covers the whole life of the drive rather than the period Scrutiny has been watching it, and it survives any gap in collection. A value far above the drive's rated maximum points at past cooling or airflow problems even when the drive is cool now.",
	},
	"lifetime_minimum": {
		DisplayName: "Lifetime Minimum Temperature",
		Description: "Lowest temperature this sensor has ever recorded, from the Environmental Reports log page. Useful mainly as evidence the drive has been powered on below its rated operating range, for example in an unheated space.",
	},
	"maximum_since_power_on": {
		DisplayName: "Maximum Temperature Since Power On",
		Description: "Highest temperature this sensor has recorded since the drive was last powered on, from the Environmental Reports log page. Compared against the current temperature it shows whether the drive has run hotter earlier in this power cycle.",
	},
	"minimum_since_power_on": {
		DisplayName: "Minimum Temperature Since Power On",
		Description: "Lowest temperature this sensor has recorded since the drive was last powered on, from the Environmental Reports log page.",
	},
}

// scsiEnvironmentalAttributePattern matches a per-sensor lifetime temperature attribute ID,
// capturing the sensor number and the suffix.
var scsiEnvironmentalAttributePattern = regexp.MustCompile(`^temperature_([0-9]+)_(.+)$`)

// ScsiMetadataFor resolves the display metadata of a SCSI attribute ID. Prefer it over
// indexing ScsiMetadata directly: the per-PHY SAS counters and the per-sensor lifetime
// temperatures are not in that map, because their IDs depend on how many ports, PHYs and
// temperature sensors the drive reports.
func ScsiMetadataFor(attributeID string) (ScsiAttributeMetadata, bool) {
	if metadata, ok := ScsiMetadata[attributeID]; ok {
		return metadata, true
	}

	if match := scsiSasPhyAttributePattern.FindStringSubmatch(attributeID); match != nil {
		if metadata, ok := scsiSasPhyMetadataTemplates[match[3]]; ok {
			metadata.ID = attributeID
			metadata.DisplayName = fmt.Sprintf("%s (Port %s PHY %s)", metadata.DisplayName, match[1], match[2])
			return metadata, true
		}
	}

	if match := scsiEnvironmentalAttributePattern.FindStringSubmatch(attributeID); match != nil {
		if metadata, ok := scsiEnvironmentalMetadataTemplates[match[2]]; ok {
			metadata.ID = attributeID
			// Always name the sensor. Suppressing it for sensor 1 would make the first row of a
			// multi-sensor drive read as a drive-wide figure, and this function sees one
			// attribute ID at a time, so it cannot tell how many sensors the drive reported.
			metadata.DisplayName = fmt.Sprintf("%s (Sensor %s)", metadata.DisplayName, match[1])
			return metadata, true
		}
	}

	return ScsiAttributeMetadata{}, false
}

// ScsiMetadataForAttributes returns the metadata map to send to the UI for one device: every
// statically known SCSI attribute, plus a resolved entry for each of the device's own
// attribute IDs that is not in the static map (the per-PHY SAS counters and the per-sensor
// lifetime temperatures). Without the resolved entries the UI has no display name or
// description for them.
func ScsiMetadataForAttributes(attributeIDs []string) map[string]ScsiAttributeMetadata {
	metadataByID := make(map[string]ScsiAttributeMetadata, len(ScsiMetadata)+len(attributeIDs))
	for attributeID, metadata := range ScsiMetadata {
		metadataByID[attributeID] = metadata
	}
	for _, attributeID := range attributeIDs {
		if _, ok := metadataByID[attributeID]; ok {
			continue
		}
		if metadata, ok := ScsiMetadataFor(attributeID); ok {
			metadataByID[attributeID] = metadata
		}
	}
	return metadataByID
}
