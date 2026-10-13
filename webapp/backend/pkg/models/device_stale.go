package models

import "time"

// MissedPingTimeout returns how long the device may stay silent before it counts as missed:
// the per-device override when set, otherwise globalMinutes.
func (d *Device) MissedPingTimeout(globalMinutes int) time.Duration {
	minutes := globalMinutes
	if d.MissedPingTimeoutOverride > 0 {
		minutes = d.MissedPingTimeoutOverride
	}
	return time.Duration(minutes) * time.Minute
}

// IsStale reports whether a device last seen at lastSeen has been silent longer than its timeout.
// A zero lastSeen (never reported, e.g. newly registered) is not stale. Archived devices are never stale.
func (d *Device) IsStale(lastSeen, now time.Time, globalMinutes int) bool {
	if d.Archived || lastSeen.IsZero() {
		return false
	}
	return now.Sub(lastSeen) > d.MissedPingTimeout(globalMinutes)
}
