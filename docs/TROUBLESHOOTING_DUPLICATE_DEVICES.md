# Troubleshooting Duplicate Devices

## Why duplicates happen

Scrutiny uses a deterministic `device_id` generated from the device model, serial number, and WWN. If one of those values changes, Scrutiny can treat the same physical drive as a new device and create a duplicate record.

Common causes:

- failing hardware that reports inconsistent identity data
- controller or enclosure changes that affect the reported WWN
- `smartmontools` upgrades that change how device identity is exposed

## Migrating from AnalogJ/scrutiny 0.9.x

AnalogJ/scrutiny 0.9.x stored SMART history under an incompatible
`scrutiny_uuid` InfluxDB tag and cleared serial-based fallback WWNs. Direct
migration to affected Staros releases could hide existing history and create a
second row when an NVMe or SCSI collector ran.

Migration `m20260803000000` repairs this state automatically when the SQLite
`migrations` table contains `m20260216155600`. It restores current WWNs, merges
an unambiguous duplicate while preserving user metadata, and rewrites
`smart`/`temp` history in the daily, weekly, monthly, and yearly buckets.

0.9.x also forked a single drive into two rows of its own. It keyed devices by a
UUID over `model + serial + WWN` and blanked the model name on the row that
stopped updating, so the drive ends up as one row with a blank model name and
one carrying it, sharing a WWN and a serial. The migration merges that pair,
keeping the row that reports a model name and carrying the older `created_at`,
any user metadata, and the self-test history onto it.

Only that exact shape is merged, because the dropped row is deleted outright
and nothing restores it. These are left alone:

- rows that each report a model name. Two of those are two drives as far as the
  migration can tell, and an enclosure that fabricates a WWN can fabricate a
  serial too.
- rows where neither reports a model name, which leaves nothing to say which row
  is the live one.
- rows that share a WWN but report different serials. Drives behind one
  controller, or reporting a vendor-default WWN such as `0x0000000000000000`,
  legitimately share it (#851).
- rows that agree on WWN and serial but name different non-empty `host_id`
  values. That is two collectors reporting one drive.
- rows that agree on WWN and serial but report different non-zero capacities.
- a group of three or more rows, which a drive whose reported model string
  changed twice produces. Nothing says which model-bearing row the blank one
  belongs to.

Each refusal is logged with the serial and the reason. Merge any of these on
request with the endpoint below.

0.9.x also cleared serial-based fallback WWNs, and the migration restores them.
When one cleared row's serial matches two rows that already hold a WWN, nothing
says which it belongs to, so that row is left exactly as it was and the reason is
logged. The migration records as applied and does not run again, and nothing
outside it reads the 0.9.x `scrutiny_uuid` tag, so the endpoint below collapses
the duplicate row but does not recover that row's pre-import history. The log
line names the `scrutiny_uuid` holding it, which is what a manual recovery needs.

A drive hit by both 0.9.x bugs has its WWN cleared on both rows and its model
name blanked on one of them. The row keeping its model name survives, for the
same reason: identity reconciliation at registration requires equal model names,
so a blank-model survivor would leave the next collector run creating a fresh
duplicate instead of reconciling.

History rewritten out of 0.9.x is tagged with both `device_id` and `device_wwn`.
A WWN alone does not identify a device, and history queries fall back to the
`device_wwn` tag only while a single device holds that WWN, so WWN-only tags
would be unreadable for every holder of a shared WWN.

Do not change either legacy UUID namespace or its `model + serial + WWN`
formula. Existing 0.9.x history cannot be matched after either value changes.

`idx_devices_wwn` is a plain index, never unique. `device_id` is the device
identity, and a 0.9.x import can present duplicate WWNs before the repair
migration runs, so a unique index aborts the migration chain (#314, #851,
#1056).

## How to find the device IDs

Use the summary API and inspect the `device_id` values:

```bash
curl http://localhost:8080/api/summary | jq '.data | to_entries[] | {device_id: .key, model: .value.device.model_name, serial: .value.device.serial_number, wwn: .value.device.wwn}'
```

If your Scrutiny instance uses auth or a custom base path, include the same token or path you use for other API calls.

## Merge a duplicate into the correct device

Call the merge endpoint with the duplicate device as the route parameter and the device to keep in the JSON body:

```bash
curl -X POST \
  -H 'Content-Type: application/json' \
  -d '{"new_device_id":"DESTINATION_DEVICE_ID"}' \
  http://localhost:8080/api/device/SOURCE_DEVICE_ID/merge_into
```

What the endpoint does:

- copies the source device's InfluxDB history onto the destination device ID and WWN
- preserves the older `created_at` timestamp on the destination record
- deletes the source device row from SQLite after the merge completes

## Before you run it

- Verify the destination device is the record you want to keep.
- Prefer taking a database backup first if this is production data.
- Do not use the same source and destination `device_id`.

## After you run it

Check the destination device details and summary views again. The retained device should include the merged history, and the duplicate source device should no longer appear.
