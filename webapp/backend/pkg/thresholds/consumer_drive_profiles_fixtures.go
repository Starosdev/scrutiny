package thresholds

import (
	"encoding/json"
	"fmt"
)

// ConsumerDriveProfileFixture is one expected-match regression case for the
// profile catalog. Fixtures pin the behavior of representative real-world
// model strings so catalog edits that unintentionally change matches fail fast.
type ConsumerDriveProfileFixture struct {
	Description string `json:"description,omitempty"`

	// Inputs, as reported by smartctl.
	Protocol    string `json:"protocol"`
	ModelFamily string `json:"model_family,omitempty"`
	ModelName   string `json:"model_name,omitempty"`

	// ExpectFamily is the catalog family the drive must resolve to (when matched).
	ExpectFamily string `json:"expect_family,omitempty"`

	// ExpectMethod is the required match method (when matched).
	ExpectMethod string `json:"expect_method,omitempty"`

	// ExpectMatched is true when the catalog should recognize the drive.
	ExpectMatched bool `json:"expect_matched"`

	// ExpectApplied is true when the matched profile should actually be used.
	ExpectApplied bool `json:"expect_applied"`
}

// CheckConsumerDriveProfileFixtures runs every fixture against the catalog and
// returns a list of human-readable failures. An empty list means all fixtures
// pass. The error return covers malformed fixture documents only.
func CheckConsumerDriveProfileFixtures(handle *ConsumerDriveCatalogHandle, fixtureData []byte) ([]string, error) {
	var fixtures []ConsumerDriveProfileFixture
	if err := json.Unmarshal(fixtureData, &fixtures); err != nil {
		return nil, fmt.Errorf("unmarshal fixtures: %w", err)
	}

	var failures []string
	for i, fixture := range fixtures {
		label := fixture.Description
		if label == "" {
			label = fmt.Sprintf("fixture #%d (family=%q model=%q)", i, fixture.ModelFamily, fixture.ModelName)
		}
		failures = append(failures, checkConsumerDriveProfileFixture(handle, &fixtures[i], label)...)
	}
	return failures, nil
}

func checkConsumerDriveProfileFixture(handle *ConsumerDriveCatalogHandle, fixture *ConsumerDriveProfileFixture, label string) []string {
	match := handle.Match(fixture.Protocol, fixture.ModelFamily, fixture.ModelName, nil)

	if !fixture.ExpectMatched {
		if match != nil {
			return []string{fmt.Sprintf("%s: expected no catalog match, got family %q via %s", label, match.Profile.ModelFamily, match.Method)}
		}
		return nil
	}

	if match == nil {
		return []string{fmt.Sprintf("%s: expected a catalog match, got none", label)}
	}

	var failures []string
	if fixture.ExpectFamily != "" && match.Profile.ModelFamily != fixture.ExpectFamily {
		failures = append(failures, fmt.Sprintf("%s: expected family %q, got %q", label, fixture.ExpectFamily, match.Profile.ModelFamily))
	}
	if fixture.ExpectMethod != "" && string(match.Method) != fixture.ExpectMethod {
		failures = append(failures, fmt.Sprintf("%s: expected match method %q, got %q", label, fixture.ExpectMethod, match.Method))
	}
	if match.Applied != fixture.ExpectApplied {
		failures = append(failures, fmt.Sprintf("%s: expected applied=%t, got applied=%t (skip reason %q)", label, fixture.ExpectApplied, match.Applied, match.SkipReason))
	}
	return failures
}

// CanonicalizeConsumerDriveProfileCatalog re-emits a catalog document in the
// canonical bundled format: 2-space indentation, version first, profiles in
// declared order, aliases sorted by key (Go's JSON encoder sorts map keys).
// The output is the exact byte form that should be committed as the embedded
// runtime catalog.
func CanonicalizeConsumerDriveProfileCatalog(data []byte) ([]byte, error) {
	if err := ValidateConsumerDriveProfileCatalog(data); err != nil {
		return nil, err
	}
	var catalog consumerDriveProfileCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("unmarshal consumer drive profiles: %w", err)
	}

	//nolint:govet // field order defines the canonical JSON layout of the catalog file
	ordered := struct {
		Version  string                 `json:"version,omitempty"`
		Profiles []ConsumerDriveProfile `json:"profiles"`
		Aliases  map[string]string      `json:"aliases"`
	}{
		Version:  catalog.Version,
		Profiles: catalog.Profiles,
		Aliases:  catalog.Aliases,
	}

	out, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
