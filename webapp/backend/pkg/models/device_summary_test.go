package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSmartSummaryJSONIncludesZeroTemperature(t *testing.T) {
	payload, err := json.Marshal(SmartSummary{Temp: 0})

	require.NoError(t, err)
	require.Contains(t, string(payload), `"temp":0`)
}
