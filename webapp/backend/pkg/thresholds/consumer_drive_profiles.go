package thresholds

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

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
	AtaObservedThresholds       map[int][]ObservedThreshold       `json:"ata_observed_thresholds,omitempty"`
	AtaCounterSeverityOverrides map[string]CounterSeverityProfile `json:"ata_counter_severity_overrides,omitempty"`
	compiledPattern             *regexp.Regexp                    `json:"-"`
	ModelPattern                string                            `json:"model_pattern,omitempty"`
	SampleCount                 int                               `json:"sample_count"`
	MinSamples                  int                               `json:"min_samples,omitempty"`
}

type consumerDriveProfileCatalog struct {
	Aliases  map[string]string      `json:"aliases"`
	Profiles []ConsumerDriveProfile `json:"profiles"`
}

func (p *ConsumerDriveProfile) MeetsConfidenceThreshold() bool {
	minSamples := p.MinSamples
	if minSamples <= 0 {
		minSamples = MinConsumerDriveProfileSamples
	}
	return p.SampleCount >= minSamples
}

//go:embed consumer_drive_profiles.json
var consumerDriveProfilesJSON []byte

var (
	consumerDriveProfilesByFamily map[string]ConsumerDriveProfile
	consumerDriveProfilesByModel  map[string]ConsumerDriveProfile
	consumerDriveProfilesByRegex  []ConsumerDriveProfile
)

func init() {
	if err := loadConsumerDriveProfiles(); err != nil {
		panic(err)
	}
}

func ValidateConsumerDriveProfileCatalog(data []byte) error {
	_, _, _, err := parseConsumerDriveProfiles(data)
	return err
}

func loadConsumerDriveProfiles() error {
	byFamily, byModel, regexProfiles, err := parseConsumerDriveProfiles(consumerDriveProfilesJSON)
	if err != nil {
		return err
	}
	consumerDriveProfilesByFamily = byFamily
	consumerDriveProfilesByModel = byModel
	consumerDriveProfilesByRegex = regexProfiles
	return nil
}

func parseConsumerDriveProfiles(data []byte) (map[string]ConsumerDriveProfile, map[string]ConsumerDriveProfile, []ConsumerDriveProfile, error) {
	var catalog consumerDriveProfileCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, nil, nil, fmt.Errorf("unmarshal consumer drive profiles: %w", err)
	}

	byFamily := make(map[string]ConsumerDriveProfile, len(catalog.Profiles))
	byModel := make(map[string]ConsumerDriveProfile, len(catalog.Aliases))
	regexProfiles := make([]ConsumerDriveProfile, 0)

	for i := range catalog.Profiles {
		profile := &catalog.Profiles[i]
		if profile.ModelFamily == "" {
			return nil, nil, nil, fmt.Errorf("profile missing model_family")
		}
		familyKey := normalizeConsumerDriveKey(profile.ModelFamily)
		if familyKey == "" {
			return nil, nil, nil, fmt.Errorf("profile has empty normalized family for %q", profile.ModelFamily)
		}
		if _, exists := byFamily[familyKey]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate profile family %q", profile.ModelFamily)
		}
		if profile.ModelPattern != "" {
			compiled, err := regexp.Compile(profile.ModelPattern)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("compile model_pattern for %q: %w", profile.ModelFamily, err)
			}
			profile.compiledPattern = compiled
			regexProfiles = append(regexProfiles, *profile)
		}
		byFamily[familyKey] = *profile
	}

	for modelAlias, family := range catalog.Aliases {
		profile, ok := byFamily[normalizeConsumerDriveKey(family)]
		if !ok {
			return nil, nil, nil, fmt.Errorf("alias %q points to unknown family %q", modelAlias, family)
		}
		modelKey := normalizeConsumerDriveKey(modelAlias)
		if modelKey == "" {
			return nil, nil, nil, fmt.Errorf("alias %q normalizes to empty key", modelAlias)
		}
		if existing, exists := byModel[modelKey]; exists {
			if normalizeConsumerDriveKey(existing.ModelFamily) != normalizeConsumerDriveKey(profile.ModelFamily) {
				return nil, nil, nil, fmt.Errorf("duplicate model alias %q", modelAlias)
			}
			continue
		}
		byModel[modelKey] = profile
	}
	return byFamily, byModel, regexProfiles, nil
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
	for i := range consumerDriveProfilesByRegex {
		profile := &consumerDriveProfilesByRegex[i]
		if profile.compiledPattern != nil && profile.compiledPattern.MatchString(modelName) && profile.MeetsConfidenceThreshold() {
			return profile, true
		}
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
