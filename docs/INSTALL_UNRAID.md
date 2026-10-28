# UnRAID Install

Installation of Scrutiny in UnRAID follows the same process as installing any other docker container, utilizing the Community Applications plugin.

## Install the 'Community Applications' Plugin

All docker containers in UnRAID are typically installed utilizing the Community Applications plugin. To get started:
- Navigate to the plugins tab ( <UnRaid_IP_Address>/Plugins )
- Select the 'Install Plugin' tab, and enter the following address into the input field
```
https://raw.githubusercontent.com/Squidly271/community.applications/master/plugins/community.applications.plg
```

You're all set with the pre-requisites!

## Official Starosdev Templates

This project provides official Unraid CA templates in the [`docker/unraid/`](../docker/unraid/) directory. You can install them manually by copying the XML files to your Unraid templates directory:

```
/boot/config/plugins/dockerMan/templates-user/
```

### Omnibus (Recommended)

The omnibus template (`scrutiny-omnibus.xml`) is the easiest way to get started. It runs the web dashboard, metrics collector, and InfluxDB in a single container.

**After installing, you must add your drives:**
1. Edit the container in the Unraid Docker tab
2. Click "Add another Path, Port, Variable, Label or Device"
3. Select "Device" and enter the path to each drive you want to monitor (e.g. `/dev/sda`, `/dev/sdb`)
4. For NVMe drives, add both the controller (`/dev/nvme0`) and namespace (`/dev/nvme0n1`)

### Hub/Spoke (Advanced)

For distributed deployments, separate templates are available:

| Template | Image | Purpose |
|----------|-------|---------|
| `scrutiny-web.xml` | `ghcr.io/starosdev/scrutiny:latest-web` | Web dashboard and API. Requires external InfluxDB. |
| `scrutiny-collector.xml` | `ghcr.io/starosdev/scrutiny:latest-collector` | S.M.A.R.T metrics collector. Point it at your web server. |
| `scrutiny-collector-omnibus.xml` | `ghcr.io/starosdev/scrutiny:latest-collector-omnibus` | Recommended single spoke container. Bundles the SMART, ZFS, MDADM, Btrfs, filesystem, and performance collectors. |
| `scrutiny-collector-zfs.xml` | `ghcr.io/starosdev/scrutiny:latest-collector-zfs` | ZFS pool health collector. |
| `scrutiny-collector-btrfs.xml` | `ghcr.io/starosdev/scrutiny:latest-collector-btrfs` | Btrfs filesystem health collector. Requires host Btrfs mounts and `btrfs-progs` access. |
| `scrutiny-collector-performance.xml` | `ghcr.io/starosdev/scrutiny:latest-collector-performance` | fio drive benchmarks for throughput/IOPS/latency tracking. |

See [Hub/Spoke Installation](INSTALL_HUB_SPOKE.md) for architecture details.

## MDADM Note

For the general Linux deployment and troubleshooting guide, see [MDADM_MONITORING.md](./MDADM_MONITORING.md).

Unraid arrays are supported. Unraid does not use Linux MD RAID, so the collector
detects its format separately:

- Unraid writes `/proc/mdstat` as `key=value` metadata (`mdState=STARTED`, `rdevStatus.3=DISK_OK`) instead of the standard `md0 : active raid1 ...` lines
- Unraid ships no `mdadm` binary, so the collector reads everything it needs from `/proc/mdstat` alone
- the whole Unraid array is one parity-protected set, so it is reported as a single array named `unraid-array`
- the reported level is `unraid-parity1`, `unraid-parity2`, or `unraid-noparity`, depending on how many parity disks are assigned

Mount `/proc/mdstat` into the container as described in
[MDADM_MONITORING.md](./MDADM_MONITORING.md). No `/dev` access or `SYS_ADMIN`
capability is needed for Unraid detection, because no `mdadm` command is run.

Two differences from a Linux `mdadm` host are expected:

- the array's used space reads as zero, because every Unraid data disk carries its own filesystem and there is no single mount point to measure
- the raw `mdstat` shown on the array details page lists the array-level keys only; the per-disk slots are omitted because Unraid reports all 30 slots on every run

For standard Linux MDADM array-ingestion validation, use a host with actual
`mdadm` arrays.

## Third-Party Docker Images

As a docker image can be created using various OS bases, the image choice is entirely the users choice. Recommendations of a specific image from a specific maintainer is beyond the scope of this guide. However, to provide some context given the number of questions posed regarding the various versions available:

- **ghcr.io/starosdev/scrutiny:latest-omnibus**
    - `Image maintained directly by the project maintainers`
    - `Debian based docker image`
- **linuxserver/scrutiny**
    - `Image maintained by the LinuxServer.io group`
    - `Alpine based docker image`
- **hotio/scrutiny**
    - `Image maintained by hotio`
    - `DETAILS TBD`

The support for a given image is provided by that images maintainers, while support for the application itself remains with the developer - i.e. LinuxServer.io supports the docker image of Scrutiny which they create, to the extent an issue is specific to that image. If an issue/enhancement pertains directly to the source code, support would still come directly from this repository's contributors.
