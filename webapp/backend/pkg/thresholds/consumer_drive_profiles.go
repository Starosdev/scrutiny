package thresholds

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const MinConsumerDriveProfileSamples = 20

// ProfileMatchMethod identifies which lookup stage produced a consumer drive
// profile match. Stages are listed strongest-first: an exact model_family hit
// is more trustworthy than a regex pattern fallback.
type ProfileMatchMethod string

const (
	// ProfileMatchMethodModelFamily is an exact match on the smartctl-reported model family.
	ProfileMatchMethodModelFamily ProfileMatchMethod = "model_family"

	// ProfileMatchMethodModelName is an exact match on the normalized model name (alias table).
	ProfileMatchMethodModelName ProfileMatchMethod = "model_name"

	// ProfileMatchMethodModelNameNormalized is an exact match after vendor-aware
	// normalization (capacity suffix or firmware suffix stripped).
	ProfileMatchMethodModelNameNormalized ProfileMatchMethod = "model_name_normalized"

	// ProfileMatchMethodModelPattern is a regex model_pattern fallback match.
	ProfileMatchMethodModelPattern ProfileMatchMethod = "model_pattern"
)

// Skip reasons reported on a ConsumerDriveProfileMatch that was found in the
// catalog but intentionally not applied.
const (
	ProfileSkipReasonBelowConfidence  = "below_confidence_threshold"
	ProfileSkipReasonFamilyDenylisted = "family_denylisted"
)

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
	Version  string                 `json:"version,omitempty"`
	Aliases  map[string]string      `json:"aliases"`
	Profiles []ConsumerDriveProfile `json:"profiles"`
}

// ConsumerDriveProfileMatch describes the outcome of matching a device against
// the bundled profile catalog. A non-nil match means the catalog recognized the
// drive; Applied reports whether the profile was actually used (confidence gate
// passed and family not denylisted).
type ConsumerDriveProfileMatch struct {
	Profile *ConsumerDriveProfile

	// Method identifies the lookup stage that produced this match.
	Method ProfileMatchMethod

	// MatchedValue is the input value (family or model name) that matched.
	MatchedValue string

	// SkipReason explains why a matched profile was not applied. Empty when Applied.
	SkipReason string

	// CatalogVersion is the version string of the bundled catalog.
	CatalogVersion string

	// Applied reports whether the profile should be used for scoring.
	Applied bool
}

func (p *ConsumerDriveProfile) MeetsConfidenceThreshold() bool {
	minSamples := p.MinSamples
	if minSamples <= 0 {
		minSamples = MinConsumerDriveProfileSamples
	}
	return p.SampleCount >= minSamples
}

// EffectiveMinSamples returns the confidence gate this profile must satisfy.
func (p *ConsumerDriveProfile) EffectiveMinSamples() int {
	if p.MinSamples > 0 {
		return p.MinSamples
	}
	return MinConsumerDriveProfileSamples
}

//go:embed consumer_drive_profiles.json
var consumerDriveProfilesJSON []byte

type loadedConsumerDriveCatalog struct {
	version  string
	byFamily map[string]ConsumerDriveProfile
	byModel  map[string]ConsumerDriveProfile
	byRegex  []ConsumerDriveProfile
}

var consumerDriveCatalog *loadedConsumerDriveCatalog

func init() {
	catalog, err := parseConsumerDriveProfiles(consumerDriveProfilesJSON)
	if err != nil {
		panic(err)
	}
	consumerDriveCatalog = catalog
}

// ConsumerDriveProfileCatalogVersion returns the version string of the bundled catalog.
func ConsumerDriveProfileCatalogVersion() string {
	return consumerDriveCatalog.version
}

func ValidateConsumerDriveProfileCatalog(data []byte) error {
	_, err := parseConsumerDriveProfiles(data)
	return err
}

func parseConsumerDriveProfiles(data []byte) (*loadedConsumerDriveCatalog, error) {
	var catalog consumerDriveProfileCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("unmarshal consumer drive profiles: %w", err)
	}

	loaded := &loadedConsumerDriveCatalog{
		version:  catalog.Version,
		byFamily: make(map[string]ConsumerDriveProfile, len(catalog.Profiles)),
		byModel:  make(map[string]ConsumerDriveProfile, len(catalog.Aliases)),
		byRegex:  make([]ConsumerDriveProfile, 0),
	}
	for i := range catalog.Profiles {
		if err := addConsumerDriveProfile(&catalog.Profiles[i], loaded.byFamily, &loaded.byRegex); err != nil {
			return nil, err
		}
	}

	for modelAlias, family := range catalog.Aliases {
		if err := addConsumerDriveAlias(modelAlias, family, loaded.byFamily, loaded.byModel); err != nil {
			return nil, err
		}
	}
	return loaded, nil
}

// addConsumerDriveProfile validates a single profile and registers it under its normalized family
// key, also appending it to regexProfiles when it carries a model pattern.
func addConsumerDriveProfile(profile *ConsumerDriveProfile, byFamily map[string]ConsumerDriveProfile, regexProfiles *[]ConsumerDriveProfile) error {
	if profile.ModelFamily == "" {
		return fmt.Errorf("profile missing model_family")
	}
	if !strings.EqualFold(profile.Protocol, "ATA") {
		return fmt.Errorf("profile %q has unsupported protocol %q (only ATA is supported)", profile.ModelFamily, profile.Protocol)
	}
	if strings.TrimSpace(profile.Source) == "" {
		return fmt.Errorf("profile %q missing source", profile.ModelFamily)
	}
	if profile.SampleCount <= 0 {
		return fmt.Errorf("profile %q has non-positive sample_count %d", profile.ModelFamily, profile.SampleCount)
	}
	if profile.MinSamples < 0 {
		return fmt.Errorf("profile %q has negative min_samples %d", profile.ModelFamily, profile.MinSamples)
	}
	familyKey := normalizeConsumerDriveKey(profile.ModelFamily)
	if familyKey == "" {
		return fmt.Errorf("profile has empty normalized family for %q", profile.ModelFamily)
	}
	if _, exists := byFamily[familyKey]; exists {
		return fmt.Errorf("duplicate profile family %q", profile.ModelFamily)
	}
	if err := validateConsumerDriveOverrides(profile); err != nil {
		return err
	}
	if profile.ModelPattern != "" {
		compiled, err := regexp.Compile(profile.ModelPattern)
		if err != nil {
			return fmt.Errorf("compile model_pattern for %q: %w", profile.ModelFamily, err)
		}
		profile.compiledPattern = compiled
		*regexProfiles = append(*regexProfiles, *profile)
	}
	byFamily[familyKey] = *profile
	return nil
}

// validateConsumerDriveOverrides applies structural sanity rules to a profile's
// severity overrides and observed-threshold buckets.
func validateConsumerDriveOverrides(profile *ConsumerDriveProfile) error {
	for attrID, sev := range profile.AtaCounterSeverityOverrides {
		if sev.Low < 0 || sev.Moderate < sev.Low || sev.High < sev.Moderate || sev.Critical < sev.High {
			return fmt.Errorf("profile %q attribute %s severity override violates low <= moderate <= high <= critical ordering", profile.ModelFamily, attrID)
		}
	}
	for attrID, buckets := range profile.AtaObservedThresholds {
		for _, bucket := range buckets {
			if err := validateObservedThresholdBucket(bucket); err != nil {
				return fmt.Errorf("profile %q attribute %d: %w", profile.ModelFamily, attrID, err)
			}
		}
	}
	return nil
}

// validateObservedThresholdBucket checks a single observed-threshold bucket for
// structural validity.
func validateObservedThresholdBucket(bucket ObservedThreshold) error {
	if bucket.Low > bucket.High {
		return fmt.Errorf("observed threshold bucket has low > high")
	}
	if bucket.AnnualFailureRate < 0 || bucket.AnnualFailureRate > 1 {
		return fmt.Errorf("annual_failure_rate outside [0, 1]")
	}
	if len(bucket.ErrorInterval) != 0 && len(bucket.ErrorInterval) != 2 {
		return fmt.Errorf("error_interval must have exactly 2 values")
	}
	if len(bucket.ErrorInterval) == 2 && bucket.ErrorInterval[0] > bucket.ErrorInterval[1] {
		return fmt.Errorf("error_interval is not ordered")
	}
	return nil
}

// addConsumerDriveAlias resolves an alias to its family profile and registers it under the
// normalized model key, tolerating duplicate aliases that point to the same family.
func addConsumerDriveAlias(modelAlias, family string, byFamily, byModel map[string]ConsumerDriveProfile) error {
	profile, ok := byFamily[normalizeConsumerDriveKey(family)]
	if !ok {
		return fmt.Errorf("alias %q points to unknown family %q", modelAlias, family)
	}
	modelKey := normalizeConsumerDriveKey(modelAlias)
	if modelKey == "" {
		return fmt.Errorf("alias %q normalizes to empty key", modelAlias)
	}
	if existing, exists := byModel[modelKey]; exists {
		if normalizeConsumerDriveKey(existing.ModelFamily) != normalizeConsumerDriveKey(profile.ModelFamily) {
			return fmt.Errorf("duplicate model alias %q", modelAlias)
		}
		return nil
	}
	byModel[modelKey] = profile
	return nil
}

// ParseConsumerDriveProfileDenylist converts a comma-separated list of family
// names into a set of normalized family keys. Empty entries are ignored.
func ParseConsumerDriveProfileDenylist(csv string) map[string]struct{} {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	denied := map[string]struct{}{}
	for _, entry := range strings.Split(csv, ",") {
		key := normalizeConsumerDriveKey(entry)
		if key == "" {
			continue
		}
		denied[key] = struct{}{}
	}
	if len(denied) == 0 {
		return nil
	}
	return denied
}

// consumerDriveProfileCandidate is an intermediate raw catalog hit before
// confidence gating and denylist filtering.
type consumerDriveProfileCandidate struct {
	profile      *ConsumerDriveProfile
	method       ProfileMatchMethod
	matchedValue string
}

// MatchConsumerDriveProfile matches a device against the bundled profile
// catalog and reports full match metadata. deniedFamilies contains normalized
// family keys (see ParseConsumerDriveProfileDenylist) that must not be applied.
//
// Lookup stages run strongest-first: exact model_family, exact model_name
// (alias table), vendor-normalized model_name, then regex model_pattern.
// A candidate that fails the confidence gate or is denylisted is skipped and
// weaker stages are still consulted. When no candidate can be applied, the
// first skipped candidate is returned (Applied=false) so callers can explain
// why the drive fell back to generic ATA rules. Returns nil when the catalog
// does not recognize the drive at all.
func MatchConsumerDriveProfile(protocol, modelFamily, modelName string, deniedFamilies map[string]struct{}) *ConsumerDriveProfileMatch {
	return consumerDriveCatalog.match(protocol, modelFamily, modelName, deniedFamilies)
}

// ConsumerDriveCatalogHandle wraps a parsed catalog document so tooling (the
// catalog-lint CLI, tests) can run matches against a catalog file on disk
// instead of the embedded bundle.
type ConsumerDriveCatalogHandle struct {
	loaded *loadedConsumerDriveCatalog
}

// LoadConsumerDriveProfileCatalog parses and validates a catalog document and
// returns a handle for running matches against it.
func LoadConsumerDriveProfileCatalog(data []byte) (*ConsumerDriveCatalogHandle, error) {
	loaded, err := parseConsumerDriveProfiles(data)
	if err != nil {
		return nil, err
	}
	return &ConsumerDriveCatalogHandle{loaded: loaded}, nil
}

// Match runs the same matching pipeline as MatchConsumerDriveProfile against
// this handle's catalog.
func (h *ConsumerDriveCatalogHandle) Match(protocol, modelFamily, modelName string, deniedFamilies map[string]struct{}) *ConsumerDriveProfileMatch {
	return h.loaded.match(protocol, modelFamily, modelName, deniedFamilies)
}

// Version returns the catalog document's version string.
func (h *ConsumerDriveCatalogHandle) Version() string {
	return h.loaded.version
}

func (lc *loadedConsumerDriveCatalog) match(protocol, modelFamily, modelName string, deniedFamilies map[string]struct{}) *ConsumerDriveProfileMatch {
	if !strings.EqualFold(protocol, "ATA") {
		return nil
	}

	var firstSkipped *ConsumerDriveProfileMatch
	for _, candidate := range lc.candidates(modelFamily, modelName) {
		match := &ConsumerDriveProfileMatch{
			Profile:        candidate.profile,
			Method:         candidate.method,
			MatchedValue:   candidate.matchedValue,
			CatalogVersion: lc.version,
		}
		if _, denied := deniedFamilies[normalizeConsumerDriveKey(candidate.profile.ModelFamily)]; denied {
			match.SkipReason = ProfileSkipReasonFamilyDenylisted
		} else if !candidate.profile.MeetsConfidenceThreshold() {
			match.SkipReason = ProfileSkipReasonBelowConfidence
		} else {
			match.Applied = true
			return match
		}
		if firstSkipped == nil {
			firstSkipped = match
		}
	}
	return firstSkipped
}

// candidates returns raw catalog hits ordered strongest-first.
func (lc *loadedConsumerDriveCatalog) candidates(modelFamily, modelName string) []consumerDriveProfileCandidate {
	candidates := make([]consumerDriveProfileCandidate, 0, 2)

	if profile, ok := lc.byFamily[normalizeConsumerDriveKey(modelFamily)]; ok {
		candidates = append(candidates, consumerDriveProfileCandidate{&profile, ProfileMatchMethodModelFamily, modelFamily})
	}
	if profile, ok := lc.byModel[normalizeConsumerDriveKey(modelName)]; ok {
		candidates = append(candidates, consumerDriveProfileCandidate{&profile, ProfileMatchMethodModelName, modelName})
	}
	for _, key := range consumerDriveModelNameVariants(modelName) {
		if profile, ok := lc.byModel[key]; ok {
			candidates = append(candidates, consumerDriveProfileCandidate{&profile, ProfileMatchMethodModelNameNormalized, modelName})
			continue
		}
		if profile, ok := lc.byFamily[key]; ok {
			candidates = append(candidates, consumerDriveProfileCandidate{&profile, ProfileMatchMethodModelNameNormalized, modelName})
		}
	}
	for i := range lc.byRegex {
		profile := lc.byRegex[i]
		if profile.compiledPattern != nil && profile.compiledPattern.MatchString(modelName) {
			candidates = append(candidates, consumerDriveProfileCandidate{&profile, ProfileMatchMethodModelPattern, modelName})
		}
	}
	return candidates
}

// LookupConsumerDriveProfile returns the applied profile for a device, if any.
// It is a convenience wrapper around MatchConsumerDriveProfile without denylist
// filtering; callers that need match metadata or denylist support should use
// MatchConsumerDriveProfile directly.
func LookupConsumerDriveProfile(protocol, modelFamily, modelName string) (*ConsumerDriveProfile, bool) {
	match := MatchConsumerDriveProfile(protocol, modelFamily, modelName, nil)
	if match == nil || !match.Applied {
		return nil, false
	}
	return match.Profile, true
}

var (
	// capacity suffix: " 500GB", "_1TB", "-2.5TB" at end of model name
	consumerDriveCapacitySuffixPattern = regexp.MustCompile(`(?i)[\s_-]+\d+(\.\d+)?\s?(gb|tb)$`)
	// WDC firmware suffix: "WDC WD80EFAX-68LHPN0" -> "WDC WD80EFAX"
	consumerDriveWdcFirmwarePattern = regexp.MustCompile(`(?i)^(wdc[\s_]+wd[0-9a-z]+)-[0-9a-z]{6,10}$`)
	// Seagate firmware suffix: "ST4000DM000-1F2168" -> "ST4000DM000"
	consumerDriveSeagateFirmwarePattern = regexp.MustCompile(`(?i)^(st\d+[a-z]{2}\d{3}[a-z]?)-[0-9a-z]+$`)
)

// consumerDriveModelNameVariants generates vendor-aware normalized lookup keys
// for a raw model name. Each variant strips one well-known decoration (capacity
// suffix, vendor firmware suffix) so real-world model strings can hit exact
// alias or family entries without resorting to broad regex patterns.
func consumerDriveModelNameVariants(modelName string) []string {
	trimmed := strings.TrimSpace(modelName)
	if trimmed == "" {
		return nil
	}
	base := normalizeConsumerDriveKey(trimmed)

	variants := make([]string, 0, 2)
	appendVariant := func(candidate string) {
		key := normalizeConsumerDriveKey(candidate)
		if key == "" || key == base {
			return
		}
		for _, existing := range variants {
			if existing == key {
				return
			}
		}
		variants = append(variants, key)
	}

	if stripped := consumerDriveCapacitySuffixPattern.ReplaceAllString(trimmed, ""); stripped != trimmed {
		appendVariant(stripped)
	}
	if m := consumerDriveWdcFirmwarePattern.FindStringSubmatch(trimmed); m != nil {
		appendVariant(m[1])
	}
	if m := consumerDriveSeagateFirmwarePattern.FindStringSubmatch(trimmed); m != nil {
		appendVariant(m[1])
	}
	return variants
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
