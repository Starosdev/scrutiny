package smartctl

import "testing"

func TestIsFatal(t *testing.T) {
	tests := []struct {
		name   string
		status int
		fatal  bool
	}{
		{"clean exit", 0, false},
		{"command line error", ExitCommandLineError, true},
		{"device open failed", ExitDeviceOpenFailed, true},
		{"both fatal bits", ExitCommandLineError | ExitDeviceOpenFailed, true},
		{"checksum error only (issue 663, exit 4)", ExitChecksumError, false},
		{"disk failing", ExitDiskFailing, false},
		{"prefail below threshold", ExitPrefailBelowThreshold, false},
		{"usage below threshold", ExitUsageBelowThreshold, false},
		{"error log has records", ExitErrorLogHasRecords, false},
		{"self-test log has errors", ExitSelfTestLogHasErrors, false},
		{"all non-fatal bits", 0xFC, false},
		{"fatal bit mixed with disk bits", ExitDeviceOpenFailed | ExitDiskFailing, true},
		{"all bits", 0xFF, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFatal(tt.status); got != tt.fatal {
				t.Errorf("IsFatal(%#x) = %v, want %v", tt.status, got, tt.fatal)
			}
		})
	}
}

func TestFatalMask(t *testing.T) {
	if FatalMask != 0x03 {
		t.Errorf("FatalMask = %#x, want 0x03", FatalMask)
	}
}
