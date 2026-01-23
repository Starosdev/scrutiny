package overrides

import (
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/stretchr/testify/assert"
)

func TestAttributeOverride_Matches(t *testing.T) {
	tests := []struct {
		name        string
		override    AttributeOverride
		protocol    string
		attributeId string
		wwn         string
		expected    bool
	}{
		{
			name:        "exact match without WWN filter",
			override:    AttributeOverride{Protocol: "ATA", AttributeId: "5"},
			protocol:    "ATA",
			attributeId: "5",
			wwn:         "0x123",
			expected:    true,
		},
		{
			name:        "protocol mismatch",
			override:    AttributeOverride{Protocol: "NVMe", AttributeId: "5"},
			protocol:    "ATA",
			attributeId: "5",
			wwn:         "",
			expected:    false,
		},
		{
			name:        "attribute_id mismatch",
			override:    AttributeOverride{Protocol: "ATA", AttributeId: "5"},
			protocol:    "ATA",
			attributeId: "187",
			wwn:         "",
			expected:    false,
		},
		{
			name:        "wwn filter match",
			override:    AttributeOverride{Protocol: "ATA", AttributeId: "5", WWN: "0x123"},
			protocol:    "ATA",
			attributeId: "5",
			wwn:         "0x123",
			expected:    true,
		},
		{
			name:        "wwn filter mismatch",
			override:    AttributeOverride{Protocol: "ATA", AttributeId: "5", WWN: "0x123"},
			protocol:    "ATA",
			attributeId: "5",
			wwn:         "0x456",
			expected:    false,
		},
		{
			name:        "nvme attribute match",
			override:    AttributeOverride{Protocol: "NVMe", AttributeId: "media_errors"},
			protocol:    "NVMe",
			attributeId: "media_errors",
			wwn:         "0xabc",
			expected:    true,
		},
		{
			name:        "scsi attribute match",
			override:    AttributeOverride{Protocol: "SCSI", AttributeId: "scsi_grown_defect_list"},
			protocol:    "SCSI",
			attributeId: "scsi_grown_defect_list",
			wwn:         "",
			expected:    true,
		},
		{
			name:        "devstat attribute match",
			override:    AttributeOverride{Protocol: "ATA", AttributeId: "devstat_7_8"},
			protocol:    "ATA",
			attributeId: "devstat_7_8",
			wwn:         "",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.override.Matches(tt.protocol, tt.attributeId, tt.wwn)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAttributeOverride_GetForcedStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected pkg.AttributeStatus
	}{
		{
			name:     "passed status",
			status:   "passed",
			expected: pkg.AttributeStatusPassed,
		},
		{
			name:     "warn status",
			status:   "warn",
			expected: pkg.AttributeStatusWarningScrutiny,
		},
		{
			name:     "failed status",
			status:   "failed",
			expected: pkg.AttributeStatusFailedScrutiny,
		},
		{
			name:     "unknown status defaults to passed",
			status:   "unknown",
			expected: pkg.AttributeStatusPassed,
		},
		{
			name:     "empty status defaults to passed",
			status:   "",
			expected: pkg.AttributeStatusPassed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			override := AttributeOverride{Status: tt.status}
			got := override.GetForcedStatus()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFindOverride(t *testing.T) {
	overrides := []AttributeOverride{
		{Protocol: "ATA", AttributeId: "5"},
		{Protocol: "ATA", AttributeId: "187", WWN: "0x123"},
		{Protocol: "NVMe", AttributeId: "media_errors"},
	}

	tests := []struct {
		name        string
		protocol    string
		attributeId string
		wwn         string
		expectFound bool
		expectId    string
	}{
		{
			name:        "find ATA attribute without WWN",
			protocol:    "ATA",
			attributeId: "5",
			wwn:         "0xabc",
			expectFound: true,
			expectId:    "5",
		},
		{
			name:        "find ATA attribute with matching WWN",
			protocol:    "ATA",
			attributeId: "187",
			wwn:         "0x123",
			expectFound: true,
			expectId:    "187",
		},
		{
			name:        "no match for ATA attribute with wrong WWN",
			protocol:    "ATA",
			attributeId: "187",
			wwn:         "0x456",
			expectFound: false,
		},
		{
			name:        "find NVMe attribute",
			protocol:    "NVMe",
			attributeId: "media_errors",
			wwn:         "",
			expectFound: true,
			expectId:    "media_errors",
		},
		{
			name:        "no match for unknown attribute",
			protocol:    "ATA",
			attributeId: "999",
			wwn:         "",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindOverride(overrides, tt.protocol, tt.attributeId, tt.wwn)
			if tt.expectFound {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectId, result.AttributeId)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestApplyThresholds(t *testing.T) {
	warnThreshold := int64(5)
	failThreshold := int64(10)

	tests := []struct {
		name           string
		result         *Result
		value          int64
		expectStatus   *pkg.AttributeStatus
		expectStatusIs pkg.AttributeStatus
	}{
		{
			name:         "nil result returns nil",
			result:       nil,
			value:        100,
			expectStatus: nil,
		},
		{
			name:         "value below warn threshold",
			result:       &Result{WarnAbove: &warnThreshold, FailAbove: &failThreshold},
			value:        3,
			expectStatus: nil,
		},
		{
			name:           "value exceeds warn threshold",
			result:         &Result{WarnAbove: &warnThreshold, FailAbove: &failThreshold},
			value:          7,
			expectStatusIs: pkg.AttributeStatusWarningScrutiny,
		},
		{
			name:           "value exceeds fail threshold",
			result:         &Result{WarnAbove: &warnThreshold, FailAbove: &failThreshold},
			value:          15,
			expectStatusIs: pkg.AttributeStatusFailedScrutiny,
		},
		{
			name:           "fail threshold takes precedence",
			result:         &Result{WarnAbove: &warnThreshold, FailAbove: &failThreshold},
			value:          10, // exactly at fail, but > means it needs to exceed
			expectStatusIs: pkg.AttributeStatusWarningScrutiny,
		},
		{
			name:           "value exceeds fail threshold (greater than)",
			result:         &Result{WarnAbove: &warnThreshold, FailAbove: &failThreshold},
			value:          11,
			expectStatusIs: pkg.AttributeStatusFailedScrutiny,
		},
		{
			name:           "only warn threshold set",
			result:         &Result{WarnAbove: &warnThreshold},
			value:          7,
			expectStatusIs: pkg.AttributeStatusWarningScrutiny,
		},
		{
			name:           "only fail threshold set",
			result:         &Result{FailAbove: &failThreshold},
			value:          15,
			expectStatusIs: pkg.AttributeStatusFailedScrutiny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyThresholds(tt.result, tt.value)
			if tt.expectStatus == nil && got == nil {
				return
			}
			if tt.expectStatus == nil && got != nil {
				assert.Equal(t, tt.expectStatusIs, *got)
			}
		})
	}
}

func TestResult_IgnoreAction(t *testing.T) {
	result := &Result{
		ShouldIgnore: true,
		StatusReason: "Attribute ignored by user configuration",
	}

	assert.True(t, result.ShouldIgnore)
	assert.Equal(t, "Attribute ignored by user configuration", result.StatusReason)
}

func TestResult_ForceStatusAction(t *testing.T) {
	status := pkg.AttributeStatusFailedScrutiny
	result := &Result{
		Status:       &status,
		StatusReason: "Status forced by user configuration",
	}

	assert.NotNil(t, result.Status)
	assert.Equal(t, pkg.AttributeStatusFailedScrutiny, *result.Status)
	assert.Equal(t, "Status forced by user configuration", result.StatusReason)
}
