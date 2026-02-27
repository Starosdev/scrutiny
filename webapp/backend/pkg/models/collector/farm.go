package collector

// SeagateFarmLog represents the Seagate FARM (Field Accessible Reliability Metrics) log
// from smartctl -l farm --json output. The top-level JSON key is "seagate_farm_log".
type SeagateFarmLog struct {
	LogHeader *FarmLogHeader             `json:"page_0_log_header,omitempty"`
	DriveInfo *FarmDriveInformation      `json:"page_1_drive_information,omitempty"`
	Workload  *FarmWorkloadStatistics    `json:"page_2_workload_statistics,omitempty"`
	Errors    *FarmErrorStatistics       `json:"page_3_error_statistics,omitempty"`
	Environ   *FarmEnvironmentStatistics `json:"page_4_environment_statistics,omitempty"`
	Reliab    *FarmReliabilityStatistics `json:"page_5_reliability_statistics,omitempty"`
	Supported bool                       `json:"supported"`
}

type FarmLogHeader struct {
	FarmLogVersion []int `json:"farm_log_version"`
	PagesSupported int   `json:"pages_supported"`
	LogSize        int64 `json:"log_size"`
	PageSize       int64 `json:"page_size"`
	HeadsSupported int   `json:"heads_supported"`
}

type FarmDriveInformation struct {
	SerialNumber            string `json:"serial_number"`
	WorldWideName           string `json:"world_wide_name"`
	DeviceInterface         string `json:"device_interface"`
	FormFactor              string `json:"form_factor"`
	FirmwareRev             string `json:"firmware_rev"`
	DriveRecordingType      string `json:"drive_recording_type"`
	DateOfAssembly          string `json:"date_of_assembly"`
	DeviceCapacityInSectors int64  `json:"device_capacity_in_sectors"`
	PhysicalSectorSize      int64  `json:"physical_sector_size"`
	LogicalSectorSize       int64  `json:"logical_sector_size"`
	Poh                     int64  `json:"poh"`
	Spoh                    int64  `json:"spoh"`
	HeadFlightHours         int64  `json:"head_flight_hours"`
	HeadLoadEvents          int64  `json:"head_load_events"`
	PowerCycleCount         int64  `json:"power_cycle_count"`
	ResetCount              int64  `json:"reset_count"`
	SpinUpTime              int64  `json:"spin_up_time"`
	NumberOfHeads           int    `json:"number_of_heads"`
	RotationRate            int    `json:"rotation_rate"`
}

type FarmWorkloadStatistics struct {
	TotalReadCommands     int64 `json:"total_read_commands"`
	TotalWriteCommands    int64 `json:"total_write_commands"`
	TotalRandomReads      int64 `json:"total_random_reads"`
	TotalRandomWrites     int64 `json:"total_random_writes"`
	LogicalSectorsWritten int64 `json:"logical_sectors_written"`
	LogicalSectorsRead    int64 `json:"logical_sectors_read"`
}

type FarmErrorStatistics struct {
	NumberOfUnrecoverableReadErrors     int64 `json:"number_of_unrecoverable_read_errors"`
	NumberOfUnrecoverableWriteErrors    int64 `json:"number_of_unrecoverable_write_errors"`
	NumberOfReallocatedSectors          int64 `json:"number_of_reallocated_sectors"`
	NumberOfReadRecoveryAttempts        int64 `json:"number_of_read_recovery_attempts"`
	NumberOfMechanicalStartFailures     int64 `json:"number_of_mechanical_start_failures"`
	NumberOfReallocatedCandidateSectors int64 `json:"number_of_reallocated_candidate_sectors"`
	TotalCrcErrors                      int64 `json:"total_crc_errors"`
	CommandTimeOutCountTotal            int64 `json:"command_time_out_count_total"`
	NumberOfIoedcErrors                 int64 `json:"number_of_ioedc_errors"`
	TotalFlashLedErrors                 int64 `json:"total_flash_led_errors"`
}

// FarmEnvironmentStatistics contains environmental metrics.
// Note: the temperature field name has a known typo in the smartctl JSON output.
type FarmEnvironmentStatistics struct {
	CurentTemp  int64 `json:"curent_temp"` //nolint:misspell // smartctl uses this spelling
	HighestTemp int64 `json:"highest_temp"`
	LowestTemp  int64 `json:"lowest_temp"`
	AverageTemp int64 `json:"average_temp"`
	MaxTemp     int64 `json:"max_temp"`
	MinTemp     int64 `json:"min_temp"`
	Humidity    int64 `json:"humidity"`
}

type FarmReliabilityStatistics struct {
	ErrorRateNormalized      int64 `json:"error_rate_normalized"`
	ErrorRateWorst           int64 `json:"error_rate_worst"`
	SeekErrorRateNormalized  int64 `json:"seek_error_rate_normalized"`
	SeekErrorRateWorst       int64 `json:"seek_error_rate_worst"`
	HighPriorityUnloadEvents int64 `json:"high_priority_unload_events"`
}
