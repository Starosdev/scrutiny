package measurements_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func expectConsumerDriveProfilesEnabledDefault(fakeConfig *mock_config.MockInterface) {
	fakeConfig.EXPECT().IsSet("user.metrics.consumer_drive_profiles_enabled").Return(false).AnyTimes()
	fakeConfig.EXPECT().GetString("user.metrics.consumer_drive_profiles_denylist").Return("").AnyTimes()
}

func TestSmart_Flatten(t *testing.T) {
	//setup
	timeNow := time.Now()
	smart := measurements.Smart{
		Date:             timeNow,
		DeviceWWN:        "test-wwn",
		DeviceProtocol:   pkg.DeviceProtocolAta,
		Temp:             50,
		PowerOnHours:     10,
		PowerCycleCount:  10,
		LogicalBlockSize: 512,
		Attributes:       nil,
		Status:           0,
	}

	//test
	tags, fields := smart.Flatten()

	//assert
	require.Equal(t, map[string]string{"device_protocol": "ATA", "device_wwn": "test-wwn", "device_id": ""}, tags)
	require.Equal(t, map[string]interface{}{"logical_block_size": int64(512), "power_cycle_count": int64(10), "power_on_hours": int64(10), "temp": int64(50)}, fields)
}

func TestSmart_Flatten_ATA(t *testing.T) {
	//setup
	timeNow := time.Now()
	smart := measurements.Smart{
		Date:             timeNow,
		DeviceWWN:        "test-wwn",
		DeviceProtocol:   pkg.DeviceProtocolAta,
		Temp:             50,
		PowerOnHours:     10,
		PowerCycleCount:  10,
		LogicalBlockSize: 512,
		Status:           0,
		Attributes: map[string]measurements.SmartAttribute{
			"1": &measurements.SmartAtaAttribute{
				AttributeId: 1,
				Value:       100,
				Threshold:   1,
				Worst:       100,
				RawValue:    0,
				RawString:   "0",
				WhenFailed:  "",
			},
			"2": &measurements.SmartAtaAttribute{
				AttributeId: 2,
				Value:       135,
				Threshold:   54,
				Worst:       135,
				RawValue:    108,
				RawString:   "108",
				WhenFailed:  "",
			},
		},
	}

	//test
	tags, fields := smart.Flatten()

	//assert
	require.Equal(t, map[string]string{"device_protocol": "ATA", "device_wwn": "test-wwn", "device_id": ""}, tags)
	require.Equal(t, map[string]interface{}{
		"attr.1.attribute_id":      "1",
		"attr.1.name":              "",
		"attr.1.failure_rate":      float64(0),
		"attr.1.raw_string":        "0",
		"attr.1.raw_value":         int64(0),
		"attr.1.status":            int64(0),
		"attr.1.status_reason":     "",
		"attr.1.thresh":            int64(1),
		"attr.1.transformed_value": int64(0),
		"attr.1.value":             int64(100),
		"attr.1.when_failed":       "",
		"attr.1.worst":             int64(100),

		"attr.2.attribute_id":      "2",
		"attr.2.name":              "",
		"attr.2.failure_rate":      float64(0),
		"attr.2.raw_string":        "108",
		"attr.2.raw_value":         int64(108),
		"attr.2.status":            int64(0),
		"attr.2.status_reason":     "",
		"attr.2.thresh":            int64(54),
		"attr.2.transformed_value": int64(0),
		"attr.2.value":             int64(135),
		"attr.2.when_failed":       "",
		"attr.2.worst":             int64(135),

		"logical_block_size": int64(512),
		"power_cycle_count":  int64(10),
		"power_on_hours":     int64(10),
		"temp":               int64(50),
	}, fields)
}

func TestSmart_Flatten_SCSI(t *testing.T) {
	//setup
	timeNow := time.Now()
	smart := measurements.Smart{
		Date:             timeNow,
		DeviceWWN:        "test-wwn",
		DeviceProtocol:   pkg.DeviceProtocolScsi,
		Temp:             50,
		PowerOnHours:     10,
		PowerCycleCount:  10,
		LogicalBlockSize: 512,
		Status:           0,
		Attributes: map[string]measurements.SmartAttribute{
			"read_errors_corrected_by_eccfast": &measurements.SmartScsiAttribute{
				AttributeId: "read_errors_corrected_by_eccfast",
				Value:       int64(300357663),
			},
		},
	}

	//test
	tags, fields := smart.Flatten()

	//assert
	require.Equal(t, map[string]string{"device_protocol": "SCSI", "device_wwn": "test-wwn", "device_id": ""}, tags)
	require.Equal(t, map[string]interface{}{
		"attr.read_errors_corrected_by_eccfast.attribute_id":      "read_errors_corrected_by_eccfast",
		"attr.read_errors_corrected_by_eccfast.failure_rate":      float64(0),
		"attr.read_errors_corrected_by_eccfast.status":            int64(0),
		"attr.read_errors_corrected_by_eccfast.status_reason":     "",
		"attr.read_errors_corrected_by_eccfast.thresh":            int64(0),
		"attr.read_errors_corrected_by_eccfast.transformed_value": int64(0),
		"attr.read_errors_corrected_by_eccfast.value":             int64(300357663),
		"logical_block_size": int64(512),
		"power_cycle_count":  int64(10),
		"power_on_hours":     int64(10),
		"temp":               int64(50)},
		fields)
}

func TestSmart_Flatten_NVMe(t *testing.T) {
	//setup
	timeNow := time.Now()
	smart := measurements.Smart{
		Date:             timeNow,
		DeviceWWN:        "test-wwn",
		DeviceProtocol:   pkg.DeviceProtocolNvme,
		Temp:             50,
		PowerOnHours:     10,
		PowerCycleCount:  10,
		LogicalBlockSize: 512,
		Status:           0,
		Attributes: map[string]measurements.SmartAttribute{
			"available_spare": &measurements.SmartNvmeAttribute{
				AttributeId: "available_spare",
				Value:       int64(100),
			},
		},
	}

	//test
	tags, fields := smart.Flatten()

	//assert
	require.Equal(t, map[string]string{"device_protocol": "NVMe", "device_wwn": "test-wwn", "device_id": ""}, tags)
	require.Equal(t, map[string]interface{}{
		"attr.available_spare.attribute_id":      "available_spare",
		"attr.available_spare.failure_rate":      float64(0),
		"attr.available_spare.status":            int64(0),
		"attr.available_spare.status_reason":     "",
		"attr.available_spare.thresh":            int64(0),
		"attr.available_spare.transformed_value": int64(0),
		"attr.available_spare.value":             int64(100),
		"logical_block_size":                     int64(512),
		"power_cycle_count":                      int64(10),
		"power_on_hours":                         int64(10),
		"temp":                                   int64(50)}, fields)
}

func TestNewSmartFromInfluxDB_ATA(t *testing.T) {
	//setup
	timeNow := time.Now()
	attrs := map[string]interface{}{
		"_time":                    timeNow,
		"device_wwn":               "test-wwn",
		"device_protocol":          pkg.DeviceProtocolAta,
		"attr.1.attribute_id":      "1",
		"attr.1.failure_rate":      float64(0),
		"attr.1.raw_string":        "108",
		"attr.1.raw_value":         int64(108),
		"attr.1.status":            int64(0),
		"attr.1.status_reason":     "",
		"attr.1.thresh":            int64(54),
		"attr.1.transformed_value": int64(0),
		"attr.1.value":             int64(135),
		"attr.1.when_failed":       "",
		"attr.1.worst":             int64(135),
		"power_cycle_count":        int64(10),
		"power_on_hours":           int64(10),
		"temp":                     int64(50),
	}

	//test
	smart, err := measurements.NewSmartFromInfluxDB(attrs, logrus.New())

	//assert
	require.NoError(t, err)
	require.Equal(t, &measurements.Smart{
		Date:            timeNow,
		DeviceWWN:       "test-wwn",
		DeviceProtocol:  "ATA",
		Temp:            50,
		PowerOnHours:    10,
		PowerCycleCount: 10,
		Attributes: map[string]measurements.SmartAttribute{
			"1": &measurements.SmartAtaAttribute{
				AttributeId: 1,
				Value:       135,
				Threshold:   54,
				Worst:       135,
				RawValue:    108,
				RawString:   "108",
				WhenFailed:  "",
			},
		}, Status: 0}, smart)
}

// A point written without a device_protocol tag keeps its plain fields and skips its attributes,
// instead of panicking or failing the whole history query.
func TestNewSmartFromInfluxDB_MissingProtocol(t *testing.T) {
	timeNow := time.Now()
	attrs := map[string]interface{}{
		"_time":               timeNow,
		"device_wwn":          "test-wwn",
		"attr.1.attribute_id": "1",
		"attr.1.value":        int64(135),
		"power_on_hours":      int64(10),
		"temp":                int64(50),
	}

	smart, err := measurements.NewSmartFromInfluxDB(attrs, logrus.New())

	require.NoError(t, err)
	require.Equal(t, &measurements.Smart{
		Date:         timeNow,
		DeviceWWN:    "test-wwn",
		Temp:         50,
		PowerOnHours: 10,
		Attributes:   map[string]measurements.SmartAttribute{},
	}, smart)
}

// A merged history row has null columns for attributes the point did not carry; they must not become
// empty attributes of the wrong type.
func TestNewSmartFromInfluxDB_NullColumnsAreAbsent(t *testing.T) {
	timeNow := time.Now()
	attrs := map[string]interface{}{
		"_time":                              timeNow,
		"device_wwn":                         "test-wwn",
		"device_protocol":                    pkg.DeviceProtocolAta,
		"attr.1.attribute_id":                "1",
		"attr.1.value":                       int64(135),
		"attr.critical_warning.value":        nil,
		"attr.critical_warning.attribute_id": nil,
		"temp":                               int64(50),
		"power_on_hours":                     nil,
	}

	smart, err := measurements.NewSmartFromInfluxDB(attrs, logrus.New())

	require.NoError(t, err)
	require.Len(t, smart.Attributes, 1)
	require.Contains(t, smart.Attributes, "1")
	require.Equal(t, int64(50), smart.Temp)
	require.Zero(t, smart.PowerOnHours)
}

func TestNewSmartFromInfluxDB_MissingTime(t *testing.T) {
	_, err := measurements.NewSmartFromInfluxDB(map[string]interface{}{"device_wwn": "test-wwn"}, logrus.New())
	require.Error(t, err)
}

func TestNewSmartFromInfluxDB_NVMe(t *testing.T) {
	//setup
	timeNow := time.Now()
	attrs := map[string]interface{}{
		"_time":                                  timeNow,
		"device_wwn":                             "test-wwn",
		"device_protocol":                        pkg.DeviceProtocolNvme,
		"attr.available_spare.attribute_id":      "available_spare",
		"attr.available_spare.failure_rate":      float64(0),
		"attr.available_spare.status":            int64(0),
		"attr.available_spare.status_reason":     "",
		"attr.available_spare.thresh":            int64(0),
		"attr.available_spare.transformed_value": int64(0),
		"attr.available_spare.value":             int64(100),
		"power_cycle_count":                      int64(10),
		"power_on_hours":                         int64(10),
		"temp":                                   int64(50),
	}

	//test
	smart, err := measurements.NewSmartFromInfluxDB(attrs, logrus.New())

	//assert
	require.NoError(t, err)
	require.Equal(t, &measurements.Smart{
		Date:            timeNow,
		DeviceWWN:       "test-wwn",
		DeviceProtocol:  "NVMe",
		Temp:            50,
		PowerOnHours:    10,
		PowerCycleCount: 10,
		Attributes: map[string]measurements.SmartAttribute{
			"available_spare": &measurements.SmartNvmeAttribute{
				AttributeId: "available_spare",
				Value:       int64(100),
			},
		}, Status: 0}, smart)
}

func TestNewSmartFromInfluxDB_SCSI(t *testing.T) {
	//setup
	timeNow := time.Now()
	attrs := map[string]interface{}{
		"_time":           timeNow,
		"device_wwn":      "test-wwn",
		"device_protocol": pkg.DeviceProtocolScsi,
		"attr.read_errors_corrected_by_eccfast.attribute_id":      "read_errors_corrected_by_eccfast",
		"attr.read_errors_corrected_by_eccfast.failure_rate":      float64(0),
		"attr.read_errors_corrected_by_eccfast.status":            int64(0),
		"attr.read_errors_corrected_by_eccfast.status_reason":     "",
		"attr.read_errors_corrected_by_eccfast.thresh":            int64(0),
		"attr.read_errors_corrected_by_eccfast.transformed_value": int64(0),
		"attr.read_errors_corrected_by_eccfast.value":             int64(300357663),
		"power_cycle_count": int64(10),
		"power_on_hours":    int64(10),
		"temp":              int64(50),
	}

	//test
	smart, err := measurements.NewSmartFromInfluxDB(attrs, logrus.New())

	//assert
	require.NoError(t, err)
	require.Equal(t, &measurements.Smart{
		Date:            timeNow,
		DeviceWWN:       "test-wwn",
		DeviceProtocol:  "SCSI",
		Temp:            50,
		PowerOnHours:    10,
		PowerCycleCount: 10,
		Attributes: map[string]measurements.SmartAttribute{
			"read_errors_corrected_by_eccfast": &measurements.SmartScsiAttribute{
				AttributeId: "read_errors_corrected_by_eccfast",
				Value:       int64(300357663),
			},
		}, Status: 0}, smart)
}

func TestFromCollectorSmartInfo(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-ata.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)
	require.Equal(t, 18, len(smartMdl.Attributes))

	//check that temperature was correctly parsed
	require.Equal(t, int64(163210330144), smartMdl.Attributes["194"].(*measurements.SmartAtaAttribute).RawValue)
	require.Equal(t, int64(32), smartMdl.Attributes["194"].(*measurements.SmartAtaAttribute).TransformedValue)

	//check that power-on hours was correctly transformed
	require.Equal(t, int64(1730), smartMdl.Attributes["9"].(*measurements.SmartAtaAttribute).TransformedValue)

	//ensure that Scrutiny warning for a non critical attribute does not set device status to failed.
	require.Equal(t, pkg.AttributeStatusWarningScrutiny, smartMdl.Attributes["3"].GetStatus())

}

func TestFromCollectorSmartInfo_Fail_Smart(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-fail.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusFailedSmart, smartMdl.Status)
	require.Equal(t, 0, len(smartMdl.Attributes))
}

func TestFromCollectorSmartInfo_Fail_ScrutinySmart(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-fail2.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny|pkg.DeviceStatusFailedSmart, smartMdl.Status)
	require.Equal(t, 17, len(smartMdl.Attributes))
}

func TestFromCollectorSmartInfo_Fail_ScrutinyNonCriticalFailed(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-ata-failed-scrutiny.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, smartMdl.Status)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, smartMdl.Attributes["199"].GetStatus(),
		"scrutiny should detect that %d failed (status: %d, %s)",
		smartMdl.Attributes["199"].(*measurements.SmartAtaAttribute).AttributeId,
		smartMdl.Attributes["199"].GetStatus(), smartMdl.Attributes["199"].(*measurements.SmartAtaAttribute).StatusReason,
	)

	require.Equal(t, 14, len(smartMdl.Attributes))
}

func TestFromCollectorSmartInfo_ATA_Issue587VendorSpecificSsdAttribute(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-ata-issue587-realssd-m4.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)
	require.Equal(t, int64(0), smartMdl.Temp)

	attr189 := smartMdl.Attributes["189"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr189.Status)
	require.Equal(t, "Skipped generic ATA observed-threshold scoring for vendor-specific SSD attribute", attr189.StatusReason)
	require.Zero(t, attr189.FailureRate)
}

func TestFromCollectorSmartInfo_ATA_HddHighFlyWritesStillUsesGenericScoring(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-sat.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)
	smartJson.RotationRate = 7200

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	attr189 := smartMdl.Attributes["189"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr189.Status)
	require.Empty(t, attr189.StatusReason)
	require.Greater(t, attr189.FailureRate, 0.0)
}

func TestFromCollectorSmartInfo_ATA_SsdRuntimeBadBlockStillUsesGenericScoring(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-ata-failed-scrutiny.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	attr183 := smartMdl.Attributes["183"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr183.Status)
	require.Empty(t, attr183.StatusReason)
	require.Greater(t, attr183.FailureRate, 0.0)
}

//TODO: Scrutiny Warn
//TODO: Smart + Scrutiny Warn

func TestFromCollectorSmartInfo_NVMe_Fail_Scrutiny(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-nvme-failed.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, smartMdl.Status)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, smartMdl.Attributes["media_errors"].GetStatus(),
		"scrutiny should detect that %s failed (status: %d, %s)",
		smartMdl.Attributes["media_errors"].(*measurements.SmartNvmeAttribute).AttributeId,
		smartMdl.Attributes["media_errors"].GetStatus(),
		smartMdl.Attributes["media_errors"].(*measurements.SmartNvmeAttribute).StatusReason,
	)

	require.Equal(t, 16, len(smartMdl.Attributes))
}

func TestFromCollectorSmartInfo_Nvme(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-nvme.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)
	require.Equal(t, 16, len(smartMdl.Attributes))

	require.Equal(t, int64(111303174), smartMdl.Attributes["host_reads"].(*measurements.SmartNvmeAttribute).Value)
	require.Equal(t, int64(83170961), smartMdl.Attributes["host_writes"].(*measurements.SmartNvmeAttribute).Value)
}

func TestFromCollectorSmartInfo_Scsi(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)
	require.Equal(t, 16, len(smartMdl.Attributes))

	require.Equal(t, int64(0), smartMdl.Attributes["scsi_grown_defect_list"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(300357663), smartMdl.Attributes["read_errors_corrected_by_eccfast"].(*measurements.SmartScsiAttribute).Value) //total_errors_corrected

	// read/write_gigabytes_processed should be parsed from the SCSI error counter log's
	// decimal-GB "gigabytes_processed" string into bytes (GB * 1e9).
	require.Contains(t, smartMdl.Attributes, "read_gigabytes_processed")
	require.Contains(t, smartMdl.Attributes, "write_gigabytes_processed")
	// smart-scsi.json / smart-scsi-failed.json both use read=176987.332 write=86472.611
	require.Equal(t, int64(176987332000000), smartMdl.Attributes["read_gigabytes_processed"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(86472611000000), smartMdl.Attributes["write_gigabytes_processed"].(*measurements.SmartScsiAttribute).Value)

	// This fixture has no endurance_used field (HDD, not SSD), so percentage_used should
	// not be created.
	require.NotContains(t, smartMdl.Attributes, "percentage_used")
}

func TestFromCollectorSmartInfo_Scsi_Fail_Scrutiny(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-failed.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, smartMdl.Status)

	// scsi_grown_defect_list should fail: Ideal="low", Value=5 > Threshold=0
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, smartMdl.Attributes["scsi_grown_defect_list"].GetStatus(),
		"scrutiny should detect that %s failed (status: %d, %s)",
		smartMdl.Attributes["scsi_grown_defect_list"].(*measurements.SmartScsiAttribute).AttributeId,
		smartMdl.Attributes["scsi_grown_defect_list"].GetStatus(),
		smartMdl.Attributes["scsi_grown_defect_list"].(*measurements.SmartScsiAttribute).StatusReason,
	)

	// read_total_uncorrected_errors should fail: Ideal="low", Value=3 > Threshold=0
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, smartMdl.Attributes["read_total_uncorrected_errors"].GetStatus(),
		"scrutiny should detect that %s failed (status: %d, %s)",
		smartMdl.Attributes["read_total_uncorrected_errors"].(*measurements.SmartScsiAttribute).AttributeId,
		smartMdl.Attributes["read_total_uncorrected_errors"].GetStatus(),
		smartMdl.Attributes["read_total_uncorrected_errors"].(*measurements.SmartScsiAttribute).StatusReason,
	)

	require.Equal(t, 16, len(smartMdl.Attributes))
}

// TestFromCollectorSmartInfo_Scsi_Verify_ErrorCounters tests the SCSI Verify Error Counter
// log page (0x05), requested in Starosdev/scrutiny#1024. Drives that report it get one
// attribute per counter, recorded for history but never failed on an absolute value, because
// background media scanning leaves these counters nonzero on a healthy drive.
func TestFromCollectorSmartInfo_Scsi_Verify_ErrorCounters(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-sas-verify.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	require.Equal(t, int64(0), smartMdl.Attributes["verify_errors_corrected_by_eccfast"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(12), smartMdl.Attributes["verify_errors_corrected_by_eccdelayed"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(3), smartMdl.Attributes["verify_errors_corrected_by_rereads_rewrites"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(15), smartMdl.Attributes["verify_total_errors_corrected"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(7054), smartMdl.Attributes["verify_correction_algorithm_invocations"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(0), smartMdl.Attributes["verify_total_uncorrected_errors"].(*measurements.SmartScsiAttribute).Value)
	// verify gigabytes_processed is "0.000" on this drive: host verify commands are rare, so
	// the counter stays 0 while the verify error counters rise from background scanning.
	require.Equal(t, int64(0), smartMdl.Attributes["verify_gigabytes_processed"].(*measurements.SmartScsiAttribute).Value)

	// Verify counters carry the same thresholds as read and write, so this fixture's three
	// recovered verify errors fail the drive here. Only the recovered counter is
	// delta-evaluated, which is what clears it at ingestion once the counter stops moving
	// (see TestApplyDeltaEvaluation_Scsi_SasPhy); FromCollectorSmartInfo has no previous
	// submission to compare against, so it still reports the failure.
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, smartMdl.Status)
	recoveredErrors := smartMdl.Attributes["verify_errors_corrected_by_rereads_rewrites"].(*measurements.SmartScsiAttribute)
	require.Equal(t, int64(0), recoveredErrors.Threshold)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, recoveredErrors.Status)
	recoveredMetadata, ok := thresholds.ScsiMetadataFor("verify_errors_corrected_by_rereads_rewrites")
	require.True(t, ok)
	require.True(t, recoveredMetadata.UseDeltaEvaluation)

	// Uncorrected verify errors are unreadable blocks the drive found itself, the same class
	// of defect as read/write uncorrected errors, so they are judged on their absolute value
	// and never cleared by a flat trend.
	uncorrectedMetadata, ok := thresholds.ScsiMetadataFor("verify_total_uncorrected_errors")
	require.True(t, ok)
	require.False(t, uncorrectedMetadata.UseDeltaEvaluation)
	require.Equal(t, int64(0), smartMdl.Attributes["verify_total_uncorrected_errors"].(*measurements.SmartScsiAttribute).Threshold)

	// The read/write pages keep their existing thresholds and attribute IDs.
	require.Equal(t, int64(0), smartMdl.Attributes["read_total_uncorrected_errors"].(*measurements.SmartScsiAttribute).Threshold)
	require.Equal(t, int64(176987332000000), smartMdl.Attributes["read_gigabytes_processed"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, 23, len(smartMdl.Attributes))

	// Every attribute the SCSI builder emits needs display metadata, or the UI renders
	// "Unknown Attribute Name" for it.
	for attributeID := range smartMdl.Attributes {
		_, ok := thresholds.ScsiMetadataFor(attributeID)
		require.True(t, ok, attributeID)
	}
}

// TestFromCollectorSmartInfo_Scsi_Verify_Absent tests that a drive which does not support the
// Verify Error Counter log page gets no verify attributes at all, rather than a set of
// permanently-zero ones. smartctl omits the "verify" key entirely on those drives.
func TestFromCollectorSmartInfo_Scsi_Verify_Absent(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)
	require.Nil(t, smartJson.ScsiErrorCounterLog.Verify)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	for _, attributeID := range []string{
		"verify_errors_corrected_by_eccfast",
		"verify_total_uncorrected_errors",
		"verify_gigabytes_processed",
	} {
		require.NotContains(t, smartMdl.Attributes, attributeID)
	}
	// The drive keeps exactly the attribute set it had before this change.
	require.Equal(t, 16, len(smartMdl.Attributes))
}

// TestFromCollectorSmartInfo_Scsi_SasPhy tests the per-PHY SAS Protocol log page counters
// requested in Starosdev/scrutiny#1024. Each PHY of each port gets its own attribute set, so
// a fault on one cable, backplane slot or HBA port stays identifiable.
func TestFromCollectorSmartInfo_Scsi_SasPhy(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-sas-phy.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	// Both ports are decoded from their numbered top-level keys, sorted by port then PHY.
	require.Len(t, smartJson.ScsiSasPhys, 2)
	require.Equal(t, 0, smartJson.ScsiSasPhys[0].Port)
	require.Equal(t, 0, smartJson.ScsiSasPhys[0].Phy)
	require.Equal(t, 1, smartJson.ScsiSasPhys[1].Port)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	require.Equal(t, int64(32), smartMdl.Attributes["sas_port_0_phy_0_invalid_dword_count"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(0), smartMdl.Attributes["sas_port_0_phy_0_running_disparity_error_count"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(8), smartMdl.Attributes["sas_port_0_phy_0_loss_of_dword_synchronization_count"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(8), smartMdl.Attributes["sas_port_0_phy_0_phy_reset_problem_count"].(*measurements.SmartScsiAttribute).Value)

	// "phy enabled; 12 Gbps" -> 12000 Mbps.
	require.Equal(t, int64(12000), smartMdl.Attributes["sas_port_0_phy_0_negotiated_link_rate_mbps"].(*measurements.SmartScsiAttribute).Value)

	// Port 1 has nothing attached: its counters are real (and zero), but "phy enabled;
	// unknown" carries no speed, so it gets no link rate attribute rather than a zero one.
	require.Equal(t, int64(0), smartMdl.Attributes["sas_port_1_phy_0_invalid_dword_count"].(*measurements.SmartScsiAttribute).Value)
	require.NotContains(t, smartMdl.Attributes, "sas_port_1_phy_0_negotiated_link_rate_mbps")

	// Each port's counters stay separate; nothing is summed across ports.
	require.Contains(t, smartMdl.Attributes, "sas_port_1_phy_0_phy_reset_problem_count")

	// Every attribute needs resolvable display metadata, including the generated PHY IDs,
	// or the UI renders "Unknown Attribute Name".
	for attributeID := range smartMdl.Attributes {
		metadata, ok := thresholds.ScsiMetadataFor(attributeID)
		require.True(t, ok, attributeID)
		require.NotEmpty(t, metadata.DisplayName, attributeID)
	}
	phyMetadata, ok := thresholds.ScsiMetadataFor("sas_port_1_phy_0_invalid_dword_count")
	require.True(t, ok)
	require.Equal(t, "Invalid DWORDs (Port 1 PHY 0)", phyMetadata.DisplayName)

	// 23 SCSI attributes from the verify fixture, plus 4 counters for each of the two PHYs,
	// plus one link rate for the only PHY reporting a speed.
	require.Equal(t, 32, len(smartMdl.Attributes))

	// Port 0's 32 invalid DWORDs exceed the counter's threshold of 0, so ingestion reports a
	// failure. ApplyDeltaEvaluation is what clears it on the next collection if the counter
	// has not moved; it needs a previous submission, which this path does not have.
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, smartMdl.Status)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, smartMdl.Attributes["sas_port_0_phy_0_invalid_dword_count"].(*measurements.SmartScsiAttribute).Status)
	require.Equal(t, pkg.AttributeStatusPassed, smartMdl.Attributes["sas_port_1_phy_0_invalid_dword_count"].(*measurements.SmartScsiAttribute).Status)
}

// TestParseScsiSasPhyLinkRateMbps covers the link rate strings smartctl emits, including the
// non-numeric states that must not become a zero-speed attribute.
func TestParseScsiSasPhyLinkRateMbps(t *testing.T) {
	for _, testCase := range []struct {
		raw      string
		expected int64
		ok       bool
	}{
		{"phy enabled; 12 Gbps", 12000, true},
		{"phy enabled; 6 Gbps", 6000, true},
		{"phy enabled; 1.5 Gbps", 1500, true},
		{"phy enabled; unknown", 0, false},
		{"phy disabled", 0, false},
		{"", 0, false},
	} {
		mbps, ok := measurements.ParseScsiSasPhyLinkRateMbps(testCase.raw)
		require.Equal(t, testCase.ok, ok, testCase.raw)
		require.Equal(t, testCase.expected, mbps, testCase.raw)
	}
}

// TestFromCollectorSmartInfo_Scsi_LifetimeCounters tests the remaining SCSI log pages
// requested in Starosdev/scrutiny#1024: Background Medium Scan status, the Environmental
// Reports lifetime temperature extremes, and the Start-Stop Cycle Counter wear counters.
func TestFromCollectorSmartInfo_Scsi_LifetimeCounters(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-sas-full.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	// Background medium scan: the drive's own media checking, on its own schedule.
	require.Equal(t, int64(10), smartMdl.Attributes["background_scans_performed"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(10), smartMdl.Attributes["background_medium_scans_performed"].(*measurements.SmartScsiAttribute).Value)
	backgroundScanStatus := smartMdl.Attributes["background_scan_status"].(*measurements.SmartScsiAttribute)
	require.Equal(t, int64(8), backgroundScanStatus.Value)
	// The drive's own wording for the state rides in the status reason, which the UI shows
	// next to the value: the attribute value itself is an int64 and cannot carry it.
	require.Equal(t, "waiting until BMS interval timer expires", backgroundScanStatus.StatusReason)
	// "0.00%" -> 0; no scan is running.
	require.Equal(t, int64(0), smartMdl.Attributes["background_scan_progress_percent"].(*measurements.SmartScsiAttribute).Value)

	// Lifetime temperature extremes the drive tracks itself, which reach further back than
	// Scrutiny's own history. The single-sensor key keeps its plain display name.
	require.Equal(t, int64(35), smartMdl.Attributes["temperature_1_lifetime_maximum"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(24), smartMdl.Attributes["temperature_1_lifetime_minimum"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(34), smartMdl.Attributes["temperature_1_maximum_since_power_on"].(*measurements.SmartScsiAttribute).Value)
	require.Equal(t, int64(28), smartMdl.Attributes["temperature_1_minimum_since_power_on"].(*measurements.SmartScsiAttribute).Value)
	// Every sensor is named, including the first: a drive can report several, and a row that
	// says only "Lifetime Maximum Temperature" would read as a drive-wide figure.
	lifetimeMaxMetadata, ok := thresholds.ScsiMetadataFor("temperature_1_lifetime_maximum")
	require.True(t, ok)
	require.Equal(t, "Lifetime Maximum Temperature (Sensor 1)", lifetimeMaxMetadata.DisplayName)
	secondSensorMetadata, ok := thresholds.ScsiMetadataFor("temperature_2_lifetime_maximum")
	require.True(t, ok)
	require.Equal(t, "Lifetime Maximum Temperature (Sensor 2)", secondSensorMetadata.DisplayName)

	// Mechanical wear: the value is the accumulated count, the threshold is the count the
	// manufacturer rates the drive for, so the pair reads as 44 of 50000.
	startStopCycles := smartMdl.Attributes["start_stop_cycles"].(*measurements.SmartScsiAttribute)
	require.Equal(t, int64(44), startStopCycles.Value)
	require.Equal(t, int64(50000), startStopCycles.Threshold)
	require.Equal(t, pkg.AttributeStatusPassed, startStopCycles.Status)
	loadUnloadCycles := smartMdl.Attributes["load_unload_cycles"].(*measurements.SmartScsiAttribute)
	require.Equal(t, int64(224), loadUnloadCycles.Value)
	require.Equal(t, int64(600000), loadUnloadCycles.Threshold)
	require.Equal(t, pkg.AttributeStatusPassed, loadUnloadCycles.Status)

	// 32 attributes from the SAS PHY fixture, plus 4 temperature extremes, 4 background scan
	// attributes and 2 wear counters.
	require.Equal(t, 42, len(smartMdl.Attributes))

	for attributeID := range smartMdl.Attributes {
		metadata, ok := thresholds.ScsiMetadataFor(attributeID)
		require.True(t, ok, attributeID)
		require.NotEmpty(t, metadata.DisplayName, attributeID)
	}
}

// TestFromCollectorSmartInfo_Scsi_StartStopCyclesPastRating tests a drive used past the
// start/stop count it was rated for. The rating is recorded as the attribute's threshold so
// the UI can show wear against it, but passing it is not reported as a failure: drives
// routinely outlive their rated cycle count, and the ATA equivalents (attributes 4, 193 and
// 225) are not failed either. It also tests that a counter reported without a rated limit
// gets no threshold rather than a zero one it would always exceed.
func TestFromCollectorSmartInfo_Scsi_StartStopCyclesPastRating(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	var smartJson collector.SmartInfo
	err := json.Unmarshal([]byte(`{
		"device": {"name": "/dev/sdg", "type": "scsi", "protocol": "SCSI"},
		"model_name": "SEAGATE ST4000NM0043",
		"serial_number": "test-serial",
		"smart_status": {"passed": true},
		"temperature": {"current": 30},
		"scsi_start_stop_cycle_counter": {
			"specified_cycle_count_over_device_lifetime": 10000,
			"accumulated_start_stop_cycles": 10001,
			"accumulated_load_unload_cycles": 1200
		}
	}`), &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	startStopCycles := smartMdl.Attributes["start_stop_cycles"].(*measurements.SmartScsiAttribute)
	require.Equal(t, int64(10001), startStopCycles.Value)
	require.Equal(t, int64(10000), startStopCycles.Threshold)
	require.Equal(t, pkg.AttributeStatusPassed, startStopCycles.Status)

	// The drive reports load/unload cycles but not the count it is rated for, so that counter
	// is recorded without a threshold rather than with a zero one it would always exceed.
	loadUnloadCycles := smartMdl.Attributes["load_unload_cycles"].(*measurements.SmartScsiAttribute)
	require.Equal(t, int64(1200), loadUnloadCycles.Value)
	require.Equal(t, int64(-1), loadUnloadCycles.Threshold)
	require.Equal(t, pkg.AttributeStatusPassed, loadUnloadCycles.Status)

	// A worn drive is still a healthy drive.
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)

	// Neither counter is marked critical, and neither can raise a notification on its own,
	// which is what keeps an old drive from going red for being old. They do still appear in
	// the UI's default attribute view: its filter admits any attribute whose value is below
	// its threshold, which is true of a healthy wear counter, and also of any attribute whose
	// value is negative, since "no threshold" is -1. A drive stored below freezing shows its
	// lifetime temperature minimum there for the same reason.
	for _, attributeID := range []string{"start_stop_cycles", "load_unload_cycles"} {
		metadata, ok := thresholds.ScsiMetadataFor(attributeID)
		require.True(t, ok, attributeID)
		require.False(t, metadata.Critical, attributeID)
		require.Empty(t, metadata.Ideal, attributeID)
	}
}

// TestFromCollectorSmartInfo_Scsi_StartStopCounterPartiallyReported tests a drive that
// supports the Start-Stop Cycle Counter log page but reports only some of its counters.
// smartctl omits a counter the drive does not report, so the absent one must produce no
// attribute at all rather than a permanently-zero one.
func TestFromCollectorSmartInfo_Scsi_StartStopCounterPartiallyReported(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	var smartJson collector.SmartInfo
	err := json.Unmarshal([]byte(`{
		"device": {"name": "/dev/sdg", "type": "scsi", "protocol": "SCSI"},
		"model_name": "SEAGATE ST4000NM0043",
		"serial_number": "test-serial",
		"smart_status": {"passed": true},
		"temperature": {"current": 30},
		"scsi_start_stop_cycle_counter": {
			"year_of_manufacture": "2025",
			"week_of_manufacture": "04",
			"specified_cycle_count_over_device_lifetime": 10000,
			"accumulated_start_stop_cycles": 44
		}
	}`), &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, int64(44), smartMdl.Attributes["start_stop_cycles"].(*measurements.SmartScsiAttribute).Value)
	require.NotContains(t, smartMdl.Attributes, "load_unload_cycles")
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)
}

// TestFromCollectorSmartInfo_Scsi_StartStopCounterReportedZero is the other half of the
// pointer decision: a counter the drive reports as zero is a fact about a drive that has
// never spun down, and must still produce an attribute. Only an absent counter is skipped.
func TestFromCollectorSmartInfo_Scsi_StartStopCounterReportedZero(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	var smartJson collector.SmartInfo
	err := json.Unmarshal([]byte(`{
		"device": {"name": "/dev/sdg", "type": "scsi", "protocol": "SCSI"},
		"model_name": "SEAGATE ST4000NM0043",
		"serial_number": "test-serial",
		"smart_status": {"passed": true},
		"temperature": {"current": 30},
		"scsi_start_stop_cycle_counter": {
			"specified_load_unload_count_over_device_lifetime": 600000,
			"accumulated_load_unload_cycles": 0
		}
	}`), &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	loadUnloadCycles := smartMdl.Attributes["load_unload_cycles"].(*measurements.SmartScsiAttribute)
	require.Equal(t, int64(0), loadUnloadCycles.Value)
	require.Equal(t, int64(600000), loadUnloadCycles.Threshold)
	require.NotContains(t, smartMdl.Attributes, "start_stop_cycles")
}

// TestFromCollectorSmartInfo_Scsi_SAS_SSD_Endurance tests that SAS SSDs (which report the
// Solid State Media log page as "endurance_used") get a "percentage_used" attribute, and that
// the read/write "gigabytes_processed" counters are converted to bytes. Fixes the SAS SSD
// workload/endurance "unknown" reporting gap.
func TestFromCollectorSmartInfo_Scsi_SAS_SSD_Endurance(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-sas-ssd.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)

	// percentage_used should be populated from endurance_used.current_percent
	require.Contains(t, smartMdl.Attributes, "percentage_used")
	percentageUsedAttr, ok := smartMdl.Attributes["percentage_used"].(*measurements.SmartScsiAttribute)
	require.True(t, ok, "SCSI percentage_used attribute should be *SmartScsiAttribute, got %T", smartMdl.Attributes["percentage_used"])
	require.Equal(t, int64(3), percentageUsedAttr.Value)

	// read_gigabytes_processed: 7.668 GB -> 7668000000 bytes
	require.Contains(t, smartMdl.Attributes, "read_gigabytes_processed")
	require.Equal(t, int64(7668000000), smartMdl.Attributes["read_gigabytes_processed"].(*measurements.SmartScsiAttribute).Value)

	// write_gigabytes_processed: 243.990 GB -> 243990000000 bytes
	require.Contains(t, smartMdl.Attributes, "write_gigabytes_processed")
	require.Equal(t, int64(243990000000), smartMdl.Attributes["write_gigabytes_processed"].(*measurements.SmartScsiAttribute).Value)
}

// TestFromCollectorSmartInfo_Scsi_SAS_SSD_Endurance_ExceedsThreshold tests that a SAS SSD
// reporting a "percentage_used" endurance value above 100 is flagged as failed, matching
// NVMe's percentage_used threshold behavior. Previously the SCSI attribute was created with
// Threshold: -1 (disabled), so a device already past its rated endurance limit stayed
// "Passed" despite the critical metadata for this attribute.
func TestFromCollectorSmartInfo_Scsi_SAS_SSD_Endurance_ExceedsThreshold(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-sas-ssd.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	// Push the reported endurance usage past the manufacturer's rated limit.
	smartJson.ScsiEnduranceUsed.CurrentPercent = 105

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, smartMdl.Status)

	require.Contains(t, smartMdl.Attributes, "percentage_used")
	percentageUsedAttr, ok := smartMdl.Attributes["percentage_used"].(*measurements.SmartScsiAttribute)
	require.True(t, ok, "SCSI percentage_used attribute should be *SmartScsiAttribute, got %T", smartMdl.Attributes["percentage_used"])
	require.Equal(t, int64(105), percentageUsedAttr.Value)
	require.True(t, pkg.AttributeStatusHas(percentageUsedAttr.Status, pkg.AttributeStatusFailedScrutiny),
		"percentage_used should fail once current_percent (%d) exceeds threshold (%d)", percentageUsedAttr.Value, percentageUsedAttr.Threshold)
}

// TestFromCollectorSmartInfo_Scsi_SAS_EnvironmentalReports tests that for SAS drives
// where the standard temperature field is 0, the temperature is correctly parsed
// from scsi_environmental_reports.temperature_1.current (fixes GitHub issue #26)
func TestFromCollectorSmartInfo_Scsi_SAS_EnvironmentalReports(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-sas-env-temp.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)

	// The temperature.current in the JSON is 0, but scsi_environmental_reports.temperature_1.current is 38
	// The fix should correctly extract 38 from scsi_environmental_reports
	require.Equal(t, int64(38), smartMdl.Temp, "Temperature should be parsed from scsi_environmental_reports when standard temperature is 0")
}

// TestFromCollectorSmartInfo_Scsi_TemperatureAttributeType verifies that the SCSI
// temperature attribute is correctly typed as SmartScsiAttribute (not SmartNvmeAttribute)
// and uses the corrected temperature value. Fixes GitHub issue #291.
func TestFromCollectorSmartInfo_Scsi_TemperatureAttributeType(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo
	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)
	require.NoError(t, err)

	// Temperature attribute must be SmartScsiAttribute, not SmartNvmeAttribute
	tempAttr, ok := smartMdl.Attributes["temperature"].(*measurements.SmartScsiAttribute)
	require.True(t, ok, "SCSI temperature attribute should be *SmartScsiAttribute, got %T", smartMdl.Attributes["temperature"])
	require.Equal(t, int64(34), tempAttr.Value, "Temperature attribute should use corrected temperature value")
	require.Equal(t, int64(34), smartMdl.Temp, "Smart.Temp should match temperature attribute value")
}

// TestFromCollectorSmartInfo_Scsi_TemperatureAttributeType_EnvReports verifies that
// when the standard temperature field is 0, the temperature attribute correctly uses
// the fallback value from scsi_environmental_reports. Fixes GitHub issue #291.
func TestFromCollectorSmartInfo_Scsi_TemperatureAttributeType_EnvReports(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-scsi-sas-env-temp.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo
	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)
	require.NoError(t, err)

	// Temperature attribute must be SmartScsiAttribute
	tempAttr, ok := smartMdl.Attributes["temperature"].(*measurements.SmartScsiAttribute)
	require.True(t, ok, "SCSI temperature attribute should be *SmartScsiAttribute, got %T", smartMdl.Attributes["temperature"])

	// Standard temp is 0 in this test data; should fall back to env reports (38)
	require.Equal(t, int64(38), tempAttr.Value, "Temperature attribute should use fallback from scsi_environmental_reports")
	require.Equal(t, int64(38), smartMdl.Temp, "Smart.Temp should match temperature attribute value")
}

// TestFromCollectorSmartInfo_ATA_DeviceStatistics tests that ATA Device Statistics
// from GP Log 0x04 are correctly parsed, including enterprise SSD metrics like
// "Percentage Used Endurance Indicator" (devstat_7_8). Fixes GitHub issue #7 (SCR-11).
func TestFromCollectorSmartInfo_ATA_DeviceStatistics(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartDataFile, err := os.Open("../testdata/smart-ata-full.json")
	require.NoError(t, err)
	defer smartDataFile.Close()

	var smartJson collector.SmartInfo

	smartDataBytes, err := ioutil.ReadAll(smartDataFile)
	require.NoError(t, err)
	err = json.Unmarshal(smartDataBytes, &smartJson)
	require.NoError(t, err)

	//test
	smartMdl := measurements.Smart{}
	err = smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)
	require.Equal(t, pkg.DeviceProtocolAta, smartMdl.DeviceProtocol)

	// Check that device statistics are parsed with correct attribute IDs
	// devstat_7_8 is "Percentage Used Endurance Indicator" - critical for enterprise SSDs
	devstat, ok := smartMdl.Attributes["devstat_7_8"]
	require.True(t, ok, "devstat_7_8 (Percentage Used Endurance Indicator) should be present")

	// Verify it's the correct type
	devstatAttr, ok := devstat.(*measurements.SmartAtaDeviceStatAttribute)
	require.True(t, ok, "devstat_7_8 should be SmartAtaDeviceStatAttribute type")

	require.Equal(t, "devstat_7_8", devstatAttr.AttributeId)
	require.Equal(t, int64(19), devstatAttr.Value, "Percentage Used should be 19%")

	// Verify other device statistics are also present (page 1, offset 8 = Lifetime Power-On Resets)
	_, ok = smartMdl.Attributes["devstat_1_8"]
	require.True(t, ok, "devstat_1_8 (Lifetime Power-On Resets) should be present")
}

// TestSmart_Flatten_WithDeviceStatistics tests that device statistics are correctly
// flattened for storage in InfluxDB with proper string-based attribute IDs.
func TestSmart_Flatten_WithDeviceStatistics(t *testing.T) {
	//setup
	timeNow := time.Now()
	smart := measurements.Smart{
		Date:            timeNow,
		DeviceWWN:       "test-wwn",
		DeviceProtocol:  pkg.DeviceProtocolAta,
		Temp:            50,
		PowerOnHours:    10,
		PowerCycleCount: 10,
		Attributes: map[string]measurements.SmartAttribute{
			"devstat_7_8": &measurements.SmartAtaDeviceStatAttribute{
				AttributeId:      "devstat_7_8",
				Value:            25,
				TransformedValue: 25,
			},
		},
		Status: 0,
	}

	//test
	_, fields := smart.Flatten()

	//assert
	require.Equal(t, "devstat_7_8", fields["attr.devstat_7_8.attribute_id"])
	require.Equal(t, int64(25), fields["attr.devstat_7_8.value"])
	require.Equal(t, int64(25), fields["attr.devstat_7_8.transformed_value"])
}

// TestNewSmartFromInfluxDB_WithDeviceStatistics tests that device statistics are
// correctly inflated from InfluxDB data.
func TestNewSmartFromInfluxDB_WithDeviceStatistics(t *testing.T) {
	//setup
	timeNow := time.Now()
	attrs := map[string]interface{}{
		"_time":           timeNow,
		"device_wwn":      "test-wwn",
		"device_protocol": pkg.DeviceProtocolAta,
		"temp":            int64(50),
		"power_on_hours":  int64(10),
		// Device statistics with string-based attribute ID
		"attr.devstat_7_8.attribute_id":      "devstat_7_8",
		"attr.devstat_7_8.value":             int64(42),
		"attr.devstat_7_8.thresh":            int64(100),
		"attr.devstat_7_8.transformed_value": int64(42),
		"attr.devstat_7_8.status":            int64(0),
		"attr.devstat_7_8.status_reason":     "",
		"attr.devstat_7_8.failure_rate":      float64(0),
	}

	//test
	smart, err := measurements.NewSmartFromInfluxDB(attrs, logrus.New())

	//assert
	require.NoError(t, err)
	require.Contains(t, smart.Attributes, "devstat_7_8")

	devstatAttr, ok := smart.Attributes["devstat_7_8"].(*measurements.SmartAtaDeviceStatAttribute)
	require.True(t, ok, "devstat_7_8 should be SmartAtaDeviceStatAttribute type")
	require.Equal(t, "devstat_7_8", devstatAttr.AttributeId)
	require.Equal(t, int64(42), devstatAttr.Value)
	require.Equal(t, int64(100), devstatAttr.Threshold)
}

// TestFromCollectorSmartInfo_ATA_DeviceStatistics_StatusPropagation tests that
// failed device statistics propagate to device status (issue #84 fix)
func TestFromCollectorSmartInfo_ATA_DeviceStatistics_StatusPropagation(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	fakeConfig.EXPECT().GetStringSlice("failures.ignored.devstat").Return([]string{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	// Create minimal smartctl output with a failing devstat attribute
	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
	}

	// Add device statistics with a value that should trigger failure
	// devstat_7_8 (Percentage Used) at 100% should fail
	smartJson.AtaDeviceStatistics.Pages = []struct {
		Number   int    `json:"number"`
		Name     string `json:"name"`
		Revision int    `json:"revision"`
		Table    []struct {
			Offset int    `json:"offset"`
			Name   string `json:"name"`
			Size   int    `json:"size"`
			Value  int64  `json:"value"`
			Flags  struct {
				Value                 int  `json:"value"`
				Valid                 bool `json:"valid"`
				Normalized            bool `json:"normalized"`
				SupportsDsn           bool `json:"supports_dsn"`
				MonitoredConditionMet bool `json:"monitored_condition_met"`
			} `json:"flags"`
		} `json:"table"`
	}{
		{
			Number: 7,
			Name:   "Solid State Device Statistics",
			Table: []struct {
				Offset int    `json:"offset"`
				Name   string `json:"name"`
				Size   int    `json:"size"`
				Value  int64  `json:"value"`
				Flags  struct {
					Value                 int  `json:"value"`
					Valid                 bool `json:"valid"`
					Normalized            bool `json:"normalized"`
					SupportsDsn           bool `json:"supports_dsn"`
					MonitoredConditionMet bool `json:"monitored_condition_met"`
				} `json:"flags"`
			}{
				{
					Offset: 8,
					Name:   "Percentage Used Endurance Indicator",
					Value:  100, // At end of life
					Flags: struct {
						Value                 int  `json:"value"`
						Valid                 bool `json:"valid"`
						Normalized            bool `json:"normalized"`
						SupportsDsn           bool `json:"supports_dsn"`
						MonitoredConditionMet bool `json:"monitored_condition_met"`
					}{Valid: true},
				},
			},
		},
	}

	//test
	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	// Device status should be failed due to devstat failure propagation
	require.True(t, pkg.DeviceStatusHas(smartMdl.Status, pkg.DeviceStatusFailedScrutiny),
		"Device status should be DeviceStatusFailedScrutiny when devstat attribute fails")

	// The attribute itself should also be marked as failed
	devstat := smartMdl.Attributes["devstat_7_8"].(*measurements.SmartAtaDeviceStatAttribute)
	require.True(t, pkg.AttributeStatusHas(devstat.Status, pkg.AttributeStatusFailedScrutiny),
		"devstat_7_8 should be marked as failed")
}

// TestFromCollectorSmartInfo_ATA_DeviceStatistics_IgnoreList tests that
// devstat attributes in the ignore list don't trigger device failure (issue #84 fix)
func TestFromCollectorSmartInfo_ATA_DeviceStatistics_IgnoreList(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	// Add devstat_7_8 to the ignore list
	fakeConfig.EXPECT().GetStringSlice("failures.ignored.devstat").Return([]string{"devstat_7_8"}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	// Create smartctl output with a failing devstat attribute that's ignored
	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
	}

	smartJson.AtaDeviceStatistics.Pages = []struct {
		Number   int    `json:"number"`
		Name     string `json:"name"`
		Revision int    `json:"revision"`
		Table    []struct {
			Offset int    `json:"offset"`
			Name   string `json:"name"`
			Size   int    `json:"size"`
			Value  int64  `json:"value"`
			Flags  struct {
				Value                 int  `json:"value"`
				Valid                 bool `json:"valid"`
				Normalized            bool `json:"normalized"`
				SupportsDsn           bool `json:"supports_dsn"`
				MonitoredConditionMet bool `json:"monitored_condition_met"`
			} `json:"flags"`
		} `json:"table"`
	}{
		{
			Number: 7,
			Name:   "Solid State Device Statistics",
			Table: []struct {
				Offset int    `json:"offset"`
				Name   string `json:"name"`
				Size   int    `json:"size"`
				Value  int64  `json:"value"`
				Flags  struct {
					Value                 int  `json:"value"`
					Valid                 bool `json:"valid"`
					Normalized            bool `json:"normalized"`
					SupportsDsn           bool `json:"supports_dsn"`
					MonitoredConditionMet bool `json:"monitored_condition_met"`
				} `json:"flags"`
			}{
				{
					Offset: 8,
					Name:   "Percentage Used Endurance Indicator",
					Value:  100, // Would normally trigger failure
					Flags: struct {
						Value                 int  `json:"value"`
						Valid                 bool `json:"valid"`
						Normalized            bool `json:"normalized"`
						SupportsDsn           bool `json:"supports_dsn"`
						MonitoredConditionMet bool `json:"monitored_condition_met"`
					}{Valid: true},
				},
			},
		},
	}

	//test
	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	// Device status should be PASSED because devstat_7_8 is in ignore list
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status,
		"Device status should be Passed when failing devstat attribute is in ignore list")

	// The attribute itself is still marked as failed (just not propagated to device)
	devstat := smartMdl.Attributes["devstat_7_8"].(*measurements.SmartAtaDeviceStatAttribute)
	require.True(t, pkg.AttributeStatusHas(devstat.Status, pkg.AttributeStatusFailedScrutiny),
		"devstat_7_8 should still be marked as failed even when ignored")
}

// TestFromCollectorSmartInfo_ATA_DeviceStatistics_InvalidValue tests that
// impossibly high values are marked as invalid and don't trigger device failure (issue #84)
func TestFromCollectorSmartInfo_ATA_DeviceStatistics_InvalidValue(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	fakeConfig.EXPECT().GetStringSlice("failures.ignored.devstat").Return([]string{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	// Create smartctl output with an impossibly high value (like reported in issue #84)
	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
	}

	smartJson.AtaDeviceStatistics.Pages = []struct {
		Number   int    `json:"number"`
		Name     string `json:"name"`
		Revision int    `json:"revision"`
		Table    []struct {
			Offset int    `json:"offset"`
			Name   string `json:"name"`
			Size   int    `json:"size"`
			Value  int64  `json:"value"`
			Flags  struct {
				Value                 int  `json:"value"`
				Valid                 bool `json:"valid"`
				Normalized            bool `json:"normalized"`
				SupportsDsn           bool `json:"supports_dsn"`
				MonitoredConditionMet bool `json:"monitored_condition_met"`
			} `json:"flags"`
		} `json:"table"`
	}{
		{
			Number: 7,
			Name:   "Solid State Device Statistics",
			Table: []struct {
				Offset int    `json:"offset"`
				Name   string `json:"name"`
				Size   int    `json:"size"`
				Value  int64  `json:"value"`
				Flags  struct {
					Value                 int  `json:"value"`
					Valid                 bool `json:"valid"`
					Normalized            bool `json:"normalized"`
					SupportsDsn           bool `json:"supports_dsn"`
					MonitoredConditionMet bool `json:"monitored_condition_met"`
				} `json:"flags"`
			}{
				{
					Offset: 8,
					Name:   "Percentage Used Endurance Indicator",
					Value:  420_000_000_000, // 420 billion - clearly corrupted data
					Flags: struct {
						Value                 int  `json:"value"`
						Valid                 bool `json:"valid"`
						Normalized            bool `json:"normalized"`
						SupportsDsn           bool `json:"supports_dsn"`
						MonitoredConditionMet bool `json:"monitored_condition_met"`
					}{Valid: true},
				},
			},
		},
	}

	//test
	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	// Device status should be PASSED because the value is invalid (corrupted)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status,
		"Device status should be Passed when devstat value is impossibly high (invalid)")

	// The attribute should be marked as invalid, not failed
	devstat := smartMdl.Attributes["devstat_7_8"].(*measurements.SmartAtaDeviceStatAttribute)
	require.True(t, pkg.AttributeStatusHas(devstat.Status, pkg.AttributeStatusInvalidValue),
		"devstat_7_8 should be marked as InvalidValue")
	require.False(t, pkg.AttributeStatusHas(devstat.Status, pkg.AttributeStatusFailedScrutiny),
		"devstat_7_8 should NOT be marked as FailedScrutiny")
}

// TestFromCollectorSmartInfo_ATA_TemperatureFallback tests that for ATA drives
// where the standard temperature field is 0, the temperature is correctly parsed
// from attribute 194 (Temperature). Fixes GitHub issue #179.
func TestFromCollectorSmartInfo_ATA_TemperatureFallback(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	// Create smartctl output where temperature.current is 0 but attribute 194 has valid temp
	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 0}, // Temperature not populated in standard field
	}

	// Add attribute 194 with temperature value in the raw value (lowest byte)
	// Temperature 42C stored as raw value with additional data in upper bytes
	smartJson.AtaSmartAttributes.Table = []collector.AtaSmartAttributesTableItem{
		{
			ID:   194,
			Name: "Temperature_Celsius",
			Raw: struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			}{
				Value:  42, // Temperature in lowest byte
				String: "42",
			},
		},
	}

	//test
	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, "WWN-test", smartMdl.DeviceWWN)

	// The temperature.current in the JSON is 0, but attribute 194 raw value is 42
	// The fix should correctly extract 42 from attribute 194
	require.Equal(t, int64(42), smartMdl.Temp, "Temperature should be parsed from attribute 194 when standard temperature is 0")
}

// TestFromCollectorSmartInfo_ATA_TemperatureFallback_BitMask tests that the bit-mask
// is correctly applied when extracting temperature from attribute 194.
// Some drives store additional data in upper bytes of the raw value.
func TestFromCollectorSmartInfo_ATA_TemperatureFallback_BitMask(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	// Create smartctl output where temperature.current is 0
	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 0},
	}

	// Add attribute 194 with temperature value encoded with additional data in upper bytes
	// This simulates drives that store min/max temps in the raw value: 0x002B00310032 = 43/49/50
	// Lowest byte (0x32 = 50) is the current temperature
	smartJson.AtaSmartAttributes.Table = []collector.AtaSmartAttributesTableItem{
		{
			ID:   194,
			Name: "Temperature_Celsius",
			Raw: struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			}{
				Value:  0x002B00310032, // Current=50, some value=49, some value=43
				String: "50 (Min/Max 43/49)",
			},
		},
	}

	//test
	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)

	// The bit-mask (& 0xFF) should extract only the lowest byte (50)
	require.Equal(t, int64(50), smartMdl.Temp, "Temperature should be extracted from lowest byte using bit-mask")
}

// TestFromCollectorSmartInfo_ATA_TemperatureFallback_NegativeTemp tests that negative
// temperature values from temperature.current trigger the fallback to attribute 194.
// Some drives (e.g., Patriot Burst Elite) report -53C due to corrupted device statistics.
func TestFromCollectorSmartInfo_ATA_TemperatureFallback_NegativeTemp(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: -53}, // Negative temperature from corrupted device statistics
	}

	smartJson.AtaSmartAttributes.Table = []collector.AtaSmartAttributesTableItem{
		{
			ID:   194,
			Name: "Temperature_Celsius",
			Raw: struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			}{
				Value:  45,
				String: "45",
			},
		},
	}

	//test
	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, int64(45), smartMdl.Temp,
		"Negative temperature should trigger fallback to attribute 194")
}

// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// CorrectedTemperature tests
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestCorrectedTemperature_NormalATA(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 35},
	}
	info.Device.Protocol = "ATA"

	require.Equal(t, int64(35), measurements.CorrectedTemperature(&info))
}

func TestCorrectedTemperature_NormalNVMe(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 42},
	}
	info.Device.Protocol = "NVMe"

	require.Equal(t, int64(42), measurements.CorrectedTemperature(&info))
}

func TestCorrectedTemperature_NegativeATA_FallbackToAttr194(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: -53},
	}
	info.Device.Protocol = "ATA"
	info.AtaSmartAttributes.Table = []collector.AtaSmartAttributesTableItem{
		{
			ID:   194,
			Name: "Temperature_Celsius",
			Raw: struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			}{Value: 45, String: "45"},
		},
	}

	require.Equal(t, int64(45), measurements.CorrectedTemperature(&info),
		"Negative temperature should fall back to attribute 194")
}

func TestCorrectedTemperature_ZeroATA_FallbackToAttr194(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 0},
	}
	info.Device.Protocol = "ATA"
	info.AtaSmartAttributes.Table = []collector.AtaSmartAttributesTableItem{
		{
			ID:   194,
			Name: "Temperature_Celsius",
			Raw: struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			}{Value: 42, String: "42"},
		},
	}

	require.Equal(t, int64(42), measurements.CorrectedTemperature(&info),
		"Zero temperature should fall back to attribute 194")
}

func TestCorrectedTemperature_ATA_BitMask(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 0},
	}
	info.Device.Protocol = "ATA"
	info.AtaSmartAttributes.Table = []collector.AtaSmartAttributesTableItem{
		{
			ID:   194,
			Name: "Temperature_Celsius",
			Raw: struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			}{Value: 0x002B00310032, String: "50 (Min/Max 43/49)"},
		},
	}

	require.Equal(t, int64(50), measurements.CorrectedTemperature(&info),
		"Should extract lowest byte via 0xFF bitmask")
}

func TestCorrectedTemperature_OverTemp_FallbackToAttr194(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 200},
	}
	info.Device.Protocol = "ATA"
	info.AtaSmartAttributes.Table = []collector.AtaSmartAttributesTableItem{
		{
			ID:   194,
			Name: "Temperature_Celsius",
			Raw: struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			}{Value: 38, String: "38"},
		},
	}

	require.Equal(t, int64(38), measurements.CorrectedTemperature(&info),
		">150C should trigger fallback to attribute 194")
}

func TestCorrectedTemperature_ATA_NoAttr194_KeepsOriginal(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: -53},
	}
	info.Device.Protocol = "ATA"

	require.Equal(t, int64(-53), measurements.CorrectedTemperature(&info),
		"Without attribute 194, should return the original value")
}

func TestCorrectedTemperature_SCSI_FallbackToEnvironmentalReports(t *testing.T) {
	info := collector.SmartInfo{
		Temperature: struct {
			Current int64 `json:"current"`
		}{Current: 0},
		ScsiEnvironmentalReports: map[string]collector.ScsiTemperatureData{
			"temperature_1": {Current: 38},
		},
	}
	info.Device.Protocol = "SCSI"

	require.Equal(t, int64(38), measurements.CorrectedTemperature(&info),
		"SCSI should fall back to scsi_environmental_reports")
}

// TestFromCollectorSmartInfo_ATA_Farm tests that Seagate FARM data is correctly
// parsed from the collector SmartInfo and produces the expected FARM attributes.
func TestFromCollectorSmartInfo_ATA_Farm(t *testing.T) {
	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
		SeagateFarmLog: &collector.SeagateFarmLog{
			Supported: true,
			DriveInfo: &collector.FarmDriveInformation{
				Poh:             2344,
				Spoh:            2322,
				HeadFlightHours: 516,
				HeadLoadEvents:  4333,
				PowerCycleCount: 19,
			},
			Workload: &collector.FarmWorkloadStatistics{
				TotalReadCommands:     72695,
				TotalWriteCommands:    172682850,
				LogicalSectorsWritten: 115597072064,
				LogicalSectorsRead:    34761656,
			},
			Errors: &collector.FarmErrorStatistics{
				NumberOfUnrecoverableReadErrors:     0,
				NumberOfUnrecoverableWriteErrors:    0,
				NumberOfReallocatedSectors:          0,
				NumberOfReallocatedCandidateSectors: 0,
				TotalCrcErrors:                      0,
				CommandTimeOutCountTotal:            1,
			},
			Environ: &collector.FarmEnvironmentStatistics{
				CurentTemp:  35,
				HighestTemp: 43,
				LowestTemp:  28,
			},
		},
	}

	//test
	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	//assert
	require.NoError(t, err)
	require.Equal(t, pkg.DeviceProtocolAta, smartMdl.DeviceProtocol)

	// Verify FARM attributes are present with correct types and values
	farmPoh, ok := smartMdl.Attributes["farm_poh"]
	require.True(t, ok, "farm_poh should be present")
	farmPohAttr, ok := farmPoh.(*measurements.SmartFarmAttribute)
	require.True(t, ok, "farm_poh should be SmartFarmAttribute type")
	require.Equal(t, int64(2344), farmPohAttr.Value)

	farmTemp, ok := smartMdl.Attributes["farm_current_temperature"]
	require.True(t, ok, "farm_current_temperature should be present")
	farmTempAttr := farmTemp.(*measurements.SmartFarmAttribute)
	require.Equal(t, int64(35), farmTempAttr.Value)

	farmRealloc, ok := smartMdl.Attributes["farm_reallocated_sectors"]
	require.True(t, ok, "farm_reallocated_sectors should be present")
	farmReallocAttr := farmRealloc.(*measurements.SmartFarmAttribute)
	require.Equal(t, int64(0), farmReallocAttr.Value)

	// Verify all 18 expected attributes are present
	expectedFarmAttrs := []string{
		"farm_poh", "farm_spoh", "farm_head_flight_hours", "farm_head_load_events", "farm_power_cycle_count",
		"farm_total_read_commands", "farm_total_write_commands", "farm_logical_sectors_written", "farm_logical_sectors_read",
		"farm_unrecoverable_read_errors", "farm_unrecoverable_write_errors", "farm_reallocated_sectors",
		"farm_reallocation_candidates", "farm_crc_errors", "farm_command_timeouts",
		"farm_current_temperature", "farm_highest_temperature", "farm_lowest_temperature",
	}
	for _, attrId := range expectedFarmAttrs {
		_, ok := smartMdl.Attributes[attrId]
		require.True(t, ok, "%s should be present in attributes", attrId)
	}

	// Device should be passing (all error counts are 0)
	require.Equal(t, pkg.DeviceStatusPassed, smartMdl.Status)
}

// TestFromCollectorSmartInfo_ATA_Farm_NilSkipped tests that nil FARM log
// produces no FARM attributes.
func TestFromCollectorSmartInfo_ATA_Farm_NilSkipped(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	expectConsumerDriveProfilesEnabledDefault(fakeConfig)
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	smartJson := collector.SmartInfo{
		Device: struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{Protocol: "ATA"},
		LocalTime: struct {
			TimeT   int64  `json:"time_t"`
			Asctime string `json:"asctime"`
		}{TimeT: time.Now().Unix()},
		SmartStatus: struct {
			Passed bool `json:"passed"`
		}{Passed: true},
		// No SeagateFarmLog field
	}

	smartMdl := measurements.Smart{}
	err := smartMdl.FromCollectorSmartInfo(fakeConfig, "WWN-test", smartJson)

	require.NoError(t, err)

	// No farm_ attributes should exist
	for attrId := range smartMdl.Attributes {
		require.False(t, len(attrId) > 5 && attrId[:5] == "farm_",
			"No farm_ attributes should exist when SeagateFarmLog is nil, found: %s", attrId)
	}
}

// TestSmart_Flatten_WithFarmAttributes tests that FARM attributes are correctly
// flattened for storage in InfluxDB.
func TestSmart_Flatten_WithFarmAttributes(t *testing.T) {
	timeNow := time.Now()
	smart := measurements.Smart{
		Date:            timeNow,
		DeviceWWN:       "test-wwn",
		DeviceProtocol:  pkg.DeviceProtocolAta,
		Temp:            35,
		PowerOnHours:    2344,
		PowerCycleCount: 19,
		Attributes: map[string]measurements.SmartAttribute{
			"farm_poh": &measurements.SmartFarmAttribute{
				AttributeId:      "farm_poh",
				Value:            2344,
				TransformedValue: 2344,
			},
			"farm_reallocated_sectors": &measurements.SmartFarmAttribute{
				AttributeId:      "farm_reallocated_sectors",
				Value:            0,
				TransformedValue: 0,
			},
		},
		Status: 0,
	}

	_, fields := smart.Flatten()

	require.Equal(t, "farm_poh", fields["attr.farm_poh.attribute_id"])
	require.Equal(t, int64(2344), fields["attr.farm_poh.value"])
	require.Equal(t, "farm_reallocated_sectors", fields["attr.farm_reallocated_sectors.attribute_id"])
	require.Equal(t, int64(0), fields["attr.farm_reallocated_sectors.value"])
}

// TestNewSmartFromInfluxDB_WithFarmAttributes tests that FARM attributes are
// correctly inflated from InfluxDB data.
func TestNewSmartFromInfluxDB_WithFarmAttributes(t *testing.T) {
	timeNow := time.Now()
	attrs := map[string]interface{}{
		"_time":           timeNow,
		"device_wwn":      "test-wwn",
		"device_protocol": pkg.DeviceProtocolAta,
		"temp":            int64(35),
		"power_on_hours":  int64(2344),
		// FARM attribute
		"attr.farm_poh.attribute_id":      "farm_poh",
		"attr.farm_poh.value":             int64(2344),
		"attr.farm_poh.thresh":            int64(0),
		"attr.farm_poh.transformed_value": int64(2344),
		"attr.farm_poh.status":            int64(0),
		"attr.farm_poh.status_reason":     "",
		"attr.farm_poh.failure_rate":      float64(0),
	}

	smart, err := measurements.NewSmartFromInfluxDB(attrs, logrus.New())

	require.NoError(t, err)
	require.Contains(t, smart.Attributes, "farm_poh")

	farmAttr, ok := smart.Attributes["farm_poh"].(*measurements.SmartFarmAttribute)
	require.True(t, ok, "farm_poh should be SmartFarmAttribute type")
	require.Equal(t, "farm_poh", farmAttr.AttributeId)
	require.Equal(t, int64(2344), farmAttr.Value)
}

func TestSmartApplyOverridesProjectsStableDeviceOverride(t *testing.T) {
	smart := measurements.Smart{
		DeviceWWN:      "nvme-serial",
		DeviceProtocol: pkg.DeviceProtocolNvme,
		Attributes: map[string]measurements.SmartAttribute{
			"media_errors": &measurements.SmartNvmeAttribute{
				AttributeId: "media_errors",
				Value:       3,
				Status:      pkg.AttributeStatusFailedScrutiny,
			},
		},
	}

	smart.ApplyOverrides([]overrides.AttributeOverride{{
		Protocol:    pkg.DeviceProtocolNvme,
		AttributeId: "media_errors",
		DeviceID:    "b62e6d86-6ce0-50da-8bff-b2ee54c4af4e",
		Action:      overrides.AttributeOverrideActionForceStatus,
		Status:      "passed",
	}}, "b62e6d86-6ce0-50da-8bff-b2ee54c4af4e")

	attribute := smart.Attributes["media_errors"].(*measurements.SmartNvmeAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attribute.Status)
	require.Equal(t, "Status forced by user configuration", attribute.StatusReason)
}

// acknowledgeOverride builds a device-scoped acknowledge override pinned to value.
func acknowledgeOverride(deviceID string, value int64) []overrides.AttributeOverride {
	return []overrides.AttributeOverride{{
		Protocol:    pkg.DeviceProtocolNvme,
		AttributeId: "media_errors",
		DeviceID:    deviceID,
		Action:      overrides.AttributeOverrideActionAcknowledge,
		PinnedValue: &value,
	}}
}

// nvmeSmartWithMediaErrors builds a failing NVMe result carrying one media_errors attribute.
func nvmeSmartWithMediaErrors(value int64, status pkg.AttributeStatus) measurements.Smart {
	return measurements.Smart{
		DeviceWWN:      "nvme-serial",
		DeviceProtocol: pkg.DeviceProtocolNvme,
		Attributes: map[string]measurements.SmartAttribute{
			"media_errors": &measurements.SmartNvmeAttribute{
				AttributeId: "media_errors",
				Value:       value,
				Status:      status,
			},
		},
	}
}

func TestSmartApplyOverridesAcknowledgePassesAtPinnedValue(t *testing.T) {
	deviceID := "b62e6d86-6ce0-50da-8bff-b2ee54c4af4e"
	smart := nvmeSmartWithMediaErrors(3, pkg.AttributeStatusFailedScrutiny)

	smart.ApplyOverrides(acknowledgeOverride(deviceID, 3), deviceID)

	attribute := smart.Attributes["media_errors"].(*measurements.SmartNvmeAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attribute.Status)
	require.Equal(t, "Acknowledged at value 3", attribute.StatusReason)
}

func TestSmartApplyOverridesAcknowledgeRefailsWhenValueChanges(t *testing.T) {
	deviceID := "b62e6d86-6ce0-50da-8bff-b2ee54c4af4e"
	smart := nvmeSmartWithMediaErrors(4, pkg.AttributeStatusFailedScrutiny)

	// The acknowledgement was pinned to 3; the drive has since reported 4.
	smart.ApplyOverrides(acknowledgeOverride(deviceID, 3), deviceID)

	attribute := smart.Attributes["media_errors"].(*measurements.SmartNvmeAttribute)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, attribute.Status)
	require.Equal(t, "Acknowledgement no longer applies: value changed from 3 to 4", attribute.StatusReason)
}

func TestSmartApplyOverridesAcknowledgeNeverMasksSmartFailure(t *testing.T) {
	deviceID := "b62e6d86-6ce0-50da-8bff-b2ee54c4af4e"
	smart := nvmeSmartWithMediaErrors(3, pkg.AttributeStatusFailedSmart)

	smart.ApplyOverrides(acknowledgeOverride(deviceID, 3), deviceID)

	attribute := smart.Attributes["media_errors"].(*measurements.SmartNvmeAttribute)
	require.Equal(t, pkg.AttributeStatusFailedSmart, attribute.Status)
}

func TestAttributeThresholdValueMatchesEvaluatedField(t *testing.T) {
	// ATA is evaluated on the raw value; every other protocol on the normalized one.
	// The acknowledge handler pins whatever this returns, so the two must not diverge.
	ataValue, ok := measurements.AttributeThresholdValue(&measurements.SmartAtaAttribute{RawValue: 42, Value: 7})
	require.True(t, ok)
	require.Equal(t, int64(42), ataValue)

	nvmeValue, ok := measurements.AttributeThresholdValue(&measurements.SmartNvmeAttribute{Value: 9})
	require.True(t, ok)
	require.Equal(t, int64(9), nvmeValue)
}
