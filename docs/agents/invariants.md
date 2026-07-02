# Invariants

Rules that must hold or the system breaks for real users. Each entry says what
the rule is, where it lives, and what enforces or depends on it. If your
change touches one of these, treat it as a migration, not an edit.
Companion docs: [danger-zones.md](danger-zones.md) (per-file failure modes),
[review-checklist.md](review-checklist.md) (what to check in review),
[../adr/](../adr/) (why these rules exist).

## Identity

1. **The deviceid namespace UUID and input formula are frozen.**
   `webapp/backend/pkg/deviceid/deviceid.go` — namespace constant, the
   `lower(trim(model)):lower(trim(serial)):lower(trim(wwn))` formula, the
   separator, and the `device_name:<name>:host_id:<host>` fallback shape.
   Any change re-identifies every device in every deployment on the next
   collector run. See ADR 0002.
2. **WWN is lowercase everywhere.** Applied independently in
   `collector/pkg/detect/detect.go`, every `devices_*.go` `wwnFallback`,
   the upload URL builders in `collector/pkg/collector/metrics.go`, and
   `performance/collector.go`. Each site is load-bearing for a different
   path; none is redundant.
3. **WWN falls back to serial number** when NAA parts are absent and ghw
   reports "unknown" (`devices_linux.go` and siblings). "WWN" therefore is
   not always a real WWN; never validate its format.
4. **`GenerateWithFallback` argument order is identical at all call sites**
   (`register_devices.go` registration and MQTT discovery paths, migration
   backfill): model, serial, wwn, deviceName, hostId.
5. **Old-collector compatibility:** `/api/device/:id/*` resolves device_id
   first, then WWN (`resolve.go`, deprecated but load-bearing). Collectors
   fall back to WWN URLs when a server does not echo device_id
   (`metrics.go` register response handling). Neither side of this handshake
   may be removed.

## Storage

6. **SQLite keys on device_id; InfluxDB queries key on the `device_wwn` tag.**
   Points carry both tags, but every Flux filter uses WWN. Summary and
   temperature paths re-key in Go. See ADR 0001.
7. **Measurement names `smart`/`temp`/`performance` and field keys
   (`temp`, `power_on_hours`, `attr.<id>.<subfield>` from `Flatten()`) are
   wire contracts** across the summary query, last-seen, downsample scripts,
   and merge (`scrutiny_repository*.go`, `measurements/*.go`).
8. **Bucket suffixes `_weekly`/`_monthly`/`_yearly` repeat as literals in
   ~6 places** (EnsureBuckets, lookupBucketName, DeleteDevice, merge,
   downsample source/dest). Change together or not at all.
9. **Daily retention must exceed the weekly downsample lookback** (15d vs
   ~14d, `config.go` defaults vs `scrutiny_repository_tasks.go`).
10. **Migrations run once per process** (`sync.Once` in
    `scrutiny_repository.go`); additional repositories are constructed
    `WithoutMigration`; tests must call `ResetMigrationGuardForTests()`.
11. **SQLite table-rebuild migrations carry hand-written column lists** that
    replay on fresh installs; they and `models.Device` must stay coherent.
12. **`RegisterDevice` only updates columns in its explicit allow-list.**
    A new Device field is write-once unless added there. `label` is
    conditionally included to preserve UI-set labels.

## Health evaluation

13. **AttributeStatus and DeviceStatus are different bitmasks with different
    bit values** (attribute FailedScrutiny=4, device FailedScrutiny=2).
    `pkg/constants.go`. Passed=0 is the absence of bits: compare with
    `== Passed`, never `Has(x, 0)`. See ADR 0003.
14. **Observed-threshold buckets are Low-exclusive, High-inclusive**, with a
    point-bucket equality case (`smart_ata_attribute.go`). AFR cutoffs:
    critical fails at 10%, non-critical fails at 20% / warns at 10%.
15. **Manufacturer FAILING_NOW short-circuits before Scrutiny thresholds.**
16. **Only non-transient, non-ignored FailedScrutiny attributes roll up to
    device status. Warnings never set a device bit** — a warn-only device is
    DeviceStatusPassed, and the notification warn-level guards depend on it.
17. **Consumer-profile threshold overrides are evaluation-time only** (local
    copy, never persisted). The profile match keys off `model_family`/
    `model_name`, which are rewritten on every submission from collector
    data.
18. **Delta-suppression ordering:** the previous submission is read
    (`GetLatestSmartSubmission`, offset 0) before the new point is written;
    post-write callers use offset 1. Reordering breaks CRC-counter
    suppression and power-on-hours rollover detection.
19. **Go constants are hand-mirrored in the frontend** at
    `detail.component.ts`, `device-status.pipe.ts`, and `app.config.ts`.
    A change to `constants.go` is a four-file change.

## Notifications

20. **`ShouldNotify` fast path requires all three guards** (filter=All,
    repeat_notifications on, level != Warn). Warn level must fall through to
    attribute checks because no DeviceStatus warn bit exists.
21. **Collector-error dedup rolls back its key on send failure**
    (`notify/gate.go`); removing the rollback permanently suppresses that
    error after one transient send failure.
22. **Dedup, rate-limit, quiet-queue, and missed-ping state is in-memory
    only.** One re-notification burst after restart is expected; do not
    "fix" it by persisting without designing for it.
23. **Bool settings with true defaults are migration-seeded, never
    `ApplyDefaults`.** See ADR 0006.

## MQTT / integrations

24. **`unique_id` format, safeID normalization, entity keys, state-topic
    shape, and StatePayload JSON keys are frozen wire format.** See ADR 0004.
25. **Legacy WWN-topic cleanup on every sync must stay** until removed by a
    deliberate migration decision.
26. **`RiskCategory` strings** (`healthy`/`monitor`/`plan_replacement`/
    `replace_soon`) are compared by ordinal rank and stored in summaries;
    unknown strings fail closed (no notification). Replacement-risk weights
    sum to 100 per protocol.

## Collector

27. **The two smartctl exit-code masks are deliberately different:** `0xBF`
    tolerance in detection (`detect.go`), `0x03` fatal-mask in collection
    (`metrics.go`). Server side rejects uploads only on bits 0-1.
28. **`--device <type>` is passed only for non-scsi/ata types, in three
    places that must stay identical** (`detect.go` info, `metrics.go` xall,
    `metrics.go` FARM).
29. **Config args must contain `--json` and must not contain `--device`**
    (validated in collector `config.go`); the device type is appended
    programmatically.
30. **Device paths keep their case; macOS IOService paths bypass prefixing
    entirely.** Only WWN is case-normalized.
31. **`SmartCtlInfo` field copy preserves existing DeviceType and
    Manufacturer** (only-if-empty semantics) to honor user overrides.
32. **The collector unmarshals smartctl JSON into server-side structs**
    (`webapp/backend/pkg/models/collector`). Editing those JSON tags breaks
    the collector with no compile error.

## Frontend

33. **Settings hydration: bundled defaults first, `/api/settings` merged
    over them, empty string/null treated as missing.**
    `ScrutinyConfigService._mergeWithDefaults` is custom on purpose; writes
    use lodash merge on purpose. `server_version` and
    `collector_trigger_enabled` are capability flags re-grafted after every
    save, not persisted settings.
34. **The mobile/desktop divide is the `lt-md` alias = max-width 959px**,
    single-sourced in `tailwind/config.js` and exported into the Treo
    variables; the layout component forces the whole shell to `mobile`.
35. **Dark mode requires `--mat-*` token overrides under `.treo-theme-dark`
    in `styles/styles.scss`** for every overlay-rendered Material component;
    the legacy pre-MDC theme provides none.
36. **The temperature pipe contract:** `formatTemperature` formats only and
    never converts; conversion happens in `transform`/callers. Guards must
    preserve 0 (use `== null || !isFinite`, never `!temp`). This
    double-conversion bug shipped twice.
37. **The `/web` serve path** is coupled to `getBasePath()` in
    `app.routing.ts` and every API URL.
38. **Nav visibility lives in two non-DRY lists** (desktop
    `material.component.html`, mobile `mobile-tab-bar.component.ts`) plus a
    hardcoded detail-route prefix list; `show_*` flags read `!== false` so
    missing config means visible.

## Build / release

39. **`const VERSION = "x.y.z"` textual shape in `version.go` is parsed by
    three independent tools** (release sed, sonar grep, packagr). Never
    hand-edit or reformat. See ADR 0005.
40. **All shipped builds are static CGO-free**; `vendor/` is committed and
    regenerated by `go mod vendor` only. See ADR 0007.
41. **The 7-binary set is listed in four places** (Makefile `binary-all`,
    omnibus Dockerfile COPY, `release.yaml` and `ci.yaml` artifact globs)
    plus `rootfs/` s6/cron wiring.
42. **InfluxDB test-container credentials are duplicated literals** in
    `ci.yaml` and `sonarqube.yaml` and must match the backend test fixtures.
43. **Omnibus s6 startup is health-gated wait loops** on binary names and
    ports (`rootfs/etc/services.d/*/run`); renaming a binary or moving a
    port deadlocks container init silently.
