# CONTEXT.md

Orientation and domain vocabulary for anyone (human or agent) working on Scrutiny.
Read this before making changes. For hard rules see [docs/agents/invariants.md](docs/agents/invariants.md),
for per-file failure modes see [docs/agents/danger-zones.md](docs/agents/danger-zones.md),
for the reasoning behind major decisions see [docs/adr/](docs/adr/).

## What this system is

Scrutiny monitors disk health. Collectors run near the disks, shell out to
`smartctl` (or `zpool`, `mdadm`, `btrfs`, `fio`), and POST results to a central
web/API server. The server evaluates health against real-world failure-rate
data, stores history, renders a dashboard, and sends notifications. This repo
(`Starosdev/scrutiny`) is an independent fork; the upstream `AnalogJ/scrutiny`
repository is reference-only and never synced from.

## The one diagram that matters

```text
collector (smartctl) --POST--> web/API server --+--> SQLite    (identity, metadata, settings)
collector-zfs (zpool)                            +--> InfluxDB  (time-series: smart, temp, performance)
collector-performance (fio)                      +--> notify    (shoutrrr, digests, dedup)
collector-btrfs / -mdadm                         +--> MQTT      (Home Assistant discovery)
                                                 +--> Prometheus /api/metrics
frontend (Angular) <--/api/*-- web/API server
```

## Domain vocabulary

Use these terms exactly. They map to specific code concepts and mixing them up
causes real bugs.

- **WWN** — World Wide Name. The collector-side device identifier. Computed
  from smartctl NAA/OUI/ID parts when available, else falls back to ghw block
  data, else to the serial number. Always lowercase. May be empty or duplicated
  across devices. It is the tag key for all InfluxDB time-series data.
- **device_id** — deterministic UUIDv5 over `model:serial:wwn` (lowercased,
  trimmed), generated in `webapp/backend/pkg/deviceid/`. The SQLite primary
  key and the stable identity for MQTT/Prometheus. Never confuse with WWN:
  SQLite and HA key on device_id; InfluxDB keys on WWN.
- **AttributeStatus** — per-SMART-attribute health bitmask (`pkg/constants.go`):
  Passed=0, FailedSmart=1, WarningScrutiny=2, FailedScrutiny=4, InvalidValue=8.
- **DeviceStatus** — per-device health bitmask, *different bit values*:
  Passed=0, FailedSmart=1, FailedScrutiny=2. A warn-only device is
  DeviceStatusPassed — warnings never set a device bit.
- **status_threshold** — user setting masking which failure sources count
  (Smart=1, Scrutiny=2, Both=3). The UI verdict is `device_status & threshold`.
- **Observed thresholds** — Backblaze-derived per-attribute failure-rate
  buckets in `pkg/thresholds/`. These tables *are* the health verdict.
- **Consumer drive profile** — model-family-specific observed-threshold
  overrides (`consumer_drive_profiles.json`), matched strongest-first
  (family, alias, vendor-normalized, regex), gated by a min-20-sample
  confidence rule. Applied at evaluation time only; never persisted.
- **Downsampling** — InfluxDB tasks copying daily data into `_weekly`,
  `_monthly`, `_yearly` buckets on cron. Daily retention (15d) must outlive
  the weekly task lookback (~14d).
- **Omnibus** — the all-in-one Docker image (web + collector + InfluxDB,
  s6-overlay init). Owns the bare `:latest`/`:beta`/`:develop` tags.
- **Hub/spoke** — split deployment: `latest-web` hub, `latest-collector`
  spokes.
- **host_id** — optional collector label for grouping devices per host. Only
  participates in device identity for metadata-less devices (fallback hash).

## Load-bearing asymmetries (things that look wrong but are deliberate)

- SQLite operations key on `device_id`; InfluxDB queries filter on the
  `device_wwn` tag. Every summary/temperature path re-keys in Go via a
  wwn-to-device_id map.
- The collector tolerates two *different* smartctl exit-code masks: `0xBF`
  during detection, `0x03` during collection. Do not unify them.
- Device paths keep their case (filesystems are case-sensitive); WWNs are
  forced lowercase (identity tokens must be canonical). Opposite rules,
  both deliberate.
- SMART downsampling aggregates with `last`, temperature with `mean`.
- Bool settings that default to true are seeded by DB migration, never by
  `ApplyDefaults` (Go zero-value `false` is indistinguishable from "user
  chose false").
- The web binary is named `scrutiny` inside Docker images but
  `scrutiny-web-*` in release artifacts. Same binary, two names, two build
  paths.
- CI builds with the Go version in `go.mod`; Docker images pin their own
  (newer) `golang` base image. They drift by design; bump both when bumping.

## Where things live

| Concern | Location |
| --- | --- |
| HTTP handlers, routes | `webapp/backend/pkg/web/` |
| Persistence (SQLite + InfluxDB) | `webapp/backend/pkg/database/` |
| Health evaluation | `webapp/backend/pkg/models/measurements/`, `pkg/thresholds/` |
| Device identity | `webapp/backend/pkg/deviceid/` |
| Notifications | `webapp/backend/pkg/notify/` |
| Home Assistant MQTT | `webapp/backend/pkg/mqtt/` |
| Collector detection | `collector/pkg/detect/` |
| Collector orchestration | `collector/pkg/collector/` |
| Frontend | `webapp/frontend/src/app/` |
| Release/version mechanics | `.releaserc.json`, `webapp/backend/pkg/version/version.go` |
| Docker/omnibus init | `docker/`, `rootfs/` |

## Ground rules recap

- All GitHub operations target `Starosdev/scrutiny` (see CLAUDE.md).
- Releases are manual-only; pushing to master builds images but releases
  nothing.
- No emojis, no AI attribution anywhere.
- Work in a worktree under `~/worktrees/scrutiny/`, never in the main clone.
