package detect

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/collector/pkg/zfs/models"
	"github.com/sirupsen/logrus"
)

func newTestDetect() *Detect {
	logger := logrus.New()
	return &Detect{
		Logger: logrus.NewEntry(logger),
	}
}

// --- parseZFSBytes tests ---

func TestParseZFSBytes(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0B", 0},
		{"0", 0},
		{"", 0},
		{"512B", 512},
		{"1K", 1024},
		{"1.5K", 1536},
		{"2M", 2 * 1024 * 1024},
		{"2.3M", 2411724},
		{"1G", 1024 * 1024 * 1024},
		{"1T", 1024 * 1024 * 1024 * 1024},
		{"1P", 1024 * 1024 * 1024 * 1024 * 1024},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseZFSBytes(tt.input)
			if result != tt.expected {
				t.Errorf("parseZFSBytes(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// --- parseScrubStatus tests ---

func TestParseScrubStatus_ScrubFinishedShortDuration(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: scrub repaired 0B in 00:10:30 with 0 errors on Sun Jan  5 00:34:31 2026
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateFinished {
		t.Errorf("expected ScrubState=finished, got %q", pool.ScrubState)
	}
	if pool.ScrubIssuedBytes != 0 {
		t.Errorf("expected ScrubIssuedBytes=0, got %d", pool.ScrubIssuedBytes)
	}
	if pool.ScrubErrorsCount != 0 {
		t.Errorf("expected ScrubErrorsCount=0, got %d", pool.ScrubErrorsCount)
	}
	if pool.ScrubPercentComplete != 100.0 {
		t.Errorf("expected ScrubPercentComplete=100, got %f", pool.ScrubPercentComplete)
	}
	if pool.ScrubEndTime == nil {
		t.Fatal("expected ScrubEndTime to be set")
	}
	expectedDate := time.Date(2026, time.January, 5, 0, 34, 31, 0, time.UTC)
	if !pool.ScrubEndTime.Equal(expectedDate) {
		t.Errorf("expected ScrubEndTime=%v, got %v", expectedDate, *pool.ScrubEndTime)
	}
}

func TestParseScrubStatus_ScrubFinishedMultiDayDuration(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: scrub repaired 0B in 1 days 00:12:08 with 0 errors on Mon Jan 12 00:36:38 2026
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateFinished {
		t.Errorf("expected ScrubState=finished, got %q", pool.ScrubState)
	}
	if pool.ScrubErrorsCount != 0 {
		t.Errorf("expected ScrubErrorsCount=0, got %d", pool.ScrubErrorsCount)
	}
	if pool.ScrubEndTime == nil {
		t.Fatal("expected ScrubEndTime to be set")
	}
	expectedDate := time.Date(2026, time.January, 12, 0, 36, 38, 0, time.UTC)
	if !pool.ScrubEndTime.Equal(expectedDate) {
		t.Errorf("expected ScrubEndTime=%v, got %v", expectedDate, *pool.ScrubEndTime)
	}
}

func TestParseScrubStatus_ScrubFinishedWithRepairs(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: scrub repaired 1.5K in 00:10:30 with 2 errors on Sun Jan  5 00:34:31 2026
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateFinished {
		t.Errorf("expected ScrubState=finished, got %q", pool.ScrubState)
	}
	if pool.ScrubIssuedBytes != 1536 {
		t.Errorf("expected ScrubIssuedBytes=1536, got %d", pool.ScrubIssuedBytes)
	}
	if pool.ScrubErrorsCount != 2 {
		t.Errorf("expected ScrubErrorsCount=2, got %d", pool.ScrubErrorsCount)
	}
}

func TestParseScrubStatus_ScrubInProgress(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: scrub in progress since Sun Jan  5 00:24:01 2026
	tank        ONLINE       0     0     0
	  mirror-0  ONLINE       0     0     0
	42.5% done, 0 days 00:05:12 to go
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateScanning {
		t.Errorf("expected ScrubState=scanning, got %q", pool.ScrubState)
	}
	if pool.ScrubStartTime == nil {
		t.Fatal("expected ScrubStartTime to be set")
	}
	if pool.ScrubPercentComplete != 42.5 {
		t.Errorf("expected ScrubPercentComplete=42.5, got %f", pool.ScrubPercentComplete)
	}
}

func TestParseScrubStatus_ScrubCanceled(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: scrub canceled on Sun Jan  5 00:30:00 2026
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateCanceled {
		t.Errorf("expected ScrubState=canceled, got %q", pool.ScrubState)
	}
	if pool.ScrubEndTime == nil {
		t.Fatal("expected ScrubEndTime to be set")
	}
}

func TestParseScrubStatus_NoneRequested(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: none requested
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != "" {
		t.Errorf("expected ScrubState to be empty, got %q", pool.ScrubState)
	}
}

func TestParseScrubStatus_ResilverFinished(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: resilver repaired 1.5K in 00:05:30 with 0 errors on Tue Jan  6 12:00:00 2026
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateFinished {
		t.Errorf("expected ScrubState=finished, got %q", pool.ScrubState)
	}
	if pool.ScrubIssuedBytes != 1536 {
		t.Errorf("expected ScrubIssuedBytes=1536, got %d", pool.ScrubIssuedBytes)
	}
	if pool.ScrubEndTime == nil {
		t.Fatal("expected ScrubEndTime to be set")
	}
}

func TestParseScrubStatus_ResilverInProgress(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: DEGRADED
  scan: resilver in progress since Tue Jan  6 11:54:30 2026
	tank        DEGRADED     0     0     0
	  mirror-0  DEGRADED     0     0     0
	15.3% done, 0 days 00:02:45 to go
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateScanning {
		t.Errorf("expected ScrubState=scanning, got %q", pool.ScrubState)
	}
	if pool.ScrubStartTime == nil {
		t.Fatal("expected ScrubStartTime to be set")
	}
	if pool.ScrubPercentComplete != 15.3 {
		t.Errorf("expected ScrubPercentComplete=15.3, got %f", pool.ScrubPercentComplete)
	}
}

func TestParseScrubStatus_ResilverCanceled(t *testing.T) {
	d := newTestDetect()
	pool := &models.ZFSPool{}

	output := `  pool: tank
 state: ONLINE
  scan: resilver canceled on Tue Jan  6 12:30:00 2026
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
`
	d.parseScrubStatus(pool, output)

	if pool.ScrubState != models.ZFSScrubStateCanceled {
		t.Errorf("expected ScrubState=canceled, got %q", pool.ScrubState)
	}
}

// --- parseZFSDate tests ---

func TestParseZFSDate(t *testing.T) {
	d := newTestDetect()

	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "single digit day with double space",
			input:    "Sun Jan  5 00:34:31 2026",
			expected: time.Date(2026, time.January, 5, 0, 34, 31, 0, time.UTC),
		},
		{
			name:     "double digit day",
			input:    "Mon Jan 12 00:36:38 2026",
			expected: time.Date(2026, time.January, 12, 0, 36, 38, 0, time.UTC),
		},
		{
			name:     "different month",
			input:    "Tue Feb 14 10:15:30 2026",
			expected: time.Date(2026, time.February, 14, 10, 15, 30, 0, time.UTC),
		},
		{
			name:    "invalid date",
			input:   "not a date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.parseZFSDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.Equal(tt.expected) {
				t.Errorf("parseZFSDate(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
