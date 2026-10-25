# Troubleshooting the Performance Collector

This page covers the fio-based performance collector
(`scrutiny-collector-performance`). For S.M.A.R.T collection problems see
[TROUBLESHOOTING_DEVICE_COLLECTOR.md](TROUBLESHOOTING_DEVICE_COLLECTOR.md).

## Write tests are skipped

The log shows read results but reports skipped writes:

```
level=warning msg="No mount point found for /dev/sdb or its partitions -- falling back to direct device I/O (read-only, write tests skipped). See docs/TROUBLESHOOTING_PERFORMANCE_COLLECTOR.md"
level=info msg="Skipping sequential write test on sdb (direct device I/O -- writes disabled for safety)"
level=info msg="Skipping random write test on sdb (direct device I/O -- writes disabled for safety)"
```

When the device does back a mount that the collector refused, the warning names
every reason instead:

```
level=warning msg="No usable mount point for /dev/sdb (/mnt/archive is mounted read-only (benchmark test file cannot be created)) -- falling back to direct device I/O (read-only, write tests skipped). See docs/TROUBLESHOOTING_PERFORMANCE_COLLECTOR.md"
```

Write tests need a filesystem. The collector writes a temporary file
(`.scrutiny_perf_bench`) into a mount point backed by the target device, then
deletes it. When no usable mount point is found, the collector benchmarks the
raw block device instead, and raw-device write tests are disabled
unconditionally so a benchmark can never overwrite data.

To get write results, give the collector a writable mount point on the target
device.

## How the benchmark target is chosen

1. `performance.mount_points` for the device name, then for its WWN.
2. `performance.allow_direct_device_io`, when set, benchmarks the raw device
   (reads only).
3. Automatic lookup of the mount points backed by the device or one of its
   partitions, in order, until one is usable.
4. Raw block device, reads only.

A mount point is rejected when it does not exist, when it is not a directory,
when the mount table cannot be read or does not account for it, when its
filesystem is a pseudo-filesystem or a container overlay, when it is a path the container
runtime bind-mounts into every container (`/etc/hosts`, `/etc/resolv.conf`,
`/opt/scrutiny/config`, `/opt/scrutiny/influxdb`), when it is mounted read-only,
when free space is below the test file size plus 64 MiB of headroom, or when the
collector cannot create a file in it. The test file size is
`performance.temp_file_size`, or 1G for the `comprehensive` profile.

The last check creates and removes a real file, because a writable filesystem
and a writable directory are not the same thing: a root-owned path or an
exhausted inode quota passes every other check and then fails inside fio, which
publishes a zero instead of skipping the test. The probe file is empty, so a
block quota is not detected.

The space, filesystem type and read-only checks read `/proc/mounts` and are
therefore Linux-only. On macOS and FreeBSD a mount point is accepted when it
exists, is a directory, and accepts the probe file.

### The root filesystem is an eligible target

A device backs whatever is mounted on it, including `/`. A single-disk server
running the collector outside a container has no other candidate, so the
benchmark writes its test file into the root filesystem and runs its write
tests there. That is the only way to measure writes on such a host, but it is
I/O against the system disk on every scheduled run. Set
`performance.allow_direct_device_io: true` to keep read-only results instead, or
name a different filesystem with `performance.mount_points`.

Step 3 considers every mount the device backs, so a device whose first candidate
is too small (an EFI system partition, say) still resolves to a larger one. Each
rejected candidate is logged with its reason at debug level, and when no
candidate is usable the collector logs every reason together at warning level.

## Containers need the filesystem mounted in

Passing `--device=/dev/sdb` or mounting `/dev` gives the collector the device
node, which is enough for reads. It does not give it the filesystem. Mount the
filesystem into the container as well, read-write:

```bash
docker run --restart unless-stopped \
  --cap-add SYS_RAWIO \
  --cap-add SYS_ADMIN \
  --device=/dev/sdb \
  -v /data:/data \
  -e COLLECTOR_PERF_API_ENDPOINT=http://SCRUTINY_WEB_IPADDRESS:8080 \
  ghcr.io/starosdev/scrutiny:latest-collector-performance
```

A path mounted `:ro` is rejected with `is mounted read-only (benchmark test file
cannot be created)`, and the collector falls back to read-only raw device I/O.

## Setting a mount point explicitly

Automatic lookup matches a device against the mount table, including its
partitions, so `/dev/sdb` resolves through `/dev/sdb1`. It cannot resolve a
device whose filesystem is reached through another layer. Configure these
explicitly:

- LVM logical volumes, where the mount source is `/dev/mapper/...`
- mdadm and hardware RAID members, where the array is mounted, not the member
- ZFS, where the mount source is the pool name rather than a device. A dataset
  is a valid benchmark target once it is named here, on OpenZFS 2.3 or newer.
  Earlier releases do not implement `O_DIRECT` on files and reject every fio
  job the collector runs, which publishes no result at all for that device.
  Check with `zfs version`, and prefer `allow_direct_device_io` below 2.3.
- A device whose mount point is not visible inside the container

Multi-device btrfs needs care rather than configuration. The mount table lists
the pool with whichever member device was passed at mount time, so that one
member resolves to the pool mount and the others do not. Benchmark writes then
land on every disk in the pool while the results are recorded against the one
device, which is misleading on RAID1 and worse on RAID10. Point such a device at
a single-device filesystem, or set `performance.allow_direct_device_io: true` to
keep read-only results.

Keys are the device name or the WWN, and values are paths as the collector sees
them. A symlink is resolved before the checks run, so the filesystem that gets
inspected is the one that gets written to:

```yaml
performance:
  mount_points:
    sdb: '/data'
    0x5000cca7aae69487: '/mnt/backup'
```

The same settings work as environment variables, which is usually easier in
Docker because no config file is needed. The key is uppercased, and `-` becomes
`_`, so a device named `dm-0` reads `..._DM_0`:

```yaml
environment:
  COLLECTOR_PERFORMANCE_MOUNT_POINTS_SDB: '/data'
  COLLECTOR_PERFORMANCE_MOUNT_POINTS_0X5000CCA7AAE69487: '/mnt/backup'
```

A device with no mounted filesystem at all, such as a blank disk or an unmounted
partition, cannot run write tests. Mount it, or expect read-only results.

## Benchmarks run but every value is zero

The collector skips publishing when all read tests return zero, and logs the
device as inaccessible. Check that the device node is passed into the container
and that `fio` can open it. Zero *write* values with non-zero read values mean
the write tests were skipped; see the sections above.

## Numbers are lower than the drive's rating

A filesystem benchmark includes filesystem overhead, and the `quick` profile
runs a single job for 10 seconds per test. Compare results against the device's
own history, which is what degradation detection does, rather than against the
manufacturer's specification.
