# 0003. Health status as bitmasks; verdicts from Backblaze observed thresholds

Status: accepted
Date: 2026-07-02 (documents inherited design plus the consumer-drive-profile extension)

## Context

A drive can fail its manufacturer SMART threshold, exceed Scrutiny's
real-world failure-rate thresholds, carry warnings, and report corrupted
values all at once. Manufacturer thresholds alone are binary and lagging;
Backblaze drive-stats give graduated, per-raw-value annualized failure rates
(AFR) that flag drives earlier.

## Decision

- `AttributeStatus` (`pkg/constants.go`) is a uint8 bitmask: Passed=0,
  FailedSmart=1, WarningScrutiny=2, FailedScrutiny=4, InvalidValue=8.
  `DeviceStatus` is a separate, smaller bitmask: Passed=0, FailedSmart=1,
  FailedScrutiny=2. Independent evaluators OR in their own bit; the user's
  `status_threshold` setting masks which sources count in the UI.
- Evaluation order for ATA attributes: manufacturer FAILING_NOW wins and
  short-circuits; IN_THE_PAST warns; otherwise the observed-threshold bucket
  lookup derives an AFR and applies it (critical attributes fail at >= 10%,
  non-critical fail at >= 20% / warn at >= 10%). Buckets are Low-exclusive,
  High-inclusive, with a point-bucket special case.
- Consumer drive profiles overlay model-family-specific observed thresholds
  at evaluation time only (local copy, never persisted), matched
  strongest-first (family, alias, vendor-normalized, regex) with a minimum
  20-sample confidence gate. Global opt-out plus per-family denylist.
- Only `FailedScrutiny` on a non-transient, non-ignored attribute rolls up to
  device level. Warnings never set a device bit: a warn-only device is
  `DeviceStatusPassed`.

## Consequences

- The bit values differ across the two masks (`FailedScrutiny` is 4 for
  attributes, 2 for devices). Never copy a mask across the boundary.
- `Has(x, 0)` is always false; passed-ness is `== Passed`, not a bit test.
- The frontend hand-copies these constants in three places
  (`detail.component.ts`, `device-status.pipe.ts`, `app.config.ts`). No
  codegen exists; changing `constants.go` requires touching all three.
- Because warn-only devices read as Passed, the notification path has
  explicit warn-level guards that must fall through to attribute-level
  checks. Removing them silently kills or floods warn notifications.
- The observed-threshold tables and `consumer_drive_profiles.json` ARE the
  verdict. Editing an AFR or bucket boundary reclassifies drives for every
  user with no code change visible in review. Such edits need data
  justification and the profile lint (`consumer_drive_profiles_lint.go`).
- Because the same stored SMART data re-evaluates differently when profiles
  are toggled, verdict changes after a settings flip are expected behavior,
  not a bug.
