# Filesystem Capacity Monitoring

Scrutiny can now collect logical filesystem capacity alongside existing SMART device health and ZFS pool metrics.

## Scope

- Filesystem capacity is reported separately from physical drive SMART data.
- Dashboard device cards and the device detail page report the free space of the filesystems a drive backs, in addition to the drive's physical capacity.
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
- backing devices

## Per-drive attribution

A filesystem's source device is rarely the disk a user is looking at. `/dev/sda1`
is a partition, `/dev/mapper/vg-root` is a logical volume, `/dev/md0` is an
array, and a LUKS volume sits on top of any of those. The collector walks the
kernel's holder tree under `/sys/class/block` from the mount source down to the
whole disks beneath it, and uploads those disk names with each snapshot. Scrutiny
stores one row per (filesystem, disk) pair, so a drive holding several
filesystems and a filesystem spanning several drives are both represented.

Scrutiny reports a single free-space figure for a drive only when exactly one
filesystem sits on it and that filesystem sits on no other drive. In every other
case the dashboard card shows the number of mounts and the device detail page
lists each filesystem with its own used, free and total bytes. Free space shared
between drives is never divided between them: the kernel does not report a
per-drive split, so any such number would be invented.

The walk reads sysfs and therefore produces attribution on Linux hosts only. A
filesystem the collector cannot attribute is still reported for the host
capacity panel, and the device detail page says why no filesystem is listed
rather than showing zero free space.

## Visibility Limits

The collector reports filesystem capacity only when it can inspect host mounts.

- If host mounts are visible, Scrutiny stores the latest filesystem snapshot for that host.
- If mounts are not visible, Scrutiny marks filesystem capacity as unavailable for that host.
- Scrutiny does not infer or approximate per-drive free space from SMART data. Per-drive figures come from the collector's holder-tree walk or they are absent.
- A collector older than the backing-device field uploads snapshots with no attribution. The host capacity panel still works; per-drive free space appears after the collector is upgraded.

This matters most in Docker and remote collector setups where the collector may see block devices but not the host filesystem namespace.

## Filtering

The filesystem collector excludes pseudo-filesystems and skips ZFS-backed mounts to avoid duplicating the dedicated ZFS capacity surface.

A btrfs filesystem spanning several devices appears in `/proc/mounts` with one
source device, so it is attributed to that device's disk only. Use the dedicated
btrfs surface for its full device list.
