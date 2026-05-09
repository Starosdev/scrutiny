package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSmartSupportValueAndScanRoundTrip(t *testing.T) {
	enabled := true
	original := SmartSupport{Available: true, Enabled: &enabled}

	value, err := original.Value()
	require.NoError(t, err)

	var decoded SmartSupport
	require.NoError(t, decoded.Scan(value))
	require.True(t, decoded.Available)
	require.NotNil(t, decoded.Enabled)
	require.True(t, *decoded.Enabled)
}

func TestSmartSupportScanLegacyBool(t *testing.T) {
	var decoded SmartSupport
	require.NoError(t, decoded.Scan(true))
	require.True(t, decoded.Available)
	require.Nil(t, decoded.Enabled)
}
