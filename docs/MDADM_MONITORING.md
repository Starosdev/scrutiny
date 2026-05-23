# MDADM Monitoring

Scrutiny supports Linux software RAID arrays managed by `mdadm`.

This guide covers:

- omnibus deployments where the collector runs inside the main Scrutiny container
- hub/spoke deployments where `collector-mdadm` runs as its own container
- the required mounts, capabilities, and API wiring
- a concrete troubleshooting flow for partial registration and missing metrics

## Requirements

The MDADM collector expects all of the following:

- a standard Linux host that exposes `mdadm` arrays through `/proc/mdstat`
- access to the array block devices under `/dev`
- permission to run `mdadm --detail /dev/mdX`
- an API endpoint that resolves to the Scrutiny web server from the collector's network namespace

Platforms such as Unraid are useful for deployment-path smoke tests, but they are not reliable end-to-end validation targets because their `/proc/mdstat` output does not follow the standard Linux `mdadm` format that Scrutiny parses.

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

2. Run the collector manually with debug logging:

```bash
docker exec -it scrutiny /opt/scrutiny/bin/scrutiny-collector-mdadm run --debug
```

3. Confirm the arrays are registered:

```bash
curl -s http://localhost:8080/api/mdadm/summary | jq .
```

4. Open the MDADM page in the UI and verify the arrays show a state and size, not just static metadata.

## Troubleshooting

### `No MDADM arrays found`

Check:

- `/host/proc/mdstat` is mounted and readable
- the host is a standard Linux `mdadm` system, not Unraid
- the host file contains lines like `md0 : active raid1 ...`

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

## Related Docs

- [INSTALL_UNRAID.md](./INSTALL_UNRAID.md)
- [DEPLOYMENTS.md](./DEPLOYMENTS.md)
- [TESTING.md](../TESTING.md)
