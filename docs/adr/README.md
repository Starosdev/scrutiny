# Architecture Decision Records

Records of decisions that shape this codebase. Each ADR explains a choice that
looks questionable in isolation but is deliberate. Before "fixing" something
that seems odd, check whether an ADR covers it. If a proposed change
contradicts an ADR, say so explicitly in the PR/issue instead of silently
overriding it.

## Index

| ADR | Title |
| --- | --- |
| [0001](0001-dual-store-sqlite-influxdb.md) | Dual store: SQLite for identity/config, InfluxDB for time-series |
| [0002](0002-device-identity-uuidv5.md) | Device identity: UUIDv5 device_id layered over WWN |
| [0003](0003-status-bitmasks-and-observed-thresholds.md) | Health status as bitmasks; verdicts from Backblaze observed thresholds |
| [0004](0004-mqtt-entity-identity.md) | MQTT/Home Assistant entity identity is frozen |
| [0005](0005-manual-releases-and-image-tags.md) | Manual-only releases; omnibus owns the bare image tags |
| [0006](0006-settings-defaults-via-migrations.md) | Bool settings defaults seeded by migration, not code |
| [0007](0007-static-builds-and-vendoring.md) | Static CGO-free builds and committed vendor/ |

## Adding an ADR

Copy the format below. Number sequentially. Keep it under a page. An ADR is
worth writing when the decision is (a) expensive to reverse, (b) likely to be
"corrected" by someone who lacks the context, or (c) a repeated discussion.

```markdown
# NNNN. Title

Status: accepted | superseded by NNNN
Date: YYYY-MM-DD

## Context
What forces were at play.

## Decision
What we do, stated as a rule.

## Consequences
What this costs us and what it buys. Include the traps it creates.
```
