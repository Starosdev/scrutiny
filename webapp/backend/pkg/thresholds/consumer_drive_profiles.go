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
	normalizeConsumerDriveKey("WDC Red"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "WDC",
		ModelFamily: "WDC Red",
		SampleCount: 44,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.02, ErrorInterval: []float64{0.01, 0.03}},
				{Low: 1, High: 4, AnnualFailureRate: 0.05, ErrorInterval: []float64{0.03, 0.08}},
				{Low: 4, High: 16, AnnualFailureRate: 0.14, ErrorInterval: []float64{0.09, 0.20}},
				{Low: 16, High: 64, AnnualFailureRate: 0.28, ErrorInterval: []float64{0.19, 0.38}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 2, High: 8, Critical: 24},
			"196": {Low: 0, Moderate: 2, High: 8, Critical: 24},
			"197": {Low: 0, Moderate: 1, High: 4, Critical: 12},
			"198": {Low: 0, Moderate: 1, High: 4, Critical: 12},
			"10":  {Low: 0, Moderate: 1, High: 2, Critical: 6},
		},
	},
	normalizeConsumerDriveKey("WDC Red Plus"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "WDC",
		ModelFamily: "WDC Red Plus",
		SampleCount: 147,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.01, ErrorInterval: []float64{0.005, 0.02}},
				{Low: 1, High: 4, AnnualFailureRate: 0.03, ErrorInterval: []float64{0.02, 0.05}},
				{Low: 4, High: 16, AnnualFailureRate: 0.09, ErrorInterval: []float64{0.06, 0.13}},
				{Low: 16, High: 64, AnnualFailureRate: 0.18, ErrorInterval: []float64{0.12, 0.25}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 3, High: 10, Critical: 28},
			"196": {Low: 0, Moderate: 3, High: 10, Critical: 28},
			"197": {Low: 0, Moderate: 2, High: 6, Critical: 16},
			"198": {Low: 0, Moderate: 2, High: 6, Critical: 16},
			"10":  {Low: 0, Moderate: 1, High: 3, Critical: 8},
		},
	},
	normalizeConsumerDriveKey("Seagate Desktop SSHD"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "Seagate",
		ModelFamily: "Seagate Desktop SSHD",
		SampleCount: 322,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.03, ErrorInterval: []float64{0.02, 0.05}},
				{Low: 1, High: 4, AnnualFailureRate: 0.08, ErrorInterval: []float64{0.05, 0.11}},
				{Low: 4, High: 16, AnnualFailureRate: 0.17, ErrorInterval: []float64{0.12, 0.23}},
				{Low: 16, High: 64, AnnualFailureRate: 0.33, ErrorInterval: []float64{0.24, 0.43}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 1, High: 6, Critical: 18},
			"196": {Low: 0, Moderate: 1, High: 6, Critical: 18},
			"197": {Low: 0, Moderate: 1, High: 3, Critical: 10},
			"198": {Low: 0, Moderate: 1, High: 3, Critical: 10},
			"10":  {Low: 0, Moderate: 1, High: 2, Critical: 5},
		},
	},
	normalizeConsumerDriveKey("Seagate Barracuda 7200.14 (AF)"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "Seagate",
		ModelFamily: "Seagate Barracuda 7200.14 (AF)",
		SampleCount: 6657,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.04, ErrorInterval: []float64{0.03, 0.05}},
				{Low: 1, High: 4, AnnualFailureRate: 0.10, ErrorInterval: []float64{0.08, 0.13}},
				{Low: 4, High: 16, AnnualFailureRate: 0.22, ErrorInterval: []float64{0.18, 0.27}},
				{Low: 16, High: 64, AnnualFailureRate: 0.40, ErrorInterval: []float64{0.33, 0.48}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 1, High: 4, Critical: 12},
			"196": {Low: 0, Moderate: 1, High: 4, Critical: 12},
			"197": {Low: 0, Moderate: 1, High: 3, Critical: 8},
			"198": {Low: 0, Moderate: 1, High: 3, Critical: 8},
			"10":  {Low: 0, Moderate: 1, High: 2, Critical: 5},
		},
	},
	normalizeConsumerDriveKey("WDC Caviar Blue"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "WDC",
		ModelFamily: "WDC Caviar Blue",
		SampleCount: 305,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.03, ErrorInterval: []float64{0.02, 0.04}},
				{Low: 1, High: 4, AnnualFailureRate: 0.07, ErrorInterval: []float64{0.05, 0.10}},
				{Low: 4, High: 16, AnnualFailureRate: 0.16, ErrorInterval: []float64{0.12, 0.21}},
				{Low: 16, High: 64, AnnualFailureRate: 0.31, ErrorInterval: []float64{0.24, 0.39}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 2, High: 8, Critical: 20},
			"196": {Low: 0, Moderate: 2, High: 8, Critical: 20},
			"197": {Low: 0, Moderate: 1, High: 4, Critical: 10},
			"198": {Low: 0, Moderate: 1, High: 4, Critical: 10},
			"10":  {Low: 0, Moderate: 1, High: 2, Critical: 5},
		},
	},
	normalizeConsumerDriveKey("WDC Caviar Green"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "WDC",
		ModelFamily: "WDC Caviar Green",
		SampleCount: 150,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.04, ErrorInterval: []float64{0.03, 0.05}},
				{Low: 1, High: 4, AnnualFailureRate: 0.09, ErrorInterval: []float64{0.06, 0.12}},
				{Low: 4, High: 16, AnnualFailureRate: 0.20, ErrorInterval: []float64{0.15, 0.26}},
				{Low: 16, High: 64, AnnualFailureRate: 0.38, ErrorInterval: []float64{0.29, 0.48}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 1, High: 6, Critical: 18},
			"196": {Low: 0, Moderate: 1, High: 6, Critical: 18},
			"197": {Low: 0, Moderate: 1, High: 3, Critical: 8},
			"198": {Low: 0, Moderate: 1, High: 3, Critical: 8},
			"10":  {Low: 0, Moderate: 1, High: 2, Critical: 5},
		},
	},
	normalizeConsumerDriveKey("Seagate Barracuda 7200.12"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "Seagate",
		ModelFamily: "Seagate Barracuda 7200.12",
		SampleCount: 3071,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.05, ErrorInterval: []float64{0.04, 0.06}},
				{Low: 1, High: 4, AnnualFailureRate: 0.12, ErrorInterval: []float64{0.10, 0.15}},
				{Low: 4, High: 16, AnnualFailureRate: 0.25, ErrorInterval: []float64{0.21, 0.30}},
				{Low: 16, High: 64, AnnualFailureRate: 0.45, ErrorInterval: []float64{0.38, 0.53}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 1, High: 4, Critical: 10},
			"196": {Low: 0, Moderate: 1, High: 4, Critical: 10},
			"197": {Low: 0, Moderate: 1, High: 3, Critical: 6},
			"198": {Low: 0, Moderate: 1, High: 3, Critical: 6},
			"10":  {Low: 0, Moderate: 1, High: 2, Critical: 4},
		},
	},
	normalizeConsumerDriveKey("Seagate Desktop HDD.15"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer HDD profile",
		Vendor:      "Seagate",
		ModelFamily: "Seagate Desktop HDD.15",
		SampleCount: 320,
		AtaObservedThresholds: map[int][]ObservedThreshold{
			5: {
				{Low: 0, High: 0, AnnualFailureRate: 0.05, ErrorInterval: []float64{0.04, 0.07}},
				{Low: 1, High: 4, AnnualFailureRate: 0.13, ErrorInterval: []float64{0.10, 0.17}},
				{Low: 4, High: 16, AnnualFailureRate: 0.27, ErrorInterval: []float64{0.21, 0.34}},
				{Low: 16, High: 64, AnnualFailureRate: 0.46, ErrorInterval: []float64{0.36, 0.56}},
			},
		},
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 1, High: 4, Critical: 10},
			"196": {Low: 0, Moderate: 1, High: 4, Critical: 10},
			"197": {Low: 0, Moderate: 1, High: 3, Critical: 6},
			"198": {Low: 0, Moderate: 1, High: 3, Critical: 6},
			"10":  {Low: 0, Moderate: 1, High: 2, Critical: 4},
		},
	},
	normalizeConsumerDriveKey("Samsung SSD 850 PRO"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer SSD profile",
		Vendor:      "Samsung",
		ModelFamily: "Samsung SSD 850 PRO",
		SampleCount: 355,
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 3, High: 10, Critical: 28},
			"196": {Low: 0, Moderate: 3, High: 10, Critical: 28},
			"197": {Low: 0, Moderate: 2, High: 6, Critical: 16},
			"198": {Low: 0, Moderate: 2, High: 6, Critical: 16},
		},
	},
	normalizeConsumerDriveKey("Samsung SSD 870 EVO"): {
		Protocol:    "ATA",
		Source:      "linuxhw/SMART curated consumer SSD profile",
		Vendor:      "Samsung",
		ModelFamily: "Samsung SSD 870 EVO",
		SampleCount: 897,
		AtaCounterSeverityOverrides: map[string]CounterSeverityProfile{
			"5":   {Low: 0, Moderate: 4, High: 12, Critical: 32},
			"196": {Low: 0, Moderate: 4, High: 12, Critical: 32},
			"197": {Low: 0, Moderate: 3, High: 8, Critical: 20},
			"198": {Low: 0, Moderate: 3, High: 8, Critical: 20},
		},
	},
}

var consumerDriveProfilesByModel = map[string]ConsumerDriveProfile{
	normalizeConsumerDriveKey("Samsung SSD 860 EVO 500GB"): consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Samsung based SSDs")],
	normalizeConsumerDriveKey("Samsung SSD 840 Series"):    consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Samsung based SSDs")],
	normalizeConsumerDriveKey("Hitachi HDS721050DLE630"):   consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Hitachi Deskstar 7K1000.D")],
	normalizeConsumerDriveKey("WDC WD80EFAX-68LHPN0"):      consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Red")],
	normalizeConsumerDriveKey("WDC_WD80EFAX-68LHPN0"):      consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Red")],
	normalizeConsumerDriveKey("WDC WD60EFRX-68MYMN1"):      consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Red")],
	normalizeConsumerDriveKey("WDC_WD60EFRX-68MYMN1"):      consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Red")],
	normalizeConsumerDriveKey("WDC WD30EFRX-68AX9N0"):      consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Red")],
	normalizeConsumerDriveKey("WDC_WD140EDFZ-11A0VA0"):     consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Red Plus")],
	normalizeConsumerDriveKey("WDC WD140EDFZ-11A0VA0"):     consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Red Plus")],
	normalizeConsumerDriveKey("ST6000DX000-1H217Z"):        consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Seagate Desktop SSHD")],
	normalizeConsumerDriveKey("ST4000DM000-1CD168"):        consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Seagate Barracuda 7200.14 (AF)")],
	normalizeConsumerDriveKey("ST4000DM000-1F2168"):        consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Seagate Barracuda 7200.14 (AF)")],
	normalizeConsumerDriveKey("WD10EZEX-08WN4A0"):          consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Caviar Blue")],
	normalizeConsumerDriveKey("WD10EZEX-00BN5A0"):          consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Caviar Blue")],
	normalizeConsumerDriveKey("WD20EARS-00MVWB0"):          consumerDriveProfilesByFamily[normalizeConsumerDriveKey("WDC Caviar Green")],
	normalizeConsumerDriveKey("ST2000DM001-1ER164"):        consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Seagate Barracuda 7200.12")],
	normalizeConsumerDriveKey("ST2000DM001-1CH164"):        consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Seagate Barracuda 7200.12")],
	normalizeConsumerDriveKey("ST3000DM001-1ER166"):        consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Seagate Desktop HDD.15")],
	normalizeConsumerDriveKey("ST3000DM001-1CH166"):        consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Seagate Desktop HDD.15")],
	normalizeConsumerDriveKey("X SSD 850 PRO 128GB"):       consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Samsung SSD 850 PRO")],
	normalizeConsumerDriveKey("Samsung SSD 870 EVO 1TB"):   consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Samsung SSD 870 EVO")],
	normalizeConsumerDriveKey("Samsung SSD 870 EVO 500GB"): consumerDriveProfilesByFamily[normalizeConsumerDriveKey("Samsung SSD 870 EVO")],
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
