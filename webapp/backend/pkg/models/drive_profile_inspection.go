package models

// DriveProfileInspection is the diagnostic view of consumer drive profile
// matching for a single device. It exposes the full override path: which
// catalog entry matched, how it matched, whether the confidence gate passed,
// which overrides would be applied, and why the drive fell back to generic
// ATA rules when no profile is in effect.
type DriveProfileInspection struct {
	// DeviceWWN identifies the inspected drive.
	DeviceWWN string `json:"device_wwn"`

	// DeviceProtocol is the device protocol (ATA, NVMe, SCSI).
	DeviceProtocol string `json:"device_protocol"`

	// ModelFamily is the smartctl-reported model family used for matching.
	ModelFamily string `json:"model_family,omitempty"`

	// ModelName is the model name used for matching.
	ModelName string `json:"model_name,omitempty"`

	// ProfilesEnabled reports the global consumer drive profiles toggle.
	ProfilesEnabled bool `json:"profiles_enabled"`

	// Denylist echoes the normalized family keys currently denylisted in settings.
	Denylist []string `json:"denylist,omitempty"`

	// CatalogVersion is the version of the bundled profile catalog.
	CatalogVersion string `json:"catalog_version,omitempty"`

	// Matched reports whether the catalog recognized this drive at all,
	// regardless of whether the profile was applied.
	Matched bool `json:"matched"`

	// Applied reports whether the matched profile is actually used for SMART
	// status evaluation and replacement-risk scoring.
	Applied bool `json:"applied"`

	// MatchMethod is the lookup stage that matched: "model_family",
	// "model_name", "model_name_normalized", or "model_pattern".
	MatchMethod string `json:"match_method,omitempty"`

	// MatchedValue is the input value (family or model name) that matched.
	MatchedValue string `json:"matched_value,omitempty"`

	// ProfileFamily is the matched catalog family.
	ProfileFamily string `json:"profile_family,omitempty"`

	// ProfileVendor is the vendor recorded on the matched profile.
	ProfileVendor string `json:"profile_vendor,omitempty"`

	// ProfileSource is the provenance string of the matched profile.
	ProfileSource string `json:"profile_source,omitempty"`

	// SampleCount is the sample size behind the matched profile.
	SampleCount int `json:"sample_count,omitempty"`

	// MinSamples is the confidence gate the matched profile must satisfy.
	MinSamples int `json:"min_samples,omitempty"`

	// ConfidenceMet reports whether the matched profile passes its confidence gate.
	ConfidenceMet bool `json:"confidence_met"`

	// ObservedThresholdAttributes lists ATA attribute IDs whose observed-threshold
	// buckets are overridden by the matched profile during SMART status evaluation.
	ObservedThresholdAttributes []int `json:"observed_threshold_attributes,omitempty"`

	// CounterSeverityAttributes lists ATA attribute IDs whose counter severity
	// breakpoints are overridden by the matched profile during replacement-risk scoring.
	CounterSeverityAttributes []string `json:"counter_severity_attributes,omitempty"`

	// FallbackReason explains, in plain language, why generic ATA rules are in
	// effect. Empty when a profile is applied.
	FallbackReason string `json:"fallback_reason,omitempty"`
}

// DriveProfileInspectionResponse is the API response envelope for the
// GET /api/device/:wwn/drive-profile endpoint.
type DriveProfileInspectionResponse struct {
	Data    DriveProfileInspection `json:"data"`
	Success bool                   `json:"success"`
}
