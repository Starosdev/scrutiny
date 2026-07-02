# 0004. MQTT/Home Assistant entity identity is frozen

Status: accepted
Date: 2026-07-02

## Context

Home Assistant keys entities permanently by `unique_id` and binds sensor
values to retained discovery configs referencing exact state-topic paths and
JSON keys. Anything Scrutiny publishes becomes part of users' HA databases,
automations, and dashboards.

## Decision

- Entity `unique_id` format is `scrutiny_<safeDeviceID>_<entityKey>` where
  `safeDeviceID` strips the `0x` prefix and dashes from device_id
  (`pkg/mqtt/discovery.go`). Entity keys are `temperature`, `status`,
  `power_on_hours`, `power_cycle_count`, `problem`.
- State topic is `scrutiny/device/<safeDeviceID>/state`; the `StatePayload`
  JSON keys (`status`, `problem`, `temperature`, `power_on_hours`,
  `power_cycle_count`, `last_updated`) are the value_template contract.
- Discovery configs are always published retained; removal publishes empty
  retained payloads. Legacy WWN-keyed topics are cleaned up on every sync
  because entities migrated from WWN to device_id keying.
- The `problem` binary sensor reflects raw `DeviceStatus != Passed`, not the
  user's status_threshold.

## Consequences

- Changing the unique_id format, safeID normalization, an entity key, or a
  StatePayload JSON key orphans every existing HA entity for every user and
  creates duplicates. `BuildRemoveMessages` can only clean the current topic
  shape plus the legacy WWN shape; it cannot clean a format you just changed
  away from. Treat all of these as frozen wire format.
- Adding a new entity is safe; renaming or removing one is a migration that
  needs explicit dual-cleanup logic like the WWN legacy path.
- Device display name derivation (Label, then ModelName/DeviceName) is safe
  to change; the `identifiers` field is not (it re-groups entities).
