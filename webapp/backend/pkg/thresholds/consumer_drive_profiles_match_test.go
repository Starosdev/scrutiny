package thresholds

import (
	"os"
	"strings"
	"testing"
)

func TestMatchConsumerDriveProfileMethods(t *testing.T) {
	tests := []struct {
		name         string
		modelFamily  string
		modelName    string
		expectFamily string
		expectMethod ProfileMatchMethod
	}{
		{"exact family", "Samsung based SSDs", "", "Samsung based SSDs", ProfileMatchMethodModelFamily},
		{"exact model alias", "", "Hitachi HDS721050DLE630", "Hitachi Deskstar 7K1000.D", ProfileMatchMethodModelName},
		{"capacity variant via vendor normalization", "", "Samsung SSD 870 EVO 2TB", "Samsung SSD 870 EVO", ProfileMatchMethodModelNameNormalized},
		{"regex pattern fallback", "", "ST2000DM001-9YN164", "Seagate Barracuda 7200.12", ProfileMatchMethodModelPattern},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := MatchConsumerDriveProfile("ATA", tt.modelFamily, tt.modelName, nil)
			if match == nil {
				t.Fatalf("expected a match")
			}
			if !match.Applied {
				t.Fatalf("expected match to be applied, skip reason %q", match.SkipReason)
			}
			if match.Profile.ModelFamily != tt.expectFamily {
				t.Fatalf("expected family %q, got %q", tt.expectFamily, match.Profile.ModelFamily)
			}
			if match.Method != tt.expectMethod {
				t.Fatalf("expected method %q, got %q", tt.expectMethod, match.Method)
			}
			if match.CatalogVersion == "" {
				t.Fatalf("expected catalog version to be set")
			}
		})
	}
}

func TestMatchConsumerDriveProfileNoMatch(t *testing.T) {
	if match := MatchConsumerDriveProfile("ATA", "", "TOSHIBA DT01ACA100", nil); match != nil {
		t.Fatalf("expected no match, got %q", match.Profile.ModelFamily)
	}
}

func TestMatchConsumerDriveProfileNonAtaProtocol(t *testing.T) {
	if match := MatchConsumerDriveProfile("NVMe", "Samsung based SSDs", "", nil); match != nil {
		t.Fatalf("expected non-ATA protocol to bypass matching")
	}
}

func TestMatchConsumerDriveProfileDenylisted(t *testing.T) {
	denied := ParseConsumerDriveProfileDenylist("Samsung based SSDs")
	match := MatchConsumerDriveProfile("ATA", "Samsung based SSDs", "", denied)
	if match == nil {
		t.Fatalf("expected a skipped match for observability")
	}
	if match.Applied {
		t.Fatalf("expected denylisted match to not be applied")
	}
	if match.SkipReason != ProfileSkipReasonFamilyDenylisted {
		t.Fatalf("expected skip reason %q, got %q", ProfileSkipReasonFamilyDenylisted, match.SkipReason)
	}
}

func TestMatchConsumerDriveProfileDenylistFallsThroughToOtherFamily(t *testing.T) {
	catalogJSON := `{
		"version": "test",
		"profiles": [
			{"protocol":"ATA","source":"test","model_family":"Family A","sample_count":25},
			{"protocol":"ATA","source":"test","model_family":"Family B","sample_count":25}
		],
		"aliases": {"MODEL-1": "Family B"}
	}`
	handle, err := LoadConsumerDriveProfileCatalog([]byte(catalogJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	denied := ParseConsumerDriveProfileDenylist("Family A")
	match := handle.Match("ATA", "Family A", "MODEL-1", denied)
	if match == nil || !match.Applied {
		t.Fatalf("expected fallthrough match to Family B to be applied")
	}
	if match.Profile.ModelFamily != "Family B" {
		t.Fatalf("expected Family B, got %q", match.Profile.ModelFamily)
	}
	if match.Method != ProfileMatchMethodModelName {
		t.Fatalf("expected model_name method, got %q", match.Method)
	}
}

func TestMatchConsumerDriveProfileBelowConfidence(t *testing.T) {
	catalogJSON := `{
		"profiles": [{"protocol":"ATA","source":"test","model_family":"Family A","sample_count":5}],
		"aliases": {}
	}`
	handle, err := LoadConsumerDriveProfileCatalog([]byte(catalogJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	match := handle.Match("ATA", "Family A", "", nil)
	if match == nil {
		t.Fatalf("expected a skipped match for observability")
	}
	if match.Applied {
		t.Fatalf("expected low-confidence match to not be applied")
	}
	if match.SkipReason != ProfileSkipReasonBelowConfidence {
		t.Fatalf("expected skip reason %q, got %q", ProfileSkipReasonBelowConfidence, match.SkipReason)
	}
}

func TestParseConsumerDriveProfileDenylist(t *testing.T) {
	if denied := ParseConsumerDriveProfileDenylist(""); denied != nil {
		t.Fatalf("expected nil denylist for empty input")
	}
	if denied := ParseConsumerDriveProfileDenylist(" , ,"); denied != nil {
		t.Fatalf("expected nil denylist for blank entries")
	}
	denied := ParseConsumerDriveProfileDenylist("Samsung based SSDs, WDC Red Plus")
	if len(denied) != 2 {
		t.Fatalf("expected 2 denied families, got %d", len(denied))
	}
	if _, ok := denied["samsung_based_ssds"]; !ok {
		t.Fatalf("expected normalized samsung key in denylist")
	}
	if _, ok := denied["wdc_red_plus"]; !ok {
		t.Fatalf("expected normalized wdc key in denylist")
	}
}

func TestConsumerDriveModelNameVariants(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		expect    []string
	}{
		{"capacity suffix stripped", "Samsung SSD 870 EVO 2TB", []string{"samsung_ssd_870_evo"}},
		{"wdc firmware suffix stripped", "WDC WD80EFAX-68LHPN0", []string{"wdc_wd80efax"}},
		{"seagate firmware suffix stripped", "ST4000DM000-1F2168", []string{"st4000dm000"}},
		{"no decoration yields no variants", "CT2000MX500SSD1", nil},
		{"empty input", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := consumerDriveModelNameVariants(tt.modelName)
			if len(got) != len(tt.expect) {
				t.Fatalf("expected variants %v, got %v", tt.expect, got)
			}
			for i := range tt.expect {
				if got[i] != tt.expect[i] {
					t.Fatalf("expected variants %v, got %v", tt.expect, got)
				}
			}
		})
	}
}

func TestParseConsumerDriveProfilesValidationRules(t *testing.T) {
	tests := []struct {
		name        string
		catalogJSON string
		expectErr   string
	}{
		{
			"severity ordering violation",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":25,
				"ata_counter_severity_overrides":{"5":{"low":0,"moderate":8,"high":4,"critical":24}}}],"aliases":{}}`,
			"severity override violates",
		},
		{
			"observed threshold low greater than high",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":25,
				"ata_observed_thresholds":{"5":[{"low":10,"high":2,"annual_failure_rate":0.1}]}}],"aliases":{}}`,
			"low > high",
		},
		{
			"annual failure rate out of range",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":25,
				"ata_observed_thresholds":{"5":[{"low":0,"high":4,"annual_failure_rate":1.5}]}}],"aliases":{}}`,
			"annual_failure_rate outside",
		},
		{
			"error interval wrong length",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":25,
				"ata_observed_thresholds":{"5":[{"low":0,"high":4,"annual_failure_rate":0.1,"error_interval":[0.1]}]}}],"aliases":{}}`,
			"error_interval must have exactly 2 values",
		},
		{
			"error interval not ordered",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":25,
				"ata_observed_thresholds":{"5":[{"low":0,"high":4,"annual_failure_rate":0.1,"error_interval":[0.3,0.1]}]}}],"aliases":{}}`,
			"error_interval is not ordered",
		},
		{
			"non-ATA protocol",
			`{"profiles":[{"protocol":"NVMe","source":"t","model_family":"F","sample_count":25}],"aliases":{}}`,
			"unsupported protocol",
		},
		{
			"missing source",
			`{"profiles":[{"protocol":"ATA","source":"","model_family":"F","sample_count":25}],"aliases":{}}`,
			"missing source",
		},
		{
			"non-positive sample count",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":0}],"aliases":{}}`,
			"non-positive sample_count",
		},
		{
			"negative min samples",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":25,"min_samples":-1}],"aliases":{}}`,
			"negative min_samples",
		},
		{
			"invalid regex pattern",
			`{"profiles":[{"protocol":"ATA","source":"t","model_family":"F","sample_count":25,"model_pattern":"^(["}],"aliases":{}}`,
			"compile model_pattern",
		},
		{
			"duplicate family",
			`{"profiles":[
				{"protocol":"ATA","source":"t","model_family":"Family A","sample_count":25},
				{"protocol":"ATA","source":"t","model_family":"family a","sample_count":25}],"aliases":{}}`,
			"duplicate profile family",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConsumerDriveProfiles([]byte(tt.catalogJSON))
			if err == nil || !strings.Contains(err.Error(), tt.expectErr) {
				t.Fatalf("expected error containing %q, got %v", tt.expectErr, err)
			}
		})
	}
}

func TestLintConsumerDriveProfileCatalogWarnings(t *testing.T) {
	catalogJSON := `{
		"profiles": [
			{"protocol":"ATA","source":"t","model_family":"Family A","sample_count":25,"min_samples":100},
			{"protocol":"ATA","source":"t","model_family":"Family B","sample_count":25,"model_pattern":"^MODEL-.*$"},
			{"protocol":"ATA","source":"t","model_family":"Family C","sample_count":25,"model_pattern":"^MODEL-.*$"}
		],
		"aliases": {
			"Family A": "Family A",
			"MODEL-9": "Family A"
		}
	}`

	result, err := LintConsumerDriveProfileCatalog([]byte(catalogJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectWarnings := []string{
		"catalog has no version",
		"dead entry",
		"redundant alias",
		"duplicate pattern",
		"pattern shadowing",
	}
	joined := strings.Join(result.Warnings, "\n")
	for _, expected := range expectWarnings {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected a warning containing %q, got:\n%s", expected, joined)
		}
	}
}

func TestBundledCatalogLintClean(t *testing.T) {
	result, err := LintConsumerDriveProfileCatalog(consumerDriveProfilesJSON)
	if err != nil {
		t.Fatalf("bundled catalog failed validation: %v", err)
	}
	if len(result.Warnings) > 0 {
		t.Fatalf("bundled catalog has lint warnings:\n%s", strings.Join(result.Warnings, "\n"))
	}
}

func TestBundledCatalogIsCanonical(t *testing.T) {
	canonical, err := CanonicalizeConsumerDriveProfileCatalog(consumerDriveProfilesJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(canonical) != string(consumerDriveProfilesJSON) {
		t.Fatalf("bundled catalog is not canonical; run `make catalog-fix`")
	}
}

func TestBundledCatalogExpectedMatchFixtures(t *testing.T) {
	fixtureData, err := os.ReadFile("testdata/consumer_drive_profile_fixtures.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	handle, err := LoadConsumerDriveProfileCatalog(consumerDriveProfilesJSON)
	if err != nil {
		t.Fatalf("load bundled catalog: %v", err)
	}
	failures, err := CheckConsumerDriveProfileFixtures(handle, fixtureData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) > 0 {
		t.Fatalf("fixture failures:\n%s", strings.Join(failures, "\n"))
	}
}

func TestConsumerDriveProfileCatalogVersionSet(t *testing.T) {
	if ConsumerDriveProfileCatalogVersion() == "" {
		t.Fatalf("bundled catalog must declare a version")
	}
}
