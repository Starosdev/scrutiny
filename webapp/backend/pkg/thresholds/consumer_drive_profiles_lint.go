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
// of other families, duplicate patterns, redundant aliases, and a missing
// catalog version.
func LintConsumerDriveProfileCatalog(data []byte) (*ConsumerDriveProfileLintResult, error) {
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

// lintPatterns flags duplicate model patterns and patterns that shadow entries
// belonging to a different family (an alias or family name of family B matching
// the pattern of family A would silently reroute drives at the regex stage).
func lintPatterns(catalog *consumerDriveProfileCatalog, result *ConsumerDriveProfileLintResult) {
	type compiledPattern struct {
		family  string
		pattern string
		regex   *regexp.Regexp
	}

	seenPatterns := map[string]string{}
	compiled := make([]compiledPattern, 0, len(catalog.Profiles))
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
		// Compile errors are caught by hard validation; ignore here.
		if regex, err := regexp.Compile(profile.ModelPattern); err == nil {
			compiled = append(compiled, compiledPattern{profile.ModelFamily, profile.ModelPattern, regex})
		}
	}

	for _, cp := range compiled {
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
		for i := range catalog.Profiles {
			other := &catalog.Profiles[i]
			otherKey := normalizeConsumerDriveKey(other.ModelFamily)
			if otherKey == patternFamilyKey {
				continue
			}
			if cp.regex.MatchString(other.ModelFamily) {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"pattern shadowing: pattern %q of family %q matches family name %q",
					cp.pattern, cp.family, other.ModelFamily))
			}
		}
	}
}
