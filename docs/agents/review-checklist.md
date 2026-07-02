# Architect Review Checklist

Questions a principal reviewer asks about a Scrutiny diff, beyond "does it
work". Run the sections whose files appear in the diff. Sources:
[invariants.md](invariants.md), [danger-zones.md](danger-zones.md),
[../adr/](../adr/).

## Any diff

- Does the change contradict an ADR in `docs/adr/`? If yes, the PR must say
  so and argue it, not slide past it.
- Does it touch a file listed in danger-zones.md? If yes, was the listed
  failure mode explicitly considered?
- Cross-language mirrors: does the diff change `pkg/constants.go`, any Go
  model with a "maps to" TS twin, or any TS enum copied from Go? Then the
  counterpart files must be in the same diff.
- Is anything here a wire contract (API payload, InfluxDB field name, MQTT
  topic/key, settings key)? Wire contracts get additive changes only,
  or an explicit migration story.

## Identity and storage (`deviceid/`, `detect/`, `database/`)

- Could this change alter the device_id or WWN computed for any existing
  device? If yes: stop, that is a fleet-wide re-identification.
- Any new InfluxDB field/tag: is it in `Flatten()`, the readers, the summary
  query if needed, and merge `tagKeys` if it is a tag?
- Any new Device column: added to the `RegisterDevice` allow-list (if it
  should update on re-registration)? Absent from replayed table-rebuild
  migrations, or coherently added?
- Any GORM update that might write a zero value: does it use `Update`/map
  instead of `Updates(struct)`?
- New settings field: struct tag + seeding migration + (string/int only)
  ApplyDefaults? Bool-true default seeded by migration?
- Tests touching migrations: `ResetMigrationGuardForTests()` called?

## Health logic (`thresholds/`, `measurements/`)

- Does the change alter any verdict for existing stored data? That is a
  user-facing behavior change and needs to be called out in the PR, not
  discovered by users.
- Threshold-table or profile-JSON edits: data source cited? Profile lint run?
- New attribute in replacement risk: weights rebalanced to sum 100?
- Any new status handling: correct bitmask (attribute vs device), `== Passed`
  for zero checks, warn-vs-fail rollup semantics preserved?

## Notifications (`notify/`, `missed_ping_monitor`)

- Can this path fire more than once for the same condition? Which dedup
  mechanism covers it, and does that mechanism survive its failure modes
  (send failure rollback, restart burst)?
- Warn-level path still reaches attribute-level checks?
- New notification type: respects the gate (rate limit, quiet hours) or
  documents why it bypasses (heartbeats bypass deliberately)?

## Collector (`collector/`)

- smartctl argument changes: applied consistently across info/xall/FARM
  call sites? Exit-code masks untouched (they differ on purpose)?
- Would this change what identifier is sent to the server for any device?
- Back-compat: does a new collector still work against an old server
  (WWN fallback), and does the server still accept an old collector?
- New config key: flag + env + yaml precedence wired, validated in
  `ValidateConfig` if constrained?

## Frontend (`webapp/frontend/`)

- New overlay component (dialog/select/menu/panel): dark tokens added under
  `.treo-theme-dark`?
- New route: desktop nav + mobile tab (or explicit decision to omit) +
  detail-prefix list if it is a detail page?
- Settings-shaped data: flows through `_mergeWithDefaults` semantics; no
  lodash merge on the hydration path; capability flags preserved on save?
- Temperature or other unit-bearing values: conversion happens exactly once,
  zero preserved?
- Status display: bitwise semantics identical to Go, threshold masking and
  `has_forced_failure` respected?

## Build / CI / release

- Version, Go toolchain, or binary-set changes: all the duplicated locations
  updated (see invariants 39-43)?
- Workflow edits: do docker-build and the channel deploy workflows still
  agree on who produces which tag?
- Anything moved in `rootfs/` or Dockerfiles: s6 wait loops, chmod lists,
  cron templates still consistent?
- Dependency changes: `go mod vendor` run; frontend installs still work with
  `--legacy-peer-deps`?

## Release-notes hygiene

- User-visible behavior changes (verdicts, notifications, defaults) named
  plainly in the PR description, because users read release notes generated
  from these.
