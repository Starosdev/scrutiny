package models_test

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestDeviceIsStale(t *testing.T) {
	now := time.Now()
	old := now.Add(-2 * time.Hour)

	require.True(t, (&models.Device{}).IsStale(old, now, 60))
	require.False(t, (&models.Device{}).IsStale(now.Add(-30*time.Minute), now, 60))
	require.False(t, (&models.Device{MissedPingTimeoutOverride: 180}).IsStale(old, now, 60))
	require.False(t, (&models.Device{Archived: true}).IsStale(old, now, 60))
	require.False(t, (&models.Device{}).IsStale(time.Time{}, now, 60))
	// zero or negative global timeout falls back to the 60 minute default
	require.False(t, (&models.Device{}).IsStale(now.Add(-30*time.Minute), now, 0))
	require.True(t, (&models.Device{}).IsStale(old, now, -5))
}
