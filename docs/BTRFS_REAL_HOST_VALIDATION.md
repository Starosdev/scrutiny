# Btrfs Real-Host Validation

This note tracks the real-host validation work for issue `#516`.

## Current Status

No real Btrfs host was reachable from this worktree session, so the acceptance criterion `At least one real Btrfs environment is exercised` is still open.

What this session does provide:

- A repeatable host capture script at [`.github/scripts/capture-btrfs-host-state.sh`](../.github/scripts/capture-btrfs-host-state.sh)
- A fixture layout for storing raw command outputs at [`collector/pkg/btrfs/testdata/README.md`](../collector/pkg/btrfs/testdata/README.md)
- A concrete checklist for validating parser assumptions against live command output

## Commands To Capture

Run the capture script on the target Linux host:

```bash
curl -L https://raw.githubusercontent.com/Starosdev/scrutiny/master/.github/scripts/capture-btrfs-host-state.sh -o capture-btrfs-host-state.sh
chmod +x capture-btrfs-host-state.sh
sudo ./capture-btrfs-host-state.sh
```

Or, from a checked-out repo on the target host:

```bash
sudo ./.github/scripts/capture-btrfs-host-state.sh
```

The script captures:

- `/proc/mounts`
- `btrfs filesystem show`
- `btrfs filesystem usage --raw`
- `btrfs device stats`
- `btrfs scrub status --raw`
- stderr output for each command to expose privilege or runtime failures

## Validation Checklist

For each captured filesystem, confirm:

- Whether `btrfs filesystem show` always emits `Label:` and `uuid:` in the format assumed by `parseFilesystemShow`
- Whether degraded or missing devices appear as `path missing` exactly as assumed by the parser
- Whether multi-device outputs change ordering, indentation, or path formatting in a way that requires parser updates
- Whether `btrfs filesystem usage --raw` emits every expected `Overall` key and whether any values are omitted for unprivileged users
- Whether `btrfs device stats` uses the same key names the collector maps into read/write/flush/corruption/generation errors
- Whether `btrfs scrub status --raw` requires root, a mounted filesystem, or `/var/lib/btrfs` visibility to produce stable output
- Whether containerized runs need additional bind mounts beyond the current `/mnt` and `/var/lib/btrfs` guidance

## Parser Assumptions To Recheck

The current collector implementation assumes:

- Mounted Btrfs filesystems can be discovered from `/proc/mounts`
- One mounted subvolume per source is enough to represent a filesystem
- UUIDs match the lowercase hexadecimal format used by the current regex
- `device stats` and `scrub status` failures are non-fatal and should degrade data quality rather than abort the collector

Any real-host output that contradicts those assumptions should be fed back into:

- `collector/pkg/btrfs/detect.go`
- `collector/pkg/btrfs/detect_test.go`
- `docs/BTRFS_FILESYSTEM_MONITORING.md`

## Docker-Specific Checks

When validating inside Docker, record:

- Which host mount roots had to be bind-mounted for `btrfs filesystem show <mount>` to succeed
- Whether `SYS_ADMIN` was sufficient or whether the runtime needed a more privileged mode
- Whether scrub status changed when `/var/lib/btrfs` was or was not mounted into the container
- Whether `btrfs filesystem usage --raw` lost fields when the container ran without root
