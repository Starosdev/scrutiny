# AppArmor Troubleshooting

## Problem

On systems with AppArmor enforced (TrueNAS SCALE, Ubuntu, Debian), the Scrutiny collector may fail to read SMART data from drives even when the container is started with `--cap-add SYS_RAWIO --cap-add SYS_ADMIN` and devices are passed through with `--device`.

Symptoms include:

- `smartctl` returns "Permission denied" or fails to open devices
- The collector logs errors like `smartctl could not open device /dev/sdX`
- The collector logs a warning: `AppArmor: the container is using the default Docker/containerd profile which blocks raw device I/O`
- Adding `--cap-add` flags alone does not resolve the issue
- The same configuration works on systems without AppArmor (e.g., Unraid)

## Root Cause

AppArmor's default Docker profile (`docker-default`) restricts raw device I/O operations regardless of Linux capabilities. The `SYS_RAWIO` capability grants kernel-level permission, but AppArmor applies an additional Mandatory Access Control (MAC) layer that independently blocks the SCSI/ATA/NVMe ioctl calls that `smartctl` requires.

## Affected Platforms

| Platform | AppArmor Status | Impact |
|----------|----------------|--------|
| TrueNAS SCALE | Enforced by default | Primary affected platform |
| Ubuntu / Debian | Enforced by default | Affected with default Docker profile |
| Unraid | Not used | Not affected |
| Proxmox LXC | Varies | Potentially affected |

## Solutions

### Option 1: Custom AppArmor Profile (Recommended)

Scrutiny ships a minimal AppArmor profile that grants only the specific access `smartctl` needs while keeping the rest of Docker's confinement in place.

**Step 1: Copy the profile to the host**

The profile is included in the Docker image at `/opt/scrutiny/apparmor-profile`. Copy it to the host's AppArmor profile directory:

```bash
# From the Docker image:
docker cp <container_name>:/opt/scrutiny/apparmor-profile /etc/apparmor.d/scrutiny-collector

# Or from the repository:
sudo cp docker/apparmor-profile /etc/apparmor.d/scrutiny-collector
```

**Step 2: Load the profile**

```bash
sudo apparmor_parser -r /etc/apparmor.d/scrutiny-collector
```

**Step 3: Configure the container to use the profile**

Docker run:

```bash
docker run \
  --cap-add SYS_RAWIO --cap-add SYS_ADMIN \
  --security-opt apparmor=scrutiny-collector \
  --device=/dev/sda --device=/dev/sdb \
  ghcr.io/starosdev/scrutiny:latest-collector
```

Docker Compose:

```yaml
services:
  scrutiny:
    image: ghcr.io/starosdev/scrutiny:latest-omnibus
    cap_add:
      - SYS_RAWIO
      - SYS_ADMIN
    security_opt:
      - apparmor=scrutiny-collector
    devices:
      - "/dev/sda"
      - "/dev/sdb"
```

**Step 4: Verify the profile is loaded (optional)**

```bash
sudo aa-status | grep scrutiny
```

The profile persists across reboots because AppArmor automatically loads profiles from `/etc/apparmor.d/` at boot.

### Option 2: Disable AppArmor for the Container

This is simpler but provides less security because the container runs without any AppArmor confinement.

Docker run:

```bash
docker run \
  --cap-add SYS_RAWIO --cap-add SYS_ADMIN \
  --security-opt apparmor=unconfined \
  --device=/dev/sda --device=/dev/sdb \
  ghcr.io/starosdev/scrutiny:latest-collector
```

Docker Compose:

```yaml
services:
  scrutiny:
    image: ghcr.io/starosdev/scrutiny:latest-omnibus
    cap_add:
      - SYS_RAWIO
      - SYS_ADMIN
    security_opt:
      - apparmor=unconfined
    devices:
      - "/dev/sda"
      - "/dev/sdb"
```

### Option 3: Privileged Mode (Not Recommended)

Running with `--privileged` disables all security restrictions. This works but is not recommended for production.

```bash
docker run --privileged --device=/dev/sda ghcr.io/starosdev/scrutiny:latest-collector
```

## Security Comparison

| Approach | AppArmor | Capabilities | Device Write | Risk Level |
|----------|----------|-------------|-------------|------------|
| Custom profile | Enforced (minimal) | SYS_RAWIO + SYS_ADMIN | Denied | Low |
| `apparmor=unconfined` | Disabled | SYS_RAWIO + SYS_ADMIN | Depends on caps | Medium |
| `--privileged` | Disabled | All | Allowed | High |

The custom profile explicitly denies writes to block devices, restricts mount operations, and only allows the specific capabilities and device access that `smartctl` requires.

## TrueNAS SCALE

TrueNAS SCALE manages Docker containers through its app system and enforces AppArmor by default. To use the custom profile:

1. SSH into the TrueNAS host
2. Copy the AppArmor profile as described in Option 1
3. Load the profile with `apparmor_parser`
4. In the TrueNAS app configuration, add the security option `apparmor=scrutiny-collector`

If the TrueNAS app UI does not expose `security_opt`, use Option 2 (`apparmor=unconfined`) or deploy via `docker-compose` directly.

## Diagnostic Logging

The collector automatically detects AppArmor confinement at startup and logs warnings when a restrictive profile is detected. Look for log lines starting with `AppArmor:` in the collector output.

When a device open failure occurs (smartctl exit code 0x02), the collector will additionally log a hint if AppArmor confinement is detected.

To enable verbose logging for troubleshooting:

```bash
docker run -e COLLECTOR_DEBUG=true ... ghcr.io/starosdev/scrutiny:latest-collector
```

## Verifying AppArmor Status

Check if AppArmor is active on the host:

```bash
sudo aa-status
```

Check which profile a running container is using:

```bash
docker inspect <container_id> | grep -i apparmor
```

Check the profile from inside the container:

```bash
cat /proc/self/attr/apparmor/current
```
