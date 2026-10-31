# Consumer Drive Profiles

Scrutiny can apply vetted ATA consumer drive model or family overrides when evaluating SMART data and computing replacement risk.

This feature is intended to improve interpretation for common consumer SATA HDD and SSD lines without changing behavior for unknown drives.

## What It Does

When enabled, Scrutiny can match an ATA drive to a bundled profile catalog and use that profile to:

- override selected ATA observed-threshold buckets during SMART status evaluation
- override selected ATA counter severity breakpoints during replacement-risk scoring

This applies only to ATA devices. NVMe and SCSI devices continue to use their existing generic logic.

## Matching Behavior

Scrutiny tries to match profiles in this order, strongest first. The stage that matched is reported as the machine-readable `match_method`:

| Order | Match method | Meaning |
| ----- | ------------- | ------- |
| 1 | `model_name` | Exact match on the normalized `model_name` (alias table) |
| 2 | `model_name_normalized` | Exact match after vendor-aware normalization (see below) |
| 3 | `model_pattern` | Regex `model_pattern` match on the model name |
| 4 | `model_family` | Exact match on the `model_family` reported by `smartctl` |

All three model-identity stages rank above the family stage. A model number is stable hardware identity, while `model_family` is `smartctl`'s own classification: it varies with the collector's `drivedb`, it is renamed between `drivedb` generations, and some families (`Samsung based SSDs`, for example) are vendor-wide catch-alls that would otherwise shadow a specific profile for the same drive.

Among the model stages, exact beats derived: an alias hit beats a normalized one, and both beat a regex, because a regex can overreach. The family stage runs last and is the fallback for a drive whose model number the catalog does not list at all.

If no vetted profile matches, Scrutiny falls back to the existing generic ATA rules.

### Vendor-aware normalization

The `model_name_normalized` stage strips well-known decorations from real-world model strings before retrying an exact lookup:

- capacity suffixes: `Samsung SSD 870 EVO 2TB` -> `Samsung SSD 870 EVO`
- WDC firmware suffixes: `WDC WD80EFAX-68LHPN0` -> `WDC WD80EFAX`
- Seagate firmware suffixes: `ST4000DM000-1F2168` -> `ST4000DM000`

This catches common variants (new capacity sizes, new firmware revisions) without broad regex rules that could create false positives.

### Why family names are the fallback and not the key

`smartctl` renames model families between `drivedb` generations, so a catalog keyed to the name current when a profile was derived stops matching the drives it was written for. `smartctl` reports `Western Digital Red (CMR)` where the catalog carries `WDC Red`, and `Toshiba N300/MN NAS HDD` where it carries `Toshiba N300`. Neither reported name is in the catalog, and both of those drives are covered: they match on their model number instead.

That is why a new drive is added to a profile by model number, through an alias or the profile's `model_pattern`, rather than by adding the family name `smartctl` currently reports. A reported family name is also coarser than the profile it would point at: the WD Red CMR family spans both `WDC Red` and `WDC Red Plus`, and `Toshiba N300/MN NAS HDD` merges two product lines, so mapping either onto a single profile would apply thresholds derived from a different population.

Which profile a model number joins follows the models already in that profile, not the drive's current marketing name. `WDC WD80EFZZ` is sold as Red Plus, but its sibling SKU `WDC WD80EFAX` is the same 8TB CMR drive from before WD's 2020 rebrand and sits in `WDC Red`, so `WD80EFZZ` joins it there. Splitting one product line across two profiles would give two drives in the same array different verdicts at the same reallocated-sector count.

### Catalog source

The bundled catalog is validated at startup and loaded from:

- [consumer_drive_profiles.json](../webapp/backend/pkg/thresholds/consumer_drive_profiles.json)

The catalog is shipped in-repo and embedded into the backend binary. Scrutiny does not fetch profile data at runtime. The catalog carries a `version` string that is reported through the API for provenance.

### What `source` has to say

A profile's thresholds are only as good as the population they were derived from, so `source` is the field a later editor has to be able to check a decision against. For a new or edited profile it must name both the dataset and the population: which models the aggregate covers, and at what capacities, for example `linuxhw/SMART aggregate, WD Red CMR 3-8TB (WD30EFRX, WD60EFRX, WD80EFAX, WD80EFZZ)`.

The 35 profiles bundled today predate that rule and all carry the same placeholder, `linuxhw/SMART curated consumer HDD profile` or its SSD equivalent. That is a known gap, not a pattern to copy: it is why a question like "does the WD Red aggregate cover the 8TB models or only the 3-6TB ones" cannot be answered from this repo. There is no lint rule forcing the convention, because it would fail on all 35 existing entries at once.

The rule binds whenever a profile's thresholds are added or changed, since that is when the population is the claim being made. Adding a model number to an existing profile's aliases or pattern does not re-derive anything, so it does not require filling the placeholder in; it does require the product-line argument for that model to be in the commit message.

## Confidence Gate

Profiles only apply when the catalog entry meets the minimum confidence threshold.

- Default minimum sample count: `20`
- A profile may raise that threshold via `min_samples`

Low-confidence entries are ignored and fall back to generic ATA behavior.

## User Control

### Global toggle

The feature is enabled by default and can be disabled globally in the dashboard:

- `Settings` -> `Consumer Drive Profiles`

Persisted setting key:

- `metrics.consumer_drive_profiles_enabled`

When disabled, Scrutiny uses generic ATA rules even for drives that would otherwise match a profile.

### Per-family denylist

Operators can exclude specific profile families without disabling the whole feature:

- `Settings` -> `Consumer Drive Profile Denylist`

Persisted setting key:

- `metrics.consumer_drive_profiles_denylist`

The value is a comma-separated list of family names (for example `Seagate Barracuda 7200.14 (AF), Crucial MX500`). Names are matched case-insensitively after normalization, so `crucial mx500` and `Crucial MX500` are equivalent.

### Precedence rules

1. If the global toggle is off, no profiles are applied. The denylist is irrelevant.
2. If the global toggle is on, a matched profile is applied unless its family is denylisted or it fails its confidence gate.
3. A denylisted or low-confidence match does not end the lookup: weaker match stages are still consulted, so a drive whose profile is denylisted can still match a different (allowed) family through a weaker stage. The stages do not always agree: a model number can resolve to a different profile than the family `smartctl` reports, and when they disagree the model number wins. Denylisting a family therefore moves a drive onto the next stage's family rather than onto generic ATA rules.
4. If nothing survives, the drive uses generic ATA rules.

## Debug and Inspection Surface

`GET /api/device/{id}/drive-profile` reports the full override path for a device without code tracing:

- whether the feature is enabled and what is denylisted
- whether the catalog matched the drive, via which `match_method`, and on which input value
- the matched family, vendor, source, sample count, and confidence gate result
- which ATA attributes would have observed-threshold or counter-severity overrides applied
- a plain-language `fallback_reason` when generic ATA rules are in effect

The catalog match is computed even when the feature is globally disabled, so you can see what would happen; the `applied` flag reflects the effective state.

## UI Behavior

On ATA drive detail pages, the replacement-risk card reports which path was used:

- `Using consumer drive profile: <family>.`
- `Using generic ATA rules. Consumer drive profile overrides are disabled in Settings.`
- `Using generic ATA rules. No vetted consumer drive profile matched this drive.`

## API Behavior

`GET /api/device/{id}/replacement-risk` returns metadata describing whether the feature was enabled and whether a profile was actually applied:

- `consumer_drive_profiles_enabled`
- `consumer_drive_profile_applied`
- `consumer_drive_profile_observed_thresholds`
- `consumer_drive_profile_family`

`consumer_drive_profile_applied` reports that a profile matched. It does not report that the
profile changed any failure rate: most profiles override counter severities only. Use
`consumer_drive_profile_observed_thresholds` for that question. It is true only when the applied
profile carries `ata_observed_thresholds`, which replace the Backblaze-derived buckets for the
attributes the profile covers. The web UI gates its failure-rate attribution on this field, so a
drive matching a severity-only profile is still credited to Backblaze.

When a profile is applied, the response also carries provenance fields (omitted when generic ATA logic is used):

- `consumer_drive_profile_source` - curated dataset description
- `consumer_drive_profile_sample_count` - sample size behind the profile
- `consumer_drive_profile_match_method` - which lookup stage matched
- `consumer_drive_profile_catalog_version` - bundled catalog version

See [API.md](./API.md) and [openapi.yaml](./openapi.yaml) for the current contract.

## Catalog Maintenance Workflow

The catalog is a first-class artifact with its own lint pipeline. Before merging any catalog change, run:

```bash
make catalog-lint     # validate + lint + expected-match fixtures (strict: warnings fail)
make catalog-fix      # rewrite the catalog in canonical form
```

Or invoke the tool directly for more control:

```bash
go run ./webapp/backend/cmd/catalog-lint -help
```

The pipeline enforces three layers:

1. **Hard validation** (also runs at startup; a violation prevents the backend from booting):
   - profiles must declare `model_family`, a non-empty `source`, a positive `sample_count`, and protocol `ATA`
   - duplicate families, conflicting aliases, aliases pointing at unknown families, and invalid regex patterns are rejected
   - counter severity overrides must satisfy `low <= moderate <= high <= critical`
   - observed-threshold buckets must have `low <= high`, an `annual_failure_rate` in `[0, 1]`, and an ordered two-value `error_interval`
2. **Lint warnings** (fail `make catalog-lint`, which runs in strict mode):
   - dead entries that can never pass their own confidence gate
   - regex patterns that shadow aliases or family names belonging to a different family
   - duplicate patterns and redundant aliases
   - two families whose patterns both match one model string, tested against every alias key and every fixture model name
   - a missing catalog `version`
3. **Expected-match fixtures** ([testdata/consumer_drive_profile_fixtures.json](../webapp/backend/pkg/thresholds/testdata/consumer_drive_profile_fixtures.json)):
   - representative real-world model strings pinned to their expected family and match method
   - matched and unmatched cases both covered, so catalog edits that unintentionally change existing matches fail fast
   - the same fixtures run in `go test ./webapp/backend/pkg/thresholds/`

Pattern overlap is the one failure mode a reviewer is unlikely to spot by eye. The matcher collects every pattern that matches a model string and applies the first, in catalog declaration order, so two overlapping patterns from different families resolve by their position in the file. Regex intersection cannot be decided in general, so the lint tests every pattern against a corpus of concrete strings instead: every alias key in the catalog, plus every model name in the fixtures. Adding a model to a second family's pattern therefore fails the lint as soon as that model appears in a fixture, which is a reason to add the fixture in the same commit as the pattern.

To add or update catalog entries:

1. Edit `consumer_drive_profiles.json` (add the profile, aliases, and optionally a narrow `model_pattern`). A profile whose thresholds you add or change needs a `source` that names its population, per the rule above. Add a model number to exactly one family: an alias or a pattern alternative, not both.
2. Bump the catalog `version` field.
3. Add expected-match fixtures for the new entries, including at least one near-miss that must stay unmatched.
4. Run `make catalog-fix` to normalize formatting, then `make catalog-lint` until clean.
5. Commit the catalog and fixtures together.

The generated canonical JSON remains the embedded runtime source of truth; there is no runtime fetching.

## Decision Record: Per-Family Weight Overrides

**Status: evaluated and deferred (2026-07).**

We considered letting profiles override the per-attribute weights used by replacement-risk scoring (for example, weighting attribute 5 more heavily for families with strong reallocated-sector failure correlation).

The bundled catalog's evidence base (curated linuxhw/SMART population aggregates) supports family-level severity breakpoints and observed-threshold buckets, but it does not provide per-attribute failure *correlation* data per family. Deriving weight overrides from it would manufacture precision the source cannot defend: sample counts in the tens-to-thousands range are sufficient to say "this family tolerates a few reallocated sectors" but not "reallocated sectors predict failure 1.4x better than pending sectors for this family."

Per-family weight overrides therefore do not ship. The existing protocol-level weights (informed by Backblaze failure-correlation research across large mixed fleets) remain in effect for all ATA drives. This decision should be revisited only if a dataset with per-family, per-attribute failure correlation becomes available; at that point support should be added narrowly, with tests and an explicit fallback path, per the acceptance criteria in issue #552.
