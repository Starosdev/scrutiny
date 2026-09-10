# ZFS Pool Monitoring

Scrutiny can monitor ZFS pool health alongside individual drive S.M.A.R.T metrics. This guide explains how to enable and configure ZFS pool monitoring.

## Features

- Pool health status (ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL)
- Capacity metrics (size, allocated, free, fragmentation percentage)
- Usable capacity from the pool's root dataset, reported alongside raw vdev capacity
- Error tracking (read errors, write errors, checksum errors)
- Scrub operation monitoring (state, progress, errors, timing)
- Virtual device (vdev) hierarchy with per-vdev status and errors
- Historical metrics with time-series storage
- Multiple host support for monitoring pools across different servers
- Inventory presence tracking for pools removed from a host or collectors that stop reporting

## Quick Start

### Omnibus Image

If you are using the omnibus image (`ghcr.io/starosdev/scrutiny:latest-omnibus`), add these environment variables and device mappings to enable ZFS monitoring:

```yaml
version: '3.5'

services:
  scrutiny:
    image: ghcr.io/starosdev/scrutiny:latest-omnibus
    environment:
      # Enable ZFS pool monitoring
      COLLECTOR_ZFS_CRON_SCHEDULE: "*/15 * * * *"
      COLLECTOR_ZFS_RUN_STARTUP: "true"
    devices:
      - /dev/sda:/dev/sda
      # ...rest of your disks
      - /dev/zfs:/dev/zfs
    # ... rest of your config
```

The ZFS collector binary and `zfsutils-linux` are already included in the omnibus image.

### Hub/Spoke Deployment

For hub/spoke deployments, run the ZFS collector container on each host with ZFS pools, and make sure to include the appropriate device mapping:

```yaml
version: '2.4'

services:
  # ... your existing web and influxdb services ...

  collector-zfs:
    restart: unless-stopped
    image: 'ghcr.io/starosdev/scrutiny:latest-collector-zfs'
    environment:
      COLLECTOR_ZFS_API_ENDPOINT: 'http://web:8080'
      COLLECTOR_ZFS_HOST_ID: 'my-zfs-server'
      COLLECTOR_ZFS_RUN_STARTUP: 'true'
    devices:
      - /dev/zfs:/dev/zfs
    depends_on:
      web:
        condition: service_healthy
```

Note: The ZFS collector container requires access to `zpool` commands. For most setups, this works automatically since the container includes `zfsutils-linux`.

## Environment Variables

### Docker-Only Variables

These variables are handled by the container entrypoint script:

| Variable | Default | Description |
|----------|---------|-------------|
| `COLLECTOR_ZFS_CRON_SCHEDULE` | (empty - disabled) | Cron schedule for ZFS data collection. Example: `*/15 * * * *` for every 15 minutes. **Required to enable ZFS monitoring in omnibus image.** |
| `COLLECTOR_ZFS_RUN_STARTUP` | `false` | Set to `true` to run the ZFS collector immediately when the container starts |
| `COLLECTOR_ZFS_RUN_STARTUP_SLEEP` | `1` | Delay in seconds before startup collection |

### Monitoring-Only Pool Management

Set the web server to monitoring-only mode for ZFS pools when dashboard users should view pool health without changing pool records:

```yaml
web:
  zfs:
    allow_pool_modifications: false
```

Docker deployments can set `SCRUTINY_WEB_ZFS_ALLOW_POOL_MODIFICATIONS=false`. Restart the web server after changing this startup configuration.

When disabled, Scrutiny returns `403` for archive, unarchive, mute, unmute, label, and delete requests. Collector registration, metric uploads, and all read endpoints remain available. The restriction is global because Scrutiny does not define per-user roles; administrators must re-enable the setting and restart the web server before modifying pools.

Archived pools remain available through the dashboard's **Archived** filter. Collector registration updates their metrics but does not unarchive them. Prometheus metrics and generated reports continue to exclude archived pools.

### Collector Configuration Variables

These variables configure the collector binary itself:

| Variable | Default | Description |
|----------|---------|-------------|
| `COLLECTOR_ZFS_API_ENDPOINT` or `COLLECTOR_API_ENDPOINT` | `http://localhost:8080` | URL of the Scrutiny web server API |
| `COLLECTOR_ZFS_HOST_ID` or `COLLECTOR_HOST_ID` | (empty) | Identifier for this host, used to group pools in the dashboard |
| `COLLECTOR_ZFS_LOG_FILE` or `COLLECTOR_LOG_FILE` | (empty) | Path to log file. Leave empty for stdout |
| `COLLECTOR_ZFS_DEBUG` or `COLLECTOR_DEBUG` or `DEBUG` | `false` | Enable debug logging |

## Pool Presence

Collectors with a host ID send a complete inventory on every successful run. An
empty inventory is valid and marks previously known pools on that host as
`missing`; it does not delete or archive them. Pools whose host inventory has
not arrived within the stale window are `stale`. The dashboard, reports, and
Prometheus metrics expose presence separately from the last observed ZFS health,
so an old `ONLINE` result is not presented as current.

Collectors without a host ID use the legacy registration path. Their pool
presence is `unknown` because the server cannot safely associate absence with a
host.

Configure the stale window on the web service with
`SCRUTINY_WEB_ZFS_POOL_STALE_AFTER_MINUTES` (default: `60`).

## Configuration File

The ZFS collector can also be configured via a YAML file. The collector looks for configuration in this order:

1. `/opt/scrutiny/config/collector-zfs.yaml`
2. `/opt/scrutiny/config/collector-zfs.yml`
3. `/opt/scrutiny/config/collector.yaml` (fallback)
4. `/opt/scrutiny/config/collector.yml` (fallback)

Example configuration (`collector-zfs.yaml`):

```yaml
version: 1

host:
  # Unique identifier for this host (used for grouping pools in dashboard)
  id: "my-zfs-server"

api:
  # Scrutiny web server endpoint
  endpoint: "http://localhost:8080"
  # Timeout in seconds for API requests
  timeout: 60

log:
  # Log level: DEBUG, INFO, WARNING, ERROR
  level: INFO
  # Optional: Path to log file (leave empty for stdout)
  file: ""
```

## Manual Installation

If you are not using Docker, you can run the ZFS collector binary directly:

1. Download the `scrutiny-collector-zfs` binary from the [releases page](https://github.com/Staros-Labs/scrutiny/releases)

2. Ensure `zpool` command is available on your system

3. Run the collector:

```bash
# Run once
scrutiny-collector-zfs run --api-endpoint http://localhost:8080

# With debug logging
scrutiny-collector-zfs run --api-endpoint http://localhost:8080 --debug

# With host identifier
scrutiny-collector-zfs run --api-endpoint http://localhost:8080 --host-id my-server
```

4. Set up a cron job to run the collector periodically:

```cron
*/15 * * * * /path/to/scrutiny-collector-zfs run --api-endpoint http://localhost:8080
```

## Verifying ZFS Monitoring

After enabling ZFS monitoring:

1. Wait for the collector to run (or trigger it manually)

2. Open the Scrutiny web UI

3. Click on "ZFS Pools" in the navigation bar

4. You should see your ZFS pools listed with their current status

You can also verify via the API:

```bash
# Check if pools are registered
curl http://localhost:8080/api/zfs/summary

# Check details for a specific pool (replace GUID with your pool's GUID)
curl http://localhost:8080/api/zfs/pool/GUID/details
```

## Raw and Usable Capacity

The UI reports two capacity figures for each pool, because ZFS itself reports two.

`zpool list` gives raw vdev capacity, which counts the parity drives. On an
8-drive raidz3 pool of 14.6 TB drives that is roughly 116 TB, even though
parity means only 5 drives' worth of that is available to store data.

`zfs list` on the pool's root dataset gives the usable figure, `used` plus
`available`, which for the same pool is roughly 66 TB. This is the number that
matches what a `df` on a mounted dataset reports.

The Zpools tab and the pool detail page lead with the usable figure and show
the raw one beneath it. A pool last reported by a collector older than this
feature has no usable figure stored, so it shows only the raw one until the
collector next runs.

Usable capacity moves with compression, quotas, and reservations, so it is not
a fixed property of the pool the way raw capacity is. Two pools built on
identical drives can report different usable sizes.

## Troubleshooting

### ZFS Pools Tab is Empty

1. **ZFS collector not enabled (omnibus)**: Make sure you have set `COLLECTOR_ZFS_CRON_SCHEDULE` environment variable. Without this, the ZFS collector will not run.

2. **Collector has not run yet**: If you just started the container, wait for the cron schedule to trigger, or set `COLLECTOR_ZFS_RUN_STARTUP=true` to run on startup.

3. **Check container logs**: Look for ZFS collector output in your container logs:
   ```bash
   docker logs scrutiny 2>&1 | grep -i zfs
   ```

4. **Verify zpool command works**: The collector requires the `zpool` command to be functional:
   ```bash
   docker exec scrutiny zpool list
   ```

### Collector Errors

1. **API endpoint unreachable**: Verify the web server is running and the endpoint URL is correct.

2. **Permission issues**: The collector needs permission to run `zpool` commands.

3. **Enable debug logging**: Set `COLLECTOR_ZFS_DEBUG=true` or `DEBUG=true` to see detailed output.

### Pool Not Updating

1. **Check cron schedule**: Verify the cron schedule is correct and running:
   ```bash
   docker exec scrutiny cat /etc/cron.d/scrutiny-zfs
   ```

2. **Run collector manually**: Test the collector directly:
   ```bash
   docker exec scrutiny /opt/scrutiny/bin/scrutiny-collector-zfs run
   ```

## API Endpoints

The ZFS monitoring feature exposes the following API endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/zfs/summary` | Get active and archived ZFS pools for dashboard filtering |
| GET | `/api/zfs/pool/:guid/details` | Get detailed pool info with vdev hierarchy |
| POST | `/api/zfs/pools/register` | Register pools (used by collector) |
| POST | `/api/zfs/pool/:guid/metrics` | Upload metrics (used by collector) |
| POST | `/api/zfs/pool/:guid/archive` | Archive a pool (hide from dashboard) |
| POST | `/api/zfs/pool/:guid/unarchive` | Unarchive a pool |
| POST | `/api/zfs/pool/:guid/mute` | Mute notifications for a pool |
| POST | `/api/zfs/pool/:guid/unmute` | Unmute notifications |
| POST | `/api/zfs/pool/:guid/label` | Set a custom label for a pool |
| DELETE | `/api/zfs/pool/:guid` | Delete a pool and its data |
