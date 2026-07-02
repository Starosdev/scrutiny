# 0006. Bool settings defaults seeded by migration, not code

Status: accepted
Date: 2026-07-02

## Context

Settings load from SQLite rows into a Go struct. For a bool setting whose
intended default is true (for example `repeat_notifications`,
`notify_on_missed_ping`, `notify_on_collector_error`), Go's zero value
`false` is indistinguishable from "the user deliberately turned this off".
A code-side default would silently re-enable features users disabled.

## Decision

- Every settings field needs: the struct field with its `mapstructure` tag in
  `models/settings.go`, plus a DB migration inserting the `SettingEntry` row
  with the default value.
- `ApplyDefaults` handles only string and int fields, where `""`/`0` is an
  unambiguous "unset" sentinel. Bool fields, especially true-by-default ones,
  are never defaulted in `ApplyDefaults`.
- `SaveSettings` updates existing rows only; it does not create keys. Row
  creation is the migration's job.

## Consequences

- "Add a settings field" is a three-place change (struct, migration,
  optionally ApplyDefaults) and looks over-engineered until you know why.
- Adding a `defaultBool` entry to `ApplyDefaults` for a true-default bool is
  a user-hostile bug that passes all tests. The warning comment in
  `models/settings.go` around lines 82-93 is the canonical statement.
- Frontend mirrors this: `ScrutinyConfigService._mergeWithDefaults` is a
  custom deep-merge that treats `''`/null/undefined as missing so empty DB
  values cannot clobber bundled defaults. Do not replace it with lodash
  `merge` (that exact bug shipped once; see commit 8d205795).
