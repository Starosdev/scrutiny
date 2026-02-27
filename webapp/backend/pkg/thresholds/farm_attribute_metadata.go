package thresholds

// FarmMetadata defines display names, thresholds, and criticality for Seagate FARM
// (Field Accessible Reliability Metrics) attributes. Uses the same metadata struct
// as ATA Device Statistics for consistency.
var FarmMetadata = map[string]AtaDeviceStatisticsMetadata{
	// Page 1: Drive Information
	"farm_poh": {
		DisplayName: "Power-On Hours (FARM)",
		Description: "Tamper-proof power-on hours from Seagate FARM log. Cannot be reset by firmware.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_spoh": {
		DisplayName: "Spindle Power-On Hours",
		Description: "Total hours the spindle motor has been powered on.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_head_flight_hours": {
		DisplayName: "Head Flight Hours",
		Description: "Total hours the read/write heads have been in flight over the platters.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_head_load_events": {
		DisplayName: "Head Load Events",
		Ideal:       ObservedThresholdIdealLow,
		Description: "Number of times the read/write heads have been loaded onto the platters.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_power_cycle_count": {
		DisplayName: "Power Cycle Count (FARM)",
		Description: "Number of power on/off cycles from FARM log.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},

	// Page 2: Workload Statistics
	"farm_total_read_commands": {
		DisplayName: "Total Read Commands",
		Description: "Total number of read commands issued to the drive.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_total_write_commands": {
		DisplayName: "Total Write Commands",
		Description: "Total number of write commands issued to the drive.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_logical_sectors_written": {
		DisplayName: "Logical Sectors Written",
		Description: "Total number of logical sectors written to the drive.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_logical_sectors_read": {
		DisplayName: "Logical Sectors Read",
		Description: "Total number of logical sectors read from the drive.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},

	// Page 3: Error Statistics
	"farm_unrecoverable_read_errors": {
		DisplayName: "Unrecoverable Read Errors (FARM)",
		Ideal:       ObservedThresholdIdealLow,
		Critical:    true,
		Description: "Number of read errors that could not be recovered by the drive's error correction.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_unrecoverable_write_errors": {
		DisplayName: "Unrecoverable Write Errors (FARM)",
		Ideal:       ObservedThresholdIdealLow,
		Critical:    true,
		Description: "Number of write errors that could not be recovered by the drive's error correction.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_reallocated_sectors": {
		DisplayName: "Reallocated Sectors (FARM)",
		Ideal:       ObservedThresholdIdealLow,
		Critical:    true,
		Description: "Number of sectors that have been remapped to spare sectors from the FARM log.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_reallocation_candidates": {
		DisplayName: "Reallocation Candidates",
		Ideal:       ObservedThresholdIdealLow,
		Critical:    true,
		Description: "Number of sectors pending reallocation. These are sectors the drive has identified as potentially failing.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_crc_errors": {
		DisplayName: "CRC Errors (FARM)",
		Ideal:       ObservedThresholdIdealLow,
		Critical:    true,
		Description: "Total number of CRC (Cyclic Redundancy Check) errors, typically indicating interface/cable issues.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_command_timeouts": {
		DisplayName: "Command Timeouts",
		Ideal:       ObservedThresholdIdealLow,
		Description: "Total number of commands that timed out.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},

	// Page 4: Environment Statistics
	"farm_current_temperature": {
		DisplayName: "Temperature (FARM)",
		Ideal:       ObservedThresholdIdealLow,
		Description: "Current drive temperature in Celsius from FARM log.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_highest_temperature": {
		DisplayName: "Highest Temperature",
		Ideal:       ObservedThresholdIdealLow,
		Description: "Highest temperature ever recorded by the drive.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
	"farm_lowest_temperature": {
		DisplayName: "Lowest Temperature",
		Description: "Lowest temperature ever recorded by the drive.",
		DisplayType: AtaSmartAttributeDisplayTypeRaw,
	},
}
