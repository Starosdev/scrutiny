// Command catalog-lint validates, lints, and canonicalizes the bundled ATA
// consumer drive profile catalog. Run it locally before merging catalog edits:
//
//	go run ./webapp/backend/cmd/catalog-lint                # validate + lint + fixtures
//	go run ./webapp/backend/cmd/catalog-lint -write         # also rewrite the catalog in canonical form
//	go run ./webapp/backend/cmd/catalog-lint -strict        # treat lint warnings as failures
//
// The tool exits non-zero on validation errors, fixture mismatches, or (with
// -strict) lint warnings. The canonical output written by -write is the exact
// byte form embedded into the backend binary at build time.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
)

const (
	defaultCatalogPath  = "webapp/backend/pkg/thresholds/consumer_drive_profiles.json"
	defaultFixturesPath = "webapp/backend/pkg/thresholds/testdata/consumer_drive_profile_fixtures.json"
)

func main() {
	catalogPath := flag.String("catalog", defaultCatalogPath, "path to the profile catalog JSON")
	fixturesPath := flag.String("fixtures", defaultFixturesPath, "path to the expected-match fixtures JSON (empty to skip)")
	write := flag.Bool("write", false, "rewrite the catalog file in canonical form")
	strict := flag.Bool("strict", false, "treat lint warnings as failures")
	flag.Parse()

	if err := run(*catalogPath, *fixturesPath, *write, *strict); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
}

func run(catalogPath, fixturesPath string, write, strict bool) error {
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("read catalog: %w", err)
	}

	// Hard validation + lint warnings.
	lintResult, err := thresholds.LintConsumerDriveProfileCatalog(data)
	if err != nil {
		return fmt.Errorf("catalog validation failed: %w", err)
	}
	for _, warning := range lintResult.Warnings {
		fmt.Printf("WARN: %s\n", warning)
	}

	handle, err := thresholds.LoadConsumerDriveProfileCatalog(data)
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}
	fmt.Printf("OK: catalog is valid (version %q)\n", handle.Version())

	// Expected-match fixtures.
	if fixturesPath != "" {
		if err := runFixtures(handle, fixturesPath); err != nil {
			return err
		}
	}

	// Canonical form check / rewrite.
	canonical, err := thresholds.CanonicalizeConsumerDriveProfileCatalog(data)
	if err != nil {
		return fmt.Errorf("canonicalize catalog: %w", err)
	}
	if write {
		if !bytes.Equal(canonical, data) {
			if err := os.WriteFile(catalogPath, canonical, 0644); err != nil {
				return fmt.Errorf("write canonical catalog: %w", err)
			}
			fmt.Printf("OK: rewrote %s in canonical form\n", catalogPath)
		} else {
			fmt.Println("OK: catalog already in canonical form")
		}
	} else if !bytes.Equal(canonical, data) {
		fmt.Println("WARN: catalog is not in canonical form; run with -write to normalize it")
		if strict {
			return fmt.Errorf("catalog is not in canonical form")
		}
	}

	if strict && len(lintResult.Warnings) > 0 {
		return fmt.Errorf("%d lint warning(s) in strict mode", len(lintResult.Warnings))
	}
	return nil
}

func runFixtures(handle *thresholds.ConsumerDriveCatalogHandle, fixturesPath string) error {
	fixtureData, err := os.ReadFile(fixturesPath)
	if err != nil {
		return fmt.Errorf("read fixtures: %w", err)
	}
	failures, err := thresholds.CheckConsumerDriveProfileFixtures(handle, fixtureData)
	if err != nil {
		return err
	}
	for _, failure := range failures {
		fmt.Fprintf(os.Stderr, "FIXTURE FAIL: %s\n", failure)
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d fixture failure(s)", len(failures))
	}
	fmt.Println("OK: all expected-match fixtures pass")
	return nil
}
