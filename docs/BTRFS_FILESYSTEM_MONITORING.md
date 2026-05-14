# Btrfs Filesystem Monitoring

Scrutiny can monitor Btrfs filesystem health alongside S.M.A.R.T, generic filesystem capacity, ZFS, and workload metrics. This guide explains how to package and run the dedicated Btrfs collector.

## Features

- Filesystem registration by UUID, label, host, and mount point
- Device membership tracking with per-device persistent error counters
- Detailed usage metrics from `btrfs filesystem usage --raw`
- Scrub status collection for mounted filesystems
- Historical metrics with time-series storage
- Multiple host support for monitoring Btrfs filesystems across different servers

## Scope

The dedicated Btrfs collector complements, rather than replaces, the generic filesystem-capacity collector documented in [FILESYSTEM_CAPACITY.md](FILESYSTEM_CAPACITY.md).

- Use the generic filesystem collector for broad logical filesystem capacity reporting.
- Use the Btrfs collector for Btrfs-specific health, topology, device stats, and scrub status.

## Quick Start

### Omnibus Image

If you are using the omnibus image (`ghcr.io/starosdev/scrutiny:latest-omnibus`), add these environment variables and host mounts to enable Btrfs monitoring:

```yaml
version: '3.5'

services:
  scrutiny:
    image: ghcr.io/starosdev/scrutiny:latest-omnibus
    environment:
      COLLECTOR_BTRFS_CRON_SCHEDULE: "*/15 * * * *"
      COLLECTOR_BTRFS_RUN_STARTUP: "true"
    volumes:
      - /mnt:/mnt:ro
      - /var/lib/btrfs:/var/lib/btrfs:ro
    cap_add:
      - SYS_ADMIN
```

The omnibus image already includes the Btrfs collector binary and `btrfs-progs`.

### Hub/Spoke Deployment

For hub/spoke deployments, run the Btrfs collector container on each host with Btrfs filesystems and make sure the host mount points and Btrfs scrub state are visible inside the container:

```yaml
version: '2.4'

services:
  collector-btrfs:
    restart: unless-stopped
    image: 'ghcr.io/starosdev/scrutiny:latest-collector-btrfs'
    cap_add:
      - SYS_ADMIN
    volumes:
      - '/mnt:/mnt:ro'
      - '/var/lib/btrfs:/var/lib/btrfs:ro'
    environment:
      COLLECTOR_BTRFS_API_ENDPOINT: 'http://web:8080'
      COLLECTOR_BTRFS_HOST_ID: 'my-btrfs-server'
      COLLECTOR_BTRFS_RUN_STARTUP: 'true'
    depends_on:
      web:
        condition: service_healthy
```

Notes:

- `/mnt` is an example. Mount every host path that contains the Btrfs filesystems you want Scrutiny to inspect.
- `/var/lib/btrfs` is required if you want scrub status history inside the container.
- `SYS_ADMIN` is recommended because some `btrfs` commands require elevated privileges.

## Environment Variables

### Docker-Only Variables

These variables are handled by the container entrypoint script:

| Variable | Default | Description |
|----------|---------|-------------|
| `COLLECTOR_BTRFS_CRON_SCHEDULE` | (empty in omnibus, `*/15 * * * *` in dedicated image) | Cron schedule for Btrfs data collection |
| `COLLECTOR_BTRFS_RUN_STARTUP` | `false` | Set to `true` to run the Btrfs collector immediately when the container starts |
| `COLLECTOR_BTRFS_RUN_STARTUP_SLEEP` | `1` | Delay in seconds before startup collection |

### Collector Configuration Variables

These variables configure the collector binary itself:

| Variable | Default | Description |
|----------|---------|-------------|
| `COLLECTOR_BTRFS_API_ENDPOINT` or `COLLECTOR_API_ENDPOINT` | `http://localhost:8080` | URL of the Scrutiny web server API |
| `COLLECTOR_BTRFS_HOST_ID` or `COLLECTOR_HOST_ID` | (empty) | Identifier for this host, used to group filesystems in the dashboard |
| `COLLECTOR_BTRFS_LOG_FILE` or `COLLECTOR_LOG_FILE` | (empty) | Path to log file. Leave empty for stdout |
| `COLLECTOR_BTRFS_DEBUG` or `COLLECTOR_DEBUG` or `DEBUG` | `false` | Enable debug logging |
| `COLLECTOR_BTRFS_API_TOKEN` or `COLLECTOR_API_TOKEN` | (empty) | API token when Scrutiny authentication is enabled |

## Configuration File

The Btrfs collector can also be configured via a YAML file. The collector looks for configuration in this order:

1. `/opt/scrutiny/config/collector-btrfs.yaml`
2. `/opt/scrutiny/config/collector-btrfs.yml`
3. `/opt/scrutiny/config/collector.yaml` (fallback)
4. `/opt/scrutiny/config/collector.yml` (fallback)

Example configuration (`collector-btrfs.yaml`):

```yaml
version: 1

host:
  id: "my-btrfs-server"

api:
  endpoint: "http://localhost:8080"
  timeout: 60

log:
  level: INFO
  file: ""
```

## Manual Installation

If you are not using Docker, you can run the Btrfs collector binary directly:

1. Download the `scrutiny-collector-btrfs` binary from the [releases page](https://github.com/Starosdev/scrutiny/releases)
2. Ensure the `btrfs` command is available on your system (`btrfs-progs` on Linux)
3. Run the collector:

```bash
# Run once
scrutiny-collector-btrfs run --api-endpoint http://localhost:8080

# With debug logging
scrutiny-collector-btrfs run --api-endpoint http://localhost:8080 --debug

# With host identifier
scrutiny-collector-btrfs run --api-endpoint http://localhost:8080 --host-id my-server
```

4. Set up a cron job to run the collector periodically:

```cron
*/15 * * * * /path/to/scrutiny-collector-btrfs run --api-endpoint http://localhost:8080
```

## Verifying Btrfs Monitoring

After enabling Btrfs monitoring:

1. Wait for the collector to run, or trigger it manually.
2. Open the Scrutiny web UI.
3. Click on `Btrfs` in the navigation bar.
4. You should see your Btrfs filesystems listed with their current health and usage.

You can also verify via the API:

```bash
curl http://localhost:8080/api/btrfs/summary
curl http://localhost:8080/api/btrfs/filesystem/UUID/details
```

## Troubleshooting

### Btrfs Tab Is Empty

1. Make sure you set `COLLECTOR_BTRFS_CRON_SCHEDULE` in the omnibus image or started the dedicated collector container.
2. Confirm the host Btrfs mount points are bind-mounted into the container.
3. Confirm `btrfs` commands work inside the container:

```bash
docker exec scrutiny btrfs filesystem show
```

4. Enable debug logging with `COLLECTOR_BTRFS_DEBUG=true`.

### Scrub Status Missing

1. `btrfs scrub status` only works for mounted filesystems.
2. Bind-mount `/var/lib/btrfs` into the container if you want persisted scrub state.
3. Run the collector as a user with enough privileges to execute `btrfs` commands.

### Collector Errors

1. Verify the API endpoint is reachable.
2. Verify the container can see the host mount points the filesystem uses.
3. Verify `btrfs-progs` is installed when running outside Docker.

## API Endpoints

The Btrfs monitoring feature exposes the following API endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/btrfs/summary` | Get summary of all Btrfs filesystems |
| GET | `/api/btrfs/filesystem/:uuid/details` | Get detailed Btrfs filesystem info |
| POST | `/api/btrfs/filesystems/register` | Register filesystems (used by collector) |
| POST | `/api/btrfs/filesystem/:uuid/metrics` | Upload metrics (used by collector) |
| POST | `/api/btrfs/filesystem/:uuid/archive` | Archive a filesystem |
| POST | `/api/btrfs/filesystem/:uuid/unarchive` | Unarchive a filesystem |
| POST | `/api/btrfs/filesystem/:uuid/mute` | Mute notifications for a filesystem |
| POST | `/api/btrfs/filesystem/:uuid/unmute` | Unmute notifications |
| POST | `/api/btrfs/filesystem/:uuid/label` | Set a custom label for a filesystem |
| DELETE | `/api/btrfs/filesystem/:uuid` | Delete a filesystem and its data |
