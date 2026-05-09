# Troubleshooting Duplicate Devices

## Why duplicates happen

Scrutiny uses a deterministic `device_id` generated from the device model, serial number, and WWN. If one of those values changes, Scrutiny can treat the same physical drive as a new device and create a duplicate record.

Common causes:

- failing hardware that reports inconsistent identity data
- controller or enclosure changes that affect the reported WWN
- `smartmontools` upgrades that change how device identity is exposed

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
