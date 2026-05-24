# Consumer Drive Profiles

Scrutiny can apply vetted ATA consumer drive model or family overrides when evaluating SMART data and computing replacement risk.

This feature is intended to improve interpretation for common consumer SATA HDD and SSD lines without changing behavior for unknown drives.

## What It Does

When enabled, Scrutiny can match an ATA drive to a bundled profile catalog and use that profile to:

- override selected ATA observed-threshold buckets during SMART status evaluation
- override selected ATA counter severity breakpoints during replacement-risk scoring

This applies only to ATA devices. NVMe and SCSI devices continue to use their existing generic logic.

## Matching Behavior

Scrutiny tries to match profiles in this order:

1. `model_family` reported by `smartctl`
2. normalized exact `model_name`
3. regex-style `model_pattern` fallback

If no vetted profile matches, Scrutiny falls back to the existing generic ATA rules.

The bundled catalog is validated at startup and loaded from:

- [consumer_drive_profiles.json](../webapp/backend/pkg/thresholds/consumer_drive_profiles.json)

The catalog is shipped in-repo and embedded into the backend binary. Scrutiny does not fetch profile data at runtime.

## Confidence Gate

Profiles only apply when the catalog entry meets the minimum confidence threshold.

- Default minimum sample count: `20`
- A profile may raise that threshold if needed

Low-confidence entries are ignored and fall back to generic ATA behavior.

## User Control

The feature is enabled by default and can be disabled globally in the dashboard:

- `Settings` -> `Consumer Drive Profiles`

Persisted setting key:

- `metrics.consumer_drive_profiles_enabled`

When disabled, Scrutiny uses generic ATA rules even for drives that would otherwise match a profile.

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

See [API.md](./API.md) and [openapi.yaml](./openapi.yaml) for the current contract.
