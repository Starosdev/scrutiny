# Filesystem Capacity Monitoring

Scrutiny can now collect logical filesystem capacity alongside existing SMART device health and ZFS pool metrics.

## Scope

- Filesystem capacity is reported separately from physical drive SMART data.
- Existing dashboard device cards still show physical drive capacity only.
- ZFS pool capacity remains on the existing ZFS dashboard/API path.

## Collector

Use the dedicated filesystem collector:

```bash
scrutiny-collector-filesystem run
```

In the omnibus image, this binary is available at:

```bash
/opt/scrutiny/bin/scrutiny-collector-filesystem run
```

In omnibus deployments, automatic collection is disabled by default. Enable it with:

```bash
COLLECTOR_FILESYSTEM_CRON_SCHEDULE="*/15 * * * *"
COLLECTOR_FILESYSTEM_RUN_STARTUP="true"
```

The collector reads host-visible mount information and uploads per-filesystem snapshots containing:

- host ID
- mount point
- source device
- filesystem type
- total bytes
- used bytes
- available bytes
- used percent

## Visibility Limits

The collector reports filesystem capacity only when it can inspect host mounts.

- If host mounts are visible, Scrutiny stores the latest filesystem snapshot for that host.
- If mounts are not visible, Scrutiny marks filesystem capacity as unavailable for that host.
- Scrutiny does not infer or approximate per-drive free space from SMART data.

This matters most in Docker and remote collector setups where the collector may see block devices but not the host filesystem namespace.

## Filtering

The filesystem collector excludes pseudo-filesystems and skips ZFS-backed mounts to avoid duplicating the dedicated ZFS capacity surface.
