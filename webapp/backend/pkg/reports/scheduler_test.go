package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsDailyDue(t *testing.T) {
	// 08:00 daily schedule, current time is 08:01, last run was yesterday
	now := time.Date(2026, 2, 17, 8, 1, 0, 0, time.Local)
	lastRun := time.Date(2026, 2, 16, 8, 0, 0, 0, time.Local)
	assert.True(t, isDailyDue(now, lastRun, "08:00"))
}

func TestIsDailyDue_NotYet(t *testing.T) {
	now := time.Date(2026, 2, 17, 7, 59, 0, 0, time.Local)
	lastRun := time.Date(2026, 2, 16, 8, 0, 0, 0, time.Local)
	assert.False(t, isDailyDue(now, lastRun, "08:00"))
}

func TestIsDailyDue_AlreadyRan(t *testing.T) {
	now := time.Date(2026, 2, 17, 9, 0, 0, 0, time.Local)
	lastRun := time.Date(2026, 2, 17, 8, 0, 0, 0, time.Local)
	assert.False(t, isDailyDue(now, lastRun, "08:00"))
}

func TestIsDailyDue_NeverRan(t *testing.T) {
	now := time.Date(2026, 2, 17, 8, 1, 0, 0, time.Local)
	assert.True(t, isDailyDue(now, time.Time{}, "08:00"))
}

func TestIsWeeklyDue(t *testing.T) {
	// Monday Feb 16, 2026 is a Monday
	now := time.Date(2026, 2, 16, 8, 1, 0, 0, time.Local)
	lastRun := time.Date(2026, 2, 9, 8, 0, 0, 0, time.Local)
	assert.True(t, isWeeklyDue(now, lastRun, 1, "08:00")) // 1=Monday
}

func TestIsWeeklyDue_WrongDay(t *testing.T) {
	// Tuesday Feb 17, 2026
	now := time.Date(2026, 2, 17, 8, 1, 0, 0, time.Local)
	lastRun := time.Date(2026, 2, 9, 8, 0, 0, 0, time.Local)
	assert.False(t, isWeeklyDue(now, lastRun, 1, "08:00"))
}

func TestIsMonthlyDue(t *testing.T) {
	now := time.Date(2026, 3, 1, 8, 1, 0, 0, time.Local)
	lastRun := time.Date(2026, 2, 1, 8, 0, 0, 0, time.Local)
	assert.True(t, isMonthlyDue(now, lastRun, 1, "08:00"))
}

func TestIsMonthlyDue_WrongDay(t *testing.T) {
	now := time.Date(2026, 2, 15, 8, 1, 0, 0, time.Local)
	lastRun := time.Date(2026, 1, 1, 8, 0, 0, 0, time.Local)
	assert.False(t, isMonthlyDue(now, lastRun, 1, "08:00"))
}

func TestParseTimeOfDay(t *testing.T) {
	h, m := parseTimeOfDay("08:00")
	assert.Equal(t, 8, h)
	assert.Equal(t, 0, m)

	h, m = parseTimeOfDay("23:30")
	assert.Equal(t, 23, h)
	assert.Equal(t, 30, m)

	h, m = parseTimeOfDay("invalid")
	assert.Equal(t, 8, h)
	assert.Equal(t, 0, m)

	h, m = parseTimeOfDay("")
	assert.Equal(t, 8, h)
	assert.Equal(t, 0, m)
}
