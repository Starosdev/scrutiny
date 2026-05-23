package thresholds

import (
	"strings"
	"testing"
)

func TestLookupConsumerDriveProfileByFamily(t *testing.T) {
	profile, ok := LookupConsumerDriveProfile("ATA", "Samsung based SSDs", "")
	if !ok || profile == nil {
		t.Fatalf("expected family profile match")
	}
	if profile.ModelFamily != "Samsung based SSDs" {
		t.Fatalf("unexpected model family: %s", profile.ModelFamily)
	}
}

func TestLookupConsumerDriveProfileByModelFallback(t *testing.T) {
	profile, ok := LookupConsumerDriveProfile("ATA", "", "Hitachi HDS721050DLE630")
	if !ok || profile == nil {
		t.Fatalf("expected model fallback profile match")
	}
	if profile.ModelFamily != "Hitachi Deskstar 7K1000.D" {
		t.Fatalf("unexpected model family: %s", profile.ModelFamily)
	}
}

func TestLookupConsumerDriveProfileByUnderscoredModelFallback(t *testing.T) {
	profile, ok := LookupConsumerDriveProfile("ATA", "", "WDC_WD140EDFZ-11A0VA0")
	if !ok || profile == nil {
		t.Fatalf("expected underscored model fallback profile match")
	}
	if profile.ModelFamily != "WDC Red Plus" {
		t.Fatalf("unexpected model family: %s", profile.ModelFamily)
	}
}

func TestLookupConsumerDriveProfileBySeagateModelFallback(t *testing.T) {
	profile, ok := LookupConsumerDriveProfile("ATA", "", "ST6000DX000-1H217Z")
	if !ok || profile == nil {
		t.Fatalf("expected seagate model fallback profile match")
	}
	if profile.ModelFamily != "Seagate Desktop SSHD" {
		t.Fatalf("unexpected model family: %s", profile.ModelFamily)
	}
}

func TestLookupConsumerDriveProfileByWdcBlueModelFallback(t *testing.T) {
	profile, ok := LookupConsumerDriveProfile("ATA", "", "WD10EZEX-08WN4A0")
	if !ok || profile == nil {
		t.Fatalf("expected WDC blue model fallback profile match")
	}
	if profile.ModelFamily != "WDC Caviar Blue" {
		t.Fatalf("unexpected model family: %s", profile.ModelFamily)
	}
}

func TestLookupConsumerDriveProfileBySamsung850ProFallback(t *testing.T) {
	profile, ok := LookupConsumerDriveProfile("ATA", "", "X SSD 850 PRO 128GB")
	if !ok || profile == nil {
		t.Fatalf("expected Samsung 850 PRO model fallback profile match")
	}
	if profile.ModelFamily != "Samsung SSD 850 PRO" {
		t.Fatalf("unexpected model family: %s", profile.ModelFamily)
	}
}

func TestLookupConsumerDriveProfileByRegexFallback(t *testing.T) {
	profile, ok := LookupConsumerDriveProfile("ATA", "", "ST3000DM001-1ER166")
	if !ok || profile == nil {
		t.Fatalf("expected regex-backed profile match")
	}
	if profile.ModelFamily != "Seagate Desktop HDD.15" {
		t.Fatalf("unexpected model family: %s", profile.ModelFamily)
	}
}

func TestLookupConsumerDriveProfileRequiresConfidence(t *testing.T) {
	lowConfidence := ConsumerDriveProfile{SampleCount: 3}
	if lowConfidence.MeetsConfidenceThreshold() {
		t.Fatalf("expected low-confidence profile to fail gate")
	}
}

func TestLookupConsumerDriveProfileProtocolFallback(t *testing.T) {
	if profile, ok := LookupConsumerDriveProfile("NVMe", "Samsung based SSDs", "Samsung SSD 860 EVO 500GB"); ok || profile != nil {
		t.Fatalf("expected non-ATA protocol to bypass consumer profile lookup")
	}
}

func TestNormalizeConsumerDriveKey(t *testing.T) {
	got := normalizeConsumerDriveKey(" Hitachi Deskstar 7K1000.D ")
	if got != "hitachi_deskstar_7k1000_d" {
		t.Fatalf("unexpected normalized key: %s", got)
	}
}

func TestParseConsumerDriveProfilesRejectsConflictingDuplicateAlias(t *testing.T) {
	invalidJSON := `{
		"profiles":[
			{"protocol":"ATA","source":"test","model_family":"Family A","sample_count":25},
			{"protocol":"ATA","source":"test","model_family":"Family B","sample_count":25}
		],
		"aliases":{"Model A":"Family A","Model_A":"Family B"}
	}`
	_, _, _, err := parseConsumerDriveProfiles([]byte(invalidJSON))
	if err == nil || !strings.Contains(err.Error(), "duplicate model alias") {
		t.Fatalf("expected duplicate alias error, got %v", err)
	}
}

func TestParseConsumerDriveProfilesRejectsUnknownFamilyAlias(t *testing.T) {
	invalidJSON := `{
		"profiles":[{"protocol":"ATA","source":"test","model_family":"Family A","sample_count":25}],
		"aliases":{"Model A":"Missing Family"}
	}`
	_, _, _, err := parseConsumerDriveProfiles([]byte(invalidJSON))
	if err == nil || !strings.Contains(err.Error(), "unknown family") {
		t.Fatalf("expected unknown family alias error, got %v", err)
	}
}
