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
| 1 | `model_family` | Exact match on the `model_family` reported by `smartctl` |
| 2 | `model_name` | Exact match on the normalized `model_name` (alias table) |
| 3 | `model_name_normalized` | Exact match after vendor-aware normalization (see below) |
| 4 | `model_pattern` | Regex `model_pattern` fallback |

Exact hits (`model_family`, `model_name`) are the strongest signals; `model_name_normalized` is nearly as strong because it still resolves to an exact catalog entry; `model_pattern` is the weakest because a regex can overreach.

If no vetted profile matches, Scrutiny falls back to the existing generic ATA rules.

### Vendor-aware normalization

The `model_name_normalized` stage strips well-known decorations from real-world model strings before retrying an exact lookup:

- capacity suffixes: `Samsung SSD 870 EVO 2TB` -> `Samsung SSD 870 EVO`
- WDC firmware suffixes: `WDC WD80EFAX-68LHPN0` -> `WDC WD80EFAX`
- Seagate firmware suffixes: `ST4000DM000-1F2168` -> `ST4000DM000`

This catches common variants (new capacity sizes, new firmware revisions) without broad regex rules that could create false positives.

### Catalog source

The bundled catalog is validated at startup and loaded from:

- [consumer_drive_profiles.json](../webapp/backend/pkg/thresholds/consumer_drive_profiles.json)

The catalog is shipped in-repo and embedded into the backend binary. Scrutiny does not fetch profile data at runtime. The catalog carries a `version` string that is reported through the API for provenance.

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
3. A denylisted or low-confidence match does not end the lookup: weaker match stages are still consulted, so a drive whose family is denylisted can still match a different (allowed) family through its model name. In practice the stages almost always resolve to the same family.
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
- `consumer_drive_profile_family`

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
   - a missing catalog `version`
3. **Expected-match fixtures** ([testdata/consumer_drive_profile_fixtures.json](../webapp/backend/pkg/thresholds/testdata/consumer_drive_profile_fixtures.json)):
   - representative real-world model strings pinned to their expected family and match method
   - matched and unmatched cases both covered, so catalog edits that unintentionally change existing matches fail fast
   - the same fixtures run in `go test ./webapp/backend/pkg/thresholds/`

To add or update catalog entries:

1. Edit `consumer_drive_profiles.json` (add the profile, aliases, and optionally a narrow `model_pattern`).
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
