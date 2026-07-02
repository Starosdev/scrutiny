# 0001. Dual store: SQLite for identity/config, InfluxDB for time-series

Status: accepted
Date: 2026-07-02 (documents a decision inherited at fork time and extended since)

## Context

Drive health data has two shapes: relational metadata that needs transactions
(device records, settings, overrides, notification URLs) and high-volume
time-series (SMART attributes, temperature, performance benchmarks) that needs
retention policies and downsampling. Neither store handles both well.

## Decision

- SQLite (GORM) holds identity, metadata, and configuration. It is the source
  of truth for device existence. Keyed by `device_id`.
- InfluxDB holds the `smart`, `temp`, and `performance` measurements. Keyed by
  the `device_wwn` tag (points also carry `device_id`, but queries filter on
  WWN).
- Four buckets derived by suffix from `web.influxdb.bucket`: base (daily,
  15d retention), `_weekly` (9wk), `_monthly` (25mo), `_yearly` (infinite).
  Server-created Flux tasks downsample between them on cron; SMART fields
  aggregate with `last`, temperature with `mean`.

## Consequences

- Every read that joins the two stores re-keys in Go through a
  WWN-to-device_id map (`scrutiny_repository.go`). InfluxDB rows with no
  matching SQLite device are dropped from summaries.
- The cross-store key mismatch is the sharpest edge in the codebase: SQLite
  moved its primary key to `device_id` (migration `m20260401000000`) but
  InfluxDB stayed on WWN for backward compatibility with existing user data.
  Devices with empty or duplicated WWNs therefore share/collide InfluxDB
  history, and `DeleteDevice` can only clean InfluxDB when WWN is non-empty.
- Bucket suffixes `_weekly`/`_monthly`/`_yearly` are string literals repeated
  across ~6 sites (ensure, lookup, delete, merge, downsample). There is no
  single constant; they must be changed together.
- Daily retention (15d) must stay longer than the weekly downsample lookback
  (~14d) or data expires before it is aggregated.
- Measurement names `smart`/`temp` and the `attr.<id>.<subfield>` field
  pattern are wire contracts across all Flux queries, downsample scripts, and
  the summary field list. Renaming any of them is a breaking migration, not a
  refactor.
