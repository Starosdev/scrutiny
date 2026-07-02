# 0002. Device identity: UUIDv5 device_id layered over WWN

Status: accepted
Date: 2026-07-02 (documents the identity migration, see #270/#311, commit 850794e4)

## Context

WWN was the original primary key, but WWN is unreliable: smartctl only emits
it (as NAA/OUI/ID parts) for ATA drives; NVMe/SCSI often omit it; USB bridges
report none; some drives share one. Empty and duplicate WWNs corrupted the old
keyspace.

## Decision

- `device_id` is a deterministic UUIDv5 over
  `lower(trim(model)):lower(trim(serial)):lower(trim(wwn))` under the fixed
  namespace in `webapp/backend/pkg/deviceid/deviceid.go`. It is the SQLite
  primary key and the identity used by MQTT and Prometheus.
- When model, serial, and WWN are all empty, the fallback hashes
  `device_name:<name>:host_id:<host>` instead, so metadata-less devices on
  one host do not collapse into a single UUID.
- WWN remains: lowercased everywhere, generated collector-side from NAA parts,
  falling back to ghw block data, then to the serial number. It stays the
  InfluxDB tag key and the legacy API lookup (`resolve.go` accepts device_id
  or WWN; WWN path is deprecated but load-bearing).
- The collector may omit device_id; the server computes it at registration and
  echoes it back. Collectors fall back to WWN URLs against older servers.

## Consequences

- The namespace UUID, the input formula (order, separator, casing, trimming),
  and the fallback shape are frozen. Any change re-identifies the entire
  fleet on the next collector run: every user gets duplicate devices and
  orphaned history. This is the single most dangerous edit in the repo.
- `GenerateWithFallback` is called with identical argument order in at least
  three server locations (registration, MQTT discovery, migration backfill);
  they must not drift.
- WWN's fallback-to-serial means "WWN" in this codebase does not always mean
  an actual World Wide Name. Do not add WWN-format validation.
- A device that first registers metadata-less (fallback keyspace) and later
  reports a WWN gets a different device_id; `reconcileLegacyDeviceIdentity`
  merges the rows only when exactly one unambiguous candidate matches.
- The performance collector still keys purely on WWN and silently skips
  no-WWN devices; known inconsistency, tolerated.
