package database

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// summaryRecordRepository builds the minimum repository needed to exercise
// applySummaryRecord, which reads only the logger.
// tempPtr returns a pointer to v. SmartSummary.Temp is a pointer so that an
// absent reading and a genuine 0C are distinguishable.
func tempPtr(v int64) *int64 {
	return &v
}

func summaryRecordRepository() *scrutinyRepository {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	return &scrutinyRepository{logger: logger}
}

func summaryRecordFixture() (map[string]*models.DeviceSummary, map[string]string) {
	summaries := map[string]*models.DeviceSummary{
		"dev-1": {Device: models.Device{DeviceID: "dev-1", WWN: "wwn-1"}},
	}

	return summaries, map[string]string{"wwn-1": "dev-1"}
}

// A record missing temp or power_on_hours used to panic with
// "interface conversion: interface {} is nil, not int64". applySummaryRecord
// runs in the goroutine started by loadInitialMetrics, so the panic killed the
// web process before it finished binding and s6 restart-looped it.
func TestApplySummaryRecordToleratesMissingFields(t *testing.T) {
	collected := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name                 string
		values               map[string]interface{}
		expectedTemp         *int64
		expectedPowerOnHours int64
		expectedDate         time.Time
	}{
		{
			name: "all fields present",
			values: map[string]interface{}{
				"device_wwn":     "wwn-1",
				"temp":           int64(35),
				"power_on_hours": int64(1200),
				"_time":          collected,
			},
			expectedTemp:         tempPtr(35),
			expectedPowerOnHours: 1200,
			expectedDate:         collected,
		},
		{
			name: "temp absent",
			values: map[string]interface{}{
				"device_wwn":     "wwn-1",
				"power_on_hours": int64(1200),
				"_time":          collected,
			},
			expectedPowerOnHours: 1200,
			expectedDate:         collected,
		},
		{
			name: "temp explicitly nil",
			values: map[string]interface{}{
				"device_wwn":     "wwn-1",
				"temp":           nil,
				"power_on_hours": int64(1200),
				"_time":          collected,
			},
			expectedPowerOnHours: 1200,
			expectedDate:         collected,
		},
		{
			name: "power_on_hours absent",
			values: map[string]interface{}{
				"device_wwn": "wwn-1",
				"temp":       int64(35),
				"_time":      collected,
			},
			expectedTemp: tempPtr(35),
			expectedDate: collected,
		},
		{
			name: "time absent",
			values: map[string]interface{}{
				"device_wwn":     "wwn-1",
				"temp":           int64(35),
				"power_on_hours": int64(1200),
			},
			expectedTemp:         tempPtr(35),
			expectedPowerOnHours: 1200,
		},
		{
			name:   "every optional field absent",
			values: map[string]interface{}{"device_wwn": "wwn-1"},
		},
		{
			name: "wrong type is treated as absent",
			values: map[string]interface{}{
				"device_wwn":     "wwn-1",
				"temp":           "35",
				"power_on_hours": float64(1200),
				"_time":          "2026-09-06",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			summaries, wwnToDeviceID := summaryRecordFixture()

			require.NotPanics(t, func() {
				summaryRecordRepository().applySummaryRecord(summaries, wwnToDeviceID, testCase.values)
			})

			results := summaries["dev-1"].SmartResults
			require.NotNil(t, results, "a partial record must still produce a summary")
			if testCase.expectedTemp == nil {
				require.Nil(t, results.Temp, "an absent temperature must stay absent, not become 0")
			} else {
				require.NotNil(t, results.Temp)
				require.Equal(t, *testCase.expectedTemp, *results.Temp)
			}
			require.Equal(t, testCase.expectedPowerOnHours, results.PowerOnHours)
			require.Equal(t, testCase.expectedDate, results.CollectorDate)
		})
	}
}

// A non-string device_wwn reached wwnToDeviceID[deviceWWN.(string)] unguarded.
func TestApplySummaryRecordIgnoresUnusableDeviceWWN(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"missing":    {"temp": int64(35)},
		"nil":        {"device_wwn": nil},
		"non-string": {"device_wwn": int64(7)},
		"unknown":    {"device_wwn": "wwn-unknown"},
	}

	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			summaries, wwnToDeviceID := summaryRecordFixture()

			require.NotPanics(t, func() {
				summaryRecordRepository().applySummaryRecord(summaries, wwnToDeviceID, values)
			})

			require.Nil(t, summaries["dev-1"].SmartResults)
		})
	}
}

func TestMissingSummaryFieldsNamesEveryAbsentField(t *testing.T) {
	require.Empty(t, missingSummaryFields(true, true, true))
	require.Equal(t, []string{"temp"}, missingSummaryFields(false, true, true))
	require.Equal(t, []string{"power_on_hours"}, missingSummaryFields(true, false, true))
	require.Equal(t, []string{"_time"}, missingSummaryFields(true, true, false))
	require.Equal(t,
		[]string{"temp", "power_on_hours", "_time"},
		missingSummaryFields(false, false, false),
	)
}
