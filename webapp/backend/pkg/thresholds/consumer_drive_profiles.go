package thresholds

import "strings"

const MinConsumerDriveProfileSamples = 20

type CounterSeverityProfile struct {
	Low      int64 `json:"low"`
	Moderate int64 `json:"moderate"`
	High     int64 `json:"high"`
	Critical int64 `json:"critical"`
}

type ConsumerDriveProfile struct {
	Protocol                    string                            `json:"protocol"`
	Source                      string                            `json:"source"`
	Vendor                      string                            `json:"vendor,omitempty"`
	ModelFamily                 string                            `json:"model_family,omitempty"`
	ModelName                   string                            `json:"model_name,omitempty"`
	SampleCount                 int                               `json:"sample_count"`
	MinSamples                  int                               `json:"min_samples,omitempty"`
	AtaObservedThresholds       map[int][]ObservedThreshold       `json:"ata_observed_thresholds,omitempty"`
	AtaCounterSeverityOverrides map[string]CounterSeverityProfile `json:"ata_counter_severity_overrides,omitempty"`
}

func (p ConsumerDriveProfile) MeetsConfidenceThreshold() bool {
	minSamples := p.MinSamples
	if minSamples <= 0 {
		minSamples = MinConsumerDriveProfileSamples
	}
	return p.SampleCount >= minSamples
}

var consumerDriveProfilesByFamily = map[string]ConsumerDriveProfile{
	normalizeConsumerDriveKey("Samsung based SSDs"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer SSD profile",
		Vendor:      "Samsung",
		ModelFamily: "Samsung based SSDs",
		SampleCount: 200,
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 2, High: 8, Critical: 24},
			"196": {Low: 0, Moderate: 2, High: 8, Critical: 24},
			"197": {Low: 0, Moderate: 1, High: 4, Critical: 12},
			"198": {Low: 0, Moderate: 1, High: 4, Critical: 12},
		},
	},
	normalizeConsumerDriveKey("Hitachi Deskstar 7K1000.D"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "Hitachi",
		ModelFamily: "Hitachi Deskstar 7K1000.D",
		SampleCount: 41,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.02, ErrorInterval: []float64{0.01, 0.03}},
				{Low: 1, High: 8, AnnualFailureRate: 0.06, ErrorInterval: []float64{0.03, 0.10}},
				{Low: 8, High: 32, AnnualFailureRate: 0.18, ErrorInterval: []float64{0.10, 0.27}},
				{Low: 32, High: 128, AnnualFailureRate: 0.35, ErrorInterval: []float64{0.22, 0.49}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 4, High: 16, Critical: 48},
			"196": {Low: 0, Moderate: 4, High: 16, Critical: 48},
			"197": {Low: 0, Moderate: 2, High: 8, Critical: 24},
			"198": {Low: 0, Moderate: 2, High: 8, Critical: 24},
			"10":  {Low: 0, Moderate: 1, High: 3, Critical: 8},
		},
	},
}

var consumerDriveProfilesByModel = map[string]ConsumerDriveProfile{
	normalizeConsumerDriveKey("Samsung SSD 860 EVO 500GB"): consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Samsung based SSDs")],
	normalizeConsumerDriveKey("Samsung SSD 840 Series"):    consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Samsung based SSDs")],
	normalizeConsumerDriveKey("Hitachi HDS721050DLE630"):   consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Hitachi Deskstar 7K1000.D")],
}

func LookupConsumerDriveProfile(protocol, modelFamily, modelName string) (*ConsumerDriveProfile, bool) {
	if !strings.EqualFold(protocol, "ATA") {
		return nil, false
	}

	if profile, ok := consumerDriveProfilesByFamily[normalizeConsumerDriveKey(modelFamily)]; ok && profile.MeetsConfidenceThreshold() {
		return &profile, true
	}
	if profile, ok := consumerDriveProfilesByModel[normalizeConsumerDriveKey(modelName)]; ok && profile.MeetsConfidenceThreshold() {
		return &profile, true
	}
	return nil, false
}

func normalizeConsumerDriveKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if lastUnderscore {
			continue
		}
		b.WriteByte('_')
		lastUnderscore = true
	}

	return strings.Trim(b.String(), "_")
}
