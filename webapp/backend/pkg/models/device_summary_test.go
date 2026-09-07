package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func tempPtr(v int64) *int64 {
	return &v
}

// A genuine 0C reading must still serialize. omitempty on a pointer omits only
// nil, so this survives Temp becoming *int64.
func TestSmartSummaryJSONIncludesZeroTemperature(t *testing.T) {
	payload, err := json.Marshal(SmartSummary{Temp: tempPtr(0)})

	require.NoError(t, err)
	require.Contains(t, string(payload), `"temp":0`)
}

// A device that reported no temperature must omit the field entirely, so the
// frontend renders "--" rather than 0C.
func TestSmartSummaryJSONOmitsAbsentTemperature(t *testing.T) {
	payload, err := json.Marshal(SmartSummary{})

	require.NoError(t, err)
	require.NotContains(t, string(payload), `"temp"`)
}
