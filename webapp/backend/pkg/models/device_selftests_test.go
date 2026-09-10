package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummarizeDeviceSelfTests(t *testing.T) {
	observedAt := int64(1234)
	health := SummarizeDeviceSelfTests([]DeviceSelfTest{
		{StatusPassed: true, ObservedAt: observedAt},
		{StatusPassed: false},
	})

	require.Equal(t, DeviceSelfTestStatusPassed, health.Status)
	require.True(t, health.HasResult)
	require.True(t, health.HasFailures)
	require.NotNil(t, health.LatestObservedAt)
	require.Equal(t, observedAt, *health.LatestObservedAt)
}

func TestSummarizeDeviceSelfTestsWithoutHistory(t *testing.T) {
	health := SummarizeDeviceSelfTests(nil)

	require.Equal(t, DeviceSelfTestStatusUnknown, health.Status)
	require.False(t, health.HasResult)
	require.False(t, health.HasFailures)
	require.Nil(t, health.LatestObservedAt)
}
