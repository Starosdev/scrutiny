package thresholds

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ConsumerDriveProfileLintResult holds the outcome of a full catalog lint run.
// Errors are hard validation failures (the catalog cannot be loaded); warnings
// are quality issues that do not prevent loading but should be reviewed before
// merging catalog changes.
type ConsumerDriveProfileLintResult struct {
	Warnings []string
}

// LintConsumerDriveProfileCatalog runs the full validation and lint pass over a
// catalog document. A non-nil error means the catalog is structurally invalid
// and would fail to load at startup. Warnings flag quality issues: dead entries
// that can never pass their confidence gate, regex patterns that shadow aliases
// of other families, duplicate patterns, redundant aliases, two patterns from
// different families matching one model string, and a missing catalog version.
//
// probes are extra model strings to test the patterns against, over and above
// the catalog's own alias keys. Pass the fixture model names: they are the
// real-world strings the catalog is maintained against, and a pattern overlap
// only matters for a string some drive actually reports.
func LintConsumerDriveProfileCatalog(data []byte, probes ...string) (*ConsumerDriveProfileLintResult, error) {
	// Hard validation first: same path as startup loading.
	if _, err := parseConsumerDriveProfiles(data); err != nil {
		return nil, err
	}

	var catalog consumerDriveProfileCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("unmarshal consumer drive profiles: %w", err)
	}

	result := &ConsumerDriveProfileLintResult{}
	if strings.TrimSpace(catalog.Version) == "" {
		result.Warnings = append(result.Warnings, "catalog has no version field; set a version so API provenance can report it")
	}

	lintDeadEntries(&catalog, result)
	lintRedundantAliases(&catalog, result)
	lintPatterns(&catalog, result)
	lintPatternOverlap(&catalog, probes, result)

	sort.Strings(result.Warnings)
	return result, nil
}

// lintDeadEntries flags profiles that fail their own confidence gate: they are
// bundled but can never be applied, so they are dead weight in the catalog.
func lintDeadEntries(catalog *consumerDriveProfileCatalog, result *ConsumerDriveProfileLintResult) {
	for i := range catalog.Profiles {
		profile := &catalog.Profiles[i]
		if !profile.MeetsConfidenceThreshold() {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"dead entry: profile %q has sample_count %d below its confidence gate of %d and can never be applied",
				profile.ModelFamily, profile.SampleCount, profile.EffectiveMinSamples()))
		}
	}
}

// lintRedundantAliases flags aliases that normalize to the same key as the
// family they point to; the family-stage lookup already covers them.
func lintRedundantAliases(catalog *consumerDriveProfileCatalog, result *ConsumerDriveProfileLintResult) {
	for alias, family := range catalog.Aliases {
		if normalizeConsumerDriveKey(alias) == normalizeConsumerDriveKey(family) {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"redundant alias: %q normalizes to the same key as its family %q", alias, family))
		}
	}
}

// compiledLintPattern pairs a profile's compiled model pattern with its family
// for cross-entry shadowing checks.
type compiledLintPattern struct {
	regex   *regexp.Regexp
	family  string
	pattern string
}

// lintPatterns flags duplicate model patterns and patterns that shadow entries
// belonging to a different family. The pattern stage runs before the family
// stage, so a pattern of family A that matches a model string family B declares
// reroutes those drives to A. An exact alias of B still wins; B's family name
// no longer does.
func lintPatterns(catalog *consumerDriveProfileCatalog, result *ConsumerDriveProfileLintResult) {
	for _, cp := range collectLintPatterns(catalog, result) {
		lintPatternShadowsAliases(catalog, cp, result)
		lintPatternShadowsFamilies(catalog, cp, result)
	}
}

// collectLintPatterns compiles every profile's model pattern, warning on
// duplicates. Compile errors are caught by hard validation and skipped here.
func collectLintPatterns(catalog *consumerDriveProfileCatalog, result *ConsumerDriveProfileLintResult) []compiledLintPattern {
	seenPatterns := map[string]string{}
	compiled := make([]compiledLintPattern, 0, len(catalog.Profiles))
	for i := range catalog.Profiles {
		profile := &catalog.Profiles[i]
		if profile.ModelPattern == "" {
			continue
		}
		if otherFamily, exists := seenPatterns[profile.ModelPattern]; exists {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"duplicate pattern: %q is declared by both %q and %q", profile.ModelPattern, otherFamily, profile.ModelFamily))
		} else {
			seenPatterns[profile.ModelPattern] = profile.ModelFamily
		}
		if regex, err := regexp.Compile(profile.ModelPattern); err == nil {
			compiled = append(compiled, compiledLintPattern{regex, profile.ModelFamily, profile.ModelPattern})
		}
	}
	return compiled
}

// lintPatternOverlap warns when the patterns of two different families both match
// one model string. The matcher appends every matching regex profile as a
// candidate and applies the first (consumer_drive_profiles.go, candidates), so
// the winner is decided by declaration order in the catalog file. That is silent
// and it is not a property anyone editing the catalog would think to check, so
// the overlap itself is the defect: narrow one pattern or move the model to an
// alias. Regex intersection is undecidable in general, which is why this tests
// concrete strings rather than the patterns against each other.
func lintPatternOverlap(catalog *consumerDriveProfileCatalog, probes []string, result *ConsumerDriveProfileLintResult) {
	compiled := make([]compiledLintPattern, 0, len(catalog.Profiles))
	for i := range catalog.Profiles {
		profile := &catalog.Profiles[i]
		if profile.ModelPattern == "" {
			continue
		}
		if regex, err := regexp.Compile(profile.ModelPattern); err == nil {
			compiled = append(compiled, compiledLintPattern{regex, profile.ModelFamily, profile.ModelPattern})
		}
	}
	if len(compiled) < 2 {
		return
	}

	seen := map[string]struct{}{}
	for _, probe := range consumerDriveProfileProbes(catalog, probes) {
		var matched []compiledLintPattern
		for _, cp := range compiled {
			if cp.regex.MatchString(probe) {
				matched = append(matched, cp)
			}
		}
		if len(matched) < 2 {
			continue
		}
		families := make([]string, 0, len(matched))
		for _, cp := range matched {
			families = append(families, fmt.Sprintf("%q (%s)", cp.family, cp.pattern))
		}
		sort.Strings(families)
		key := strings.Join(families, " | ")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"pattern overlap: model %q matches the patterns of more than one family (%s); the catalog file's declaration order decides which applies",
			probe, strings.Join(families, ", ")))
	}
}

// consumerDriveProfileProbes returns the model strings to test patterns against:
// every alias key in the catalog, every profile's declared model name, and
// whatever the caller passed in. Duplicates and blanks are dropped.
func consumerDriveProfileProbes(catalog *consumerDriveProfileCatalog, extra []string) []string {
	seen := map[string]struct{}{}
	probes := make([]string, 0, len(catalog.Aliases)+len(extra))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		probes = append(probes, value)
	}
	for alias := range catalog.Aliases {
		add(alias)
	}
	for i := range catalog.Profiles {
		add(catalog.Profiles[i].ModelName)
	}
	for _, value := range extra {
		add(value)
	}
	sort.Strings(probes)
	return probes
}

// lintPatternShadowsAliases warns when a pattern matches an alias that belongs
// to a different family.
func lintPatternShadowsAliases(catalog *consumerDriveProfileCatalog, cp compiledLintPattern, result *ConsumerDriveProfileLintResult) {
	patternFamilyKey := normalizeConsumerDriveKey(cp.family)
	for alias, family := range catalog.Aliases {
		if normalizeConsumerDriveKey(family) == patternFamilyKey {
			continue
		}
		if cp.regex.MatchString(alias) {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"pattern shadowing: pattern %q of family %q matches alias %q which belongs to family %q",
				cp.pattern, cp.family, alias, family))
		}
	}
}

// lintPatternShadowsFamilies warns when a pattern matches another profile's
// family name. A family name is reachable as a lookup key only through the
// model_name_normalized stage, which runs above the pattern stage, so this rule
// covers that narrow collision and not the broader case of a pattern matching
// another family's models, which a regex comparison cannot decide.
func lintPatternShadowsFamilies(catalog *consumerDriveProfileCatalog, cp compiledLintPattern, result *ConsumerDriveProfileLintResult) {
	patternFamilyKey := normalizeConsumerDriveKey(cp.family)
	for i := range catalog.Profiles {
		other := &catalog.Profiles[i]
		if normalizeConsumerDriveKey(other.ModelFamily) == patternFamilyKey {
			continue
		}
		if cp.regex.MatchString(other.ModelFamily) {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"pattern shadowing: pattern %q of family %q matches family name %q",
				cp.pattern, cp.family, other.ModelFamily))
		}
	}
}
