# MDADM Monitoring

Scrutiny supports Linux software RAID arrays managed by `mdadm`, and Unraid parity arrays.

MDADM arrays appear on the dedicated MDADM page. The SMART dashboard contains SMART device cards only, so MDADM arrays do not affect SMART host search or host-count pagination.

This guide covers:

- omnibus deployments where the collector runs inside the main Scrutiny container
- hub/spoke deployments where `collector-mdadm` runs as its own container
- the required mounts, capabilities, and API wiring
- a concrete troubleshooting flow for partial registration and missing metrics

## Requirements

On a standard Linux host, the MDADM collector expects all of the following:

- a host that exposes `mdadm` arrays through `/proc/mdstat`
- access to the array block devices under `/dev`
- permission to run `mdadm --detail /dev/mdX`
- an API endpoint that resolves to the Scrutiny web server from the collector's network namespace

On Unraid, only `/proc/mdstat` and the API endpoint are required. Unraid does not
use Linux MD RAID: it writes `/proc/mdstat` as `key=value` metadata and ships no
`mdadm` binary, so the collector detects that format separately and runs no
commands. See [INSTALL_UNRAID.md](./INSTALL_UNRAID.md) for what Unraid reports
and how it differs.

## Omnibus Deployment

In omnibus mode the MDADM collector runs inside the main `scrutiny` container.

Use these mounts and capabilities:

```yaml
services:
  scrutiny:
    image: ghcr.io/starosdev/scrutiny:latest-omnibus
    cap_add:
      - SYS_RAWIO
      - SYS_ADMIN
    volumes:
      - /dev:/dev:ro
      - /proc/mdstat:/host/proc/mdstat:ro
      - ./config:/opt/scrutiny/config
      - ./influxdb:/opt/scrutiny/influxdb
    environment:
      COLLECTOR_MDADM_RUN_STARTUP: "true"
      COLLECTOR_MDADM_CRON_SCHEDULE: "*/15 * * * *"
```

Notes:

- `localhost:8080` is valid inside the omnibus container because the API and collector share the same container.
- Docker does not let you bind the host file directly onto `/proc/mdstat`, so Scrutiny reads `/host/proc/mdstat` first.

To run the collector manually inside the omnibus container:

```bash
docker exec -it scrutiny /opt/scrutiny/bin/scrutiny-collector-mdadm run --debug
```

## Hub And Spoke Deployment

In hub/spoke mode the MDADM collector runs as a separate container.

```yaml
services:
  web:
    image: ghcr.io/starosdev/scrutiny:latest-web

  collector-mdadm:
    image: ghcr.io/starosdev/scrutiny:latest-collector-mdadm
    restart: unless-stopped
    cap_add:
      - SYS_ADMIN
    volumes:
      - /dev:/dev
      - /proc/mdstat:/host/proc/mdstat:ro
    environment:
      COLLECTOR_MDADM_API_ENDPOINT: http://web:8080
      COLLECTOR_MDADM_RUN_STARTUP: "true"
```

Endpoint rules:

- use `http://web:8080` when the collector and web containers share a compose network
- use `http://127.0.0.1:8080` together with `--network host` when you are running the collector on Zeus against a web server bound on the Zeus host
- do not use `http://localhost:8080` for a separate collector container unless the API is actually inside that same container

## Validation

1. Confirm the collector can see host array metadata:

```bash
docker exec -it scrutiny cat /host/proc/mdstat
docker exec -it scrutiny mdadm --detail /dev/md0
```

For a standalone collector container, replace `scrutiny` with `collector-mdadm`.

On Unraid, skip the `mdadm --detail` command. It is not installed and is not
used; `cat /host/proc/mdstat` is the whole check.

2. Run the collector manually with debug logging:

```bash
docker exec -it scrutiny /opt/scrutiny/bin/scrutiny-collector-mdadm run --debug
```

3. Confirm the arrays are registered:

```bash
curl -s http://localhost:8080/api/mdadm/summary | jq .
```

4. Open the dedicated MDADM page in the UI and verify the arrays show a state and size, not just static metadata.

Upgrade note:

- builds that persist MDADM `host_id` on array re-registration do not retroactively backfill older rows
- if arrays were first registered before that fix, run one fresh MDADM collection on each affected host
- confirm `GET /api/mdadm/summary` now includes `host_id` for those arrays before expecting grouped host headings in the UI

## Troubleshooting

### `No MDADM arrays found`

Check:

- `/host/proc/mdstat` is mounted and readable
- on a Linux host, the file contains lines like `md0 : active raid1 ...`
- on Unraid, the file contains lines like `mdState=STARTED` and the collector logs `Detected Unraid-format /proc/mdstat`

### `failed to run mdadm --detail /dev/mdX`

Check:

- the relevant `/dev/mdX` devices are visible inside the container
- the container has `SYS_ADMIN`
- the container image includes `mdadm`

### Arrays appear in the UI but show no metrics

This usually means registration partially succeeded for some arrays while another array failed registration.

Run the collector with `--debug` and then check the Scrutiny web/API container logs for the specific registration error:

```bash
docker logs <web-or-omnibus-container> | grep -i mdadm
```

Recent builds now continue uploading metrics for the arrays that were registered successfully and log per-array registration failures instead of collapsing the entire run into one generic error.

### One array is missing

Check the backend logs for a per-array registration error. Common causes include:

- the array did not report a UUID
- the array UUID collided with another discovered array
- the database rejected one array record while accepting the others

### Arrays are present but not grouped under the correct host

Check:

- `GET /api/mdadm/summary` includes a non-empty `host_id` for each affected array
- the affected host has completed at least one fresh MDADM collector registration after upgrading
- the collector `host.id` value is stable and not changing between runs

If older rows were created before `host_id` was persisted on re-registration, force one new collection on each affected host to backfill the stored `host_id`.

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/mdadm/summary` | Get summary of all MDADM arrays |
| GET | `/api/mdadm/array/:uuid/details` | Get detailed array info and metrics history |
| POST | `/api/mdadm/arrays/register` | Register arrays (used by collector) |
| POST | `/api/mdadm/array/:uuid/metrics` | Upload metrics (used by collector) |
| POST | `/api/mdadm/array/:uuid/archive` | Archive an array |
| POST | `/api/mdadm/array/:uuid/unarchive` | Unarchive an array |
| POST | `/api/mdadm/array/:uuid/mute` | Mute notifications for an array |
| POST | `/api/mdadm/array/:uuid/unmute` | Unmute notifications |
| POST | `/api/mdadm/array/:uuid/label` | Set a custom label for an array |
| DELETE | `/api/mdadm/array/:uuid` | Delete an array and its metrics history |

The identifier in the path is the array's UUID as mdadm reports it, the
`synthetic:<hash>` identifier Scrutiny assigns to an array that has none, or
the `unraid:<hash>` identifier an Unraid array carries. It is
not an RFC 4122 UUID, so these routes take any identifier of letters, digits,
`:`, `.`, `_` and `-`, up to 256 characters. Escape it before putting it in a
URL: an mdadm UUID contains colons.

Deleting an array removes its metadata and its metrics series from InfluxDB.
An array the collector still finds is registered again on its next run, so
delete is for an array that is gone: a retired host, or one left behind by
testing.

The arrays page offers archive, mute and delete per array. Archiving removes
the array from that page, because `/api/mdadm/summary` returns unarchived
arrays only; unarchiving is through the API until the page offers a way to
list archived arrays.

## Related Docs

- [INSTALL_UNRAID.md](./INSTALL_UNRAID.md)
- [DEPLOYMENTS.md](./DEPLOYMENTS.md)
- [TESTING.md](../TESTING.md)
