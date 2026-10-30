# Filesystem Capacity Monitoring

Scrutiny can now collect logical filesystem capacity alongside existing SMART device health and ZFS pool metrics.

## Scope

- Filesystem capacity is reported separately from physical drive SMART data.
- The device detail page reports the free space of every filesystem a drive backs, in addition to the drive's physical capacity.
- Dashboard device cards report one free-space figure only when the attribution is unambiguous, and a mount count otherwise. See "Per-drive attribution" below.
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

Attribution needs sysfs, not device nodes. In a container `/dev` holds only the
devices the deployment passes through, which are whole disks, while a mount
source is usually a partition such as `/dev/md1p1` or `/dev/sdm1`. A source
naming one segment under `/dev` is read as that kernel name whether or not the
node exists, and sysfs decides whether the name is real. A nested source such as
`/dev/mapper/vg-root` still needs its node, because only the symlink records
what it points at.

Two names the kernel reports are translated before upload, because the drive
Scrutiny registers is not the block device the walk finds:

- An NVMe filesystem resolves to a namespace, `nvme0n1`, while the SMART
  collector registers the controller, `nvme0`. The namespace is reduced to its
  controller, and several namespaces on one controller collapse to that one
  drive.
- An Unraid array device, `md1p1`, is a top-level virtual block device with no
  `partition` file and an empty `slaves` directory, so the holder tree cannot
  reach the disk beneath it. The pairing is read from Unraid's own mdstat,
  `diskName` against `rdevName`, which is the only place it exists. It is
  published only while the array is started, so a stopped array attributes
  nothing rather than guessing.

The walk reads sysfs and therefore produces attribution on Linux hosts only. A
filesystem the collector cannot attribute is still reported for the host
capacity panel, and the device detail page says why no filesystem is listed
rather than showing zero free space.

An md array member that holds none of the array's data is skipped: an idle hot
spare, which waits until a disk fails, and a write-journal member, which holds
the journal. The walk reads each member's role from `md/dev-<name>/slot`, which
the kernel answers with `journal`, with `none` for a member holding no slot in
the array, and with the slot number otherwise. A member being rebuilt already
holds its slot, so it keeps its filesystems through the resync.

`md/dev-<name>/state` cannot answer this, which is why the slot is read
instead: the kernel prints `spare` for any member that is not in sync, faulty
or a journal, so a member mid-recovery reads exactly like an idle spare.

A member whose slot cannot be read is kept: over-attributing one drive is a
smaller error than hiding a data-carrying drive's filesystems.

A dm-cache or bcache cache device is not separated from the origin device this
way, because the holder tree alone does not record which role a member plays.
Such a cache device is attributed the filesystem it fronts.

Per-drive views collapse repeated mounts of one filesystem, which is what a bind
mount and a set of btrfs subvolume mounts produce: the mounts share one block
device, so they are one filesystem reported several times. The per-host capacity
panel still lists every mount.

Loop, zram and ram devices are not reported as backing devices. They are
kernel-synthesized rather than drives, so Scrutiny never registers them and a
link naming one could match nothing.

The filesystem collector and the SMART collector must agree on `host.id`, which
is how a filesystem is matched to a drive. With `COLLECTOR_HOST_ID` unset both
derive the same id from the machine, taking the hostname and falling back to
`/etc/machine-id`, so two collectors on one machine agree and two machines do
not collide. Set `COLLECTOR_HOST_ID` anyway when the collectors run in separate
containers: each container has its own hostname, so they derive different ids
and nothing is attributed. The device detail page names that case when it sees
the collector reporting for other hosts but not for the drive's own host id.

Collectors before this behavior uploaded an empty host id for drives and the
literal `default` for filesystems. Those rows are not migrated: after the
upgrade the host appears under its derived or configured id, and the old
`default` heading keeps the figures it last held. Host management lists it,
with no devices against it, so it can be purged there.

A drive that reports no model, serial number and WWN takes its identity from
its device name and host id, so changing the host id gives it a new identity
and its SMART history stays under the old one. Drives that report any of the
three keep their history.

Purging a SMART host deletes its filesystem capacity rows with its devices.
Keeping them would attribute the purged host's free space to whichever drive
later registers under the same device name on the same host id.

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
