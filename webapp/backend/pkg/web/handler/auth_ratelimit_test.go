package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimiter_PrunesExpiredRecordsOfOtherIPs(t *testing.T) {
	rl := &rateLimiter{failures: map[string]*failureRecord{}, maxFailures: 2, window: 10 * time.Millisecond}
	rl.recordFailure("203.0.113.1")
	rl.recordFailure("203.0.113.2")
	require.Len(t, rl.failures, 2)

	time.Sleep(25 * time.Millisecond)
	rl.recordFailure("203.0.113.3")

	require.Len(t, rl.failures, 1, "expired records of other IPs must be pruned on the next failure")
	require.Contains(t, rl.failures, "203.0.113.3")
}

func TestRateLimiter_BlocksAfterMaxFailures(t *testing.T) {
	rl := &rateLimiter{failures: map[string]*failureRecord{}, maxFailures: 2, window: time.Minute}
	rl.recordFailure("198.51.100.1")
	require.False(t, rl.isBlocked("198.51.100.1"))
	rl.recordFailure("198.51.100.1")
	require.True(t, rl.isBlocked("198.51.100.1"))
}
