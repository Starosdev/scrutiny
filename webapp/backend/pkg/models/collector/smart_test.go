package collector

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmartInfo_Capacity(t *testing.T) {
	t.Run("should report nvme capacity", func(t *testing.T) {
		smartInfo := SmartInfo{
			UserCapacity: UserCapacity{
				Bytes: 1234,
			},
			NvmeTotalCapacity: 5678,
		}
		assert.Equal(t, int64(5678), smartInfo.Capacity())
	})

	t.Run("should report user capacity", func(t *testing.T) {
		smartInfo := SmartInfo{
			UserCapacity: UserCapacity{
				Bytes: 1234,
			},
		}
		assert.Equal(t, int64(1234), smartInfo.Capacity())
	})

	t.Run("should report 0 for unknown capacities", func(t *testing.T) {
		var smartInfo SmartInfo
		assert.Zero(t, smartInfo.Capacity())
	})
}

func TestSmartInfo_Firmware(t *testing.T) {
	t.Run("should report firmware_version when present (ATA/NVMe)", func(t *testing.T) {
		smartInfo := SmartInfo{
			FirmwareVersion: "0004",
			ScsiRevision:    "HPD5",
		}
		assert.Equal(t, "0004", smartInfo.Firmware())
	})

	t.Run("should fall back to scsi_revision when firmware_version is empty (SCSI/SAS)", func(t *testing.T) {
		smartInfo := SmartInfo{
			ScsiRevision: "HPD5",
		}
		assert.Equal(t, "HPD5", smartInfo.Firmware())
	})

	t.Run("should report empty string when neither field is present", func(t *testing.T) {
		var smartInfo SmartInfo
		assert.Equal(t, "", smartInfo.Firmware())
	})
}

func TestSmartInfo_LargeLBAValues(t *testing.T) {
	// Test for GitHub issue #24 / upstream issue #800
	// LBA values can be large unsigned 64-bit integers that overflow signed int
	t.Run("should parse large LBA values in selective self-test log", func(t *testing.T) {
		// This JSON contains LBA values that exceed int64 max (9223372036854775807)
		// Value 18446743534724713985 is a valid uint64 but overflows int64
		jsonData := `{
			"ata_smart_selective_self_test_log": {
				"revision": 1,
				"table": [
					{
						"lba_min": 18446743534724713985,
						"lba_max": 7205816247684983039,
						"status": {
							"value": 0,
							"string": "Not_testing"
						}
					}
				],
				"flags": {
					"value": 0,
					"remainder_scan_enabled": false
				},
				"power_up_scan_resume_minutes": 0
			}
		}`

		var smartInfo SmartInfo
		err := json.Unmarshal([]byte(jsonData), &smartInfo)
		require.NoError(t, err, "should unmarshal large LBA values without error")

		// Verify the values were parsed correctly
		require.Len(t, smartInfo.AtaSmartSelectiveSelfTestLog.Table, 1)
		assert.Equal(t, uint64(18446743534724713985), smartInfo.AtaSmartSelectiveSelfTestLog.Table[0].LbaMin)
		assert.Equal(t, uint64(7205816247684983039), smartInfo.AtaSmartSelectiveSelfTestLog.Table[0].LbaMax)
	})

	t.Run("should parse large LBA values in error log", func(t *testing.T) {
		// LBA values in error logs can also be large
		jsonData := `{
			"ata_smart_error_log": {
				"summary": {
					"revision": 1,
					"count": 1,
					"logged_count": 1,
					"table": [
						{
							"error_number": 1,
							"lifetime_hours": 1000,
							"completion_registers": {
								"error": 0,
								"status": 0,
								"count": 0,
								"lba": 18446744073709551615,
								"device": 0
							},
							"error_description": "test",
							"previous_commands": [
								{
									"registers": {
										"command": 0,
										"features": 0,
										"count": 0,
										"lba": 18446744073709551615,
										"device": 0,
										"device_control": 0
									},
									"powerup_milliseconds": 0,
									"command_name": "test"
								}
							]
						}
					]
				}
			}
		}`

		var smartInfo SmartInfo
		err := json.Unmarshal([]byte(jsonData), &smartInfo)
		require.NoError(t, err, "should unmarshal large LBA values in error log without error")

		// Verify the values were parsed correctly
		require.Len(t, smartInfo.AtaSmartErrorLog.Summary.Table, 1)
		assert.Equal(t, uint64(18446744073709551615), smartInfo.AtaSmartErrorLog.Summary.Table[0].CompletionRegisters.Lba)
		require.Len(t, smartInfo.AtaSmartErrorLog.Summary.Table[0].PreviousCommands, 1)
		assert.Equal(t, uint64(18446744073709551615), smartInfo.AtaSmartErrorLog.Summary.Table[0].PreviousCommands[0].Registers.Lba)
	})
}

func TestSmartInfo_AtaSmartSelfTestLogEntries(t *testing.T) {
	t.Run("should prefer standard self-test entries when present", func(t *testing.T) {
		jsonData := `{
			"ata_smart_self_test_log": {
				"standard": {
					"revision": 1,
					"table": [
						{
							"type": { "value": 1, "string": "Short offline" },
							"status": { "value": 0, "string": "Completed without error", "passed": true },
							"lifetime_hours": 100
						}
					]
				}
			}
		}`

		var smartInfo SmartInfo
		err := json.Unmarshal([]byte(jsonData), &smartInfo)
		require.NoError(t, err)
		require.Len(t, smartInfo.AtaSmartSelfTestLog.Entries(), 1)
		assert.Equal(t, 100, smartInfo.AtaSmartSelfTestLog.Entries()[0].LifetimeHours)
	})

	t.Run("should fall back to extended self-test entries", func(t *testing.T) {
		jsonData := `{
			"ata_smart_self_test_log": {
				"extended": {
					"revision": 1,
					"sectors": 1,
					"table": [
						{
							"type": { "value": 2, "string": "Extended offline" },
							"status": { "value": 0, "string": "Completed without error", "passed": true },
							"lifetime_hours": 200
						}
					]
				}
			}
		}`

		var smartInfo SmartInfo
		err := json.Unmarshal([]byte(jsonData), &smartInfo)
		require.NoError(t, err)
		require.Len(t, smartInfo.AtaSmartSelfTestLog.Entries(), 1)
		assert.Equal(t, 2, smartInfo.AtaSmartSelfTestLog.Entries()[0].Type.Value)
		assert.Equal(t, 200, smartInfo.AtaSmartSelfTestLog.Entries()[0].LifetimeHours)
	})
}

func TestSmartInfo_ScsiSelfTestEntries(t *testing.T) {
	t.Run("should parse numbered scsi_self_test_N keys in ascending order", func(t *testing.T) {
		jsonData := `{
			"device": { "protocol": "SCSI" },
			"scsi_self_test_0": {
				"code": { "value": 1, "string": "Background short" },
				"result": { "value": 0, "string": "Completed" },
				"power_on_time": { "hours": 48239 }
			},
			"scsi_self_test_1": {
				"code": { "value": 2, "string": "Background long" },
				"result": { "value": 0, "string": "Completed" },
				"power_on_time": { "hours": 48234 }
			},
			"scsi_self_test_10": {
				"code": { "value": 1, "string": "Background short" },
				"result": { "value": 3, "string": "Aborted by host" },
				"power_on_time": { "hours": 100 }
			}
		}`

		var smartInfo SmartInfo
		err := json.Unmarshal([]byte(jsonData), &smartInfo)
		require.NoError(t, err)
		require.Len(t, smartInfo.ScsiSelfTests, 3)
		// numeric sort, not lexicographic ("scsi_self_test_10" must sort after "_1", not before it)
		assert.Equal(t, 1, smartInfo.ScsiSelfTests[0].Code.Value)
		assert.Equal(t, 48239, smartInfo.ScsiSelfTests[0].PowerOnTime.Hours)
		assert.Equal(t, 2, smartInfo.ScsiSelfTests[1].Code.Value)
		assert.Equal(t, 48234, smartInfo.ScsiSelfTests[1].PowerOnTime.Hours)
		assert.Equal(t, 100, smartInfo.ScsiSelfTests[2].PowerOnTime.Hours)
	})

	t.Run("should not confuse scsi_background_scan or other keys with self-test entries", func(t *testing.T) {
		jsonData := `{
			"device": { "protocol": "SCSI" },
			"scsi_background_scan": {
				"status": { "value": 1, "string": "scan is active" }
			}
		}`

		var smartInfo SmartInfo
		err := json.Unmarshal([]byte(jsonData), &smartInfo)
		require.NoError(t, err)
		require.Empty(t, smartInfo.ScsiSelfTests)
		require.NotNil(t, smartInfo.ScsiBackgroundScan)
		assert.Equal(t, "scan is active", smartInfo.ScsiBackgroundScan.Status.String)
	})
}

func TestSmartInfo_SelfTestEntries(t *testing.T) {
	t.Run("should return ATA entries as-is when present", func(t *testing.T) {
		jsonData := `{
			"device": { "protocol": "ATA" },
			"ata_smart_self_test_log": {
				"standard": {
					"revision": 1,
					"table": [
						{
							"type": { "value": 1, "string": "Short offline" },
							"status": { "value": 0, "string": "Completed without error", "passed": true },
							"lifetime_hours": 100
						}
					]
				}
			}
		}`
		var smartInfo SmartInfo
		require.NoError(t, json.Unmarshal([]byte(jsonData), &smartInfo))

		entries := smartInfo.SelfTestEntries()
		require.Len(t, entries, 1)
		assert.Equal(t, 100, entries[0].LifetimeHours)
		assert.True(t, entries[0].Status.Passed)
	})

	t.Run("should normalize scsi self-test entries", func(t *testing.T) {
		jsonData := `{
			"device": { "protocol": "SCSI" },
			"scsi_self_test_0": {
				"code": { "value": 1, "string": "Background short" },
				"result": { "value": 0, "string": "Completed" },
				"power_on_time": { "hours": 48239 }
			},
			"scsi_self_test_1": {
				"code": { "value": 1, "string": "Background short" },
				"result": { "value": 3, "string": "Aborted by host" },
				"power_on_time": { "hours": 100 }
			}
		}`
		var smartInfo SmartInfo
		require.NoError(t, json.Unmarshal([]byte(jsonData), &smartInfo))

		entries := smartInfo.SelfTestEntries()
		require.Len(t, entries, 2)

		assert.Equal(t, 1, entries[0].Type.Value)
		assert.Equal(t, "Background short", entries[0].Type.String)
		assert.Equal(t, 0, entries[0].Status.Value)
		assert.True(t, entries[0].Status.Passed)
		assert.Equal(t, 48239, entries[0].LifetimeHours)

		assert.Equal(t, 3, entries[1].Status.Value)
		assert.False(t, entries[1].Status.Passed)
		assert.Equal(t, 100, entries[1].LifetimeHours)
	})

	t.Run("should return nil when no self-test data is present", func(t *testing.T) {
		var smartInfo SmartInfo
		assert.Nil(t, smartInfo.SelfTestEntries())
	})
}
