# Danger Zones

Files and functions where a competent, well-intentioned edit causes data
loss, silent corruption, mass mis-verdicts, or fleet-wide duplicate devices.
Ordered roughly by blast radius. Before editing anything listed here, read
the matching entry in [invariants.md](invariants.md) and the linked ADR.

## Fleet-wide identity breakage

| Location | Naive edit | Failure mode |
| --- | --- | --- |
| `webapp/backend/pkg/deviceid/deviceid.go` | Touch namespace, formula, casing, separator, fallback shape | Every device re-IDs on next collector run; duplicate devices and orphaned history for all users (ADR 0002) |
| `collector/pkg/detect/devices_*.go` `wwnFallback` | Drop the serial fallback or the ghw `"unknown"` guard | Changed WWN changes device_id hash input; duplicates + orphaned InfluxDB history for NVMe/SCSI/USB drives |
| `collector/pkg/collector/metrics.go` upload URL builders | Remove a "redundant" `strings.ToLower` | Mixed-case ID misses `ResolveDevice`; duplicate device row or 404 metric drop |
| `collector/pkg/detect/detect.go` `FilterRedundantDevices` / `TransformDetectedDevices` | Simplify megaraid dedup or the case-fold delete flags | RAID-backed drives silently dropped or duplicated; the exact bugs of commits 64695116 and e5e3dcfc return |

## Silent health-verdict flips

| Location | Naive edit | Failure mode |
| --- | --- | --- |
| `webapp/backend/pkg/models/measurements/smart_ata_attribute.go` | Bucket containment boundaries or the 10%/20% AFR cutoffs | Existing drives reclassify healthy/failed for all users with no data change |
| `webapp/backend/pkg/thresholds/` metadata + `consumer_drive_profiles.json` | Casual AFR/bucket edits | Tables ARE the verdict; `init()` panics only on structural errors, not wrong rates. Run `consumer_drive_profiles_lint.go` |
| `measurements/smart.go` transient/ignored propagation conditions | Flip the condition | Ignored attributes fail whole devices, or real failures suppressed |
| `pkg/constants.go` | Change bit values | UI mislabels health (three hand-copied frontend mirrors), threshold masking breaks |
| `scrutiny_repository_device_smart_attributes.go` | Reorder the pre-write read, or swap offset-0/offset-1 helpers | CRC delta suppression and power-on-hours rollover detection break |

## Data loss / corruption

| Location | Naive edit | Failure mode |
| --- | --- | --- |
| `scrutiny_repository_device.go` `DeleteDevice`, `scrutiny_repository_device_merge.go` | Assume WWN unique | InfluxDB delete predicate is WWN-wide; devices sharing a WWN lose each other's history; merge of shared-WWN devices deletes just-copied data |
| `scrutiny_repository_device_merge.go` `measurementFields` | Add an InfluxDB tag without extending `tagKeys` | Tag rewritten as field during merge; schema corruption |
| GORM `Updates(struct)` anywhere | Use it to clear a zero-value field | `device_status = 0` and `false` are silently skipped; failed status never clears. Use `Update(col, val)` or a map |
| `scrutiny_repository_settings.go` | Bypass `settingsMu`, or expect SaveSettings to create rows | Shared Viper races across the several repository instances; new keys never persist (migrations create rows) |
| `models/settings.go` `ApplyDefaults` | Add a `defaultBool` for a true-default setting | Force-re-enables features users deliberately disabled (ADR 0006) |

## Integration orphans and spam

| Location | Naive edit | Failure mode |
| --- | --- | --- |
| `pkg/mqtt/discovery.go` | "Cosmetic" rename of entity key, unique_id format, safeID, or StatePayload key | Every HA entity orphaned + duplicated for every MQTT user; removal logic cannot clean the old shape (ADR 0004) |
| `pkg/notify/notify.go` `ShouldNotify` fast path | Remove a guard | Warn-only devices never notify, or every submission spams |
| `pkg/notify/gate.go` | Remove the dedup-key rollback on send failure | One transient failure permanently mutes that collector error |
| `web/missed_ping_monitor.go` | Drop `clearNotificationState` on healthy, or mishandle cooldown<=0 fallback | Missed-ping notification spam |

## Frontend traps

| Location | Naive edit | Failure mode |
| --- | --- | --- |
| `shared/temperature.pipe.ts` + `dashboard.component.ts` tooltip | Make `formatTemperature` convert, or use `transform` in the tooltip | Fahrenheit double-conversion; shipped twice (50561f34, 4c343bcf) |
| Temperature guards | `!temp` instead of `== null \|\| !isFinite` | Valid 0 degree readings render as `--` (commit 5809ebf3) |
| `core/config/scrutiny-config.service.ts` | Replace `_mergeWithDefaults` with lodash merge | Empty DB strings clobber defaults; blank UI (commit 8d205795) |
| `styles/styles.scss` dark section | Style a new overlay component only for light mode | Unreadable dark-on-dark dialogs/selects (the #165 class of bug) |
| `styles/styles.scss` SMART-table contrast rule | Add a colored badge class without extending the `:not()` exclusion list | Badge text washes out white |
| New routes | Skip one of: desktop nav, mobile tab list, detail-prefix list | Route unreachable on mobile, or tab bar shown on detail pages |

## Build / release traps

| Location | Naive edit | Failure mode |
| --- | --- | --- |
| `webapp/backend/pkg/version/version.go` | Hand-edit or reformat | Release sed no-ops; ships old version (ADR 0005) |
| `docker-build.yaml` | Remove the `v*.*.*` tag trigger | Released versions never get images (path filter excludes version.go) |
| CI job env | Drop `STATIC: true` | CGO re-enabled; binaries fail slim runtime and smoke checks |
| `docker/Dockerfile` s6/arch section | Add an omnibus arch without extending the S6 arch map | Empty S6_ARCH, download 404, `/init` broken |
| `Makefile` / Dockerfile COPY / workflow globs | Add a collector binary to fewer than all four lists | Binary builds but never ships, or ships but missing from omnibus |
| `rootfs/etc/services.d/*` | Rename a binary or move a port | Health-gated wait loops deadlock container start silently |

## Historical scars (why some code looks paranoid)

- InfluxDB queries are string-built, not parameterized: parameterized queries
  broke InfluxDB OSS and were reverted (#157). Do not "modernize" them.
- Repeat-notification detection compares against the previous raw submission,
  not the daily aggregate (#129).
- `release-frontend.yaml` exists because a release once shipped an empty
  frontend tarball (#64); keep its artifact-path assumptions in sync with
  the Angular output path.
- WAL journal mode plus busy_timeout defaults exist for Docker
  `cap_drop: ALL` deployments (#25, #341); the readonly-DB error message
  advertises the `journal_mode: DELETE` escape hatch.
