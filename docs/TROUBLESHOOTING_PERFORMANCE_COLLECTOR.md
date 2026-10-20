# Troubleshooting the Performance Collector

This page covers the fio-based performance collector
(`scrutiny-collector-performance`). For S.M.A.R.T collection problems see
[TROUBLESHOOTING_DEVICE_COLLECTOR.md](TROUBLESHOOTING_DEVICE_COLLECTOR.md).

## Write tests are skipped

The log shows read results but reports skipped writes:

```
level=warning msg="Mount point /dev for device /dev/sdb is not suitable: filesystem type is devtmpfs (device not mounted as a real filesystem) -- falling back to direct device I/O (read-only)"
level=info msg="Skipping sequential write test on sdb (direct device I/O -- writes disabled for safety)"
level=info msg="Skipping random write test on sdb (direct device I/O -- writes disabled for safety)"
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
3. Automatic lookup of a mount point backed by the device or one of its
   partitions.
4. Raw block device, reads only.

Steps 1 and 3 reject a mount point that is a `tmpfs` or `devtmpfs`, is mounted
read-only, or has less free space than the test file needs
(`performance.temp_file_size`, or 1G for the `comprehensive` profile).

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
- ZFS and multi-device btrfs pools
- A device whose mount point is not visible inside the container

Keys are the device name or the WWN, and values are paths as the collector sees
them:

```yaml
performance:
  mount_points:
    sdb: '/data'
    0x5000cca7aae69487: '/mnt/backup'
```

The same settings work as environment variables, which is usually easier in
Docker because no config file is needed:

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
