# Btrfs Real-Host Capture Layout

Issue `#516` requires validating the parser against at least one real Btrfs host.

Use [`.github/scripts/capture-btrfs-host-state.sh`](../../../../.github/scripts/capture-btrfs-host-state.sh) on a Linux host with mounted Btrfs filesystems, then copy the resulting files into a subdirectory here.

Recommended layout:

```text
collector/pkg/btrfs/testdata/
  real-host-single-device/
    environment.txt
    proc-mounts.txt
    filesystem-show-all.txt
    mount-points.txt
    root-filesystem-show.txt
    root-filesystem-usage.txt
    root-device-stats.txt
    root-scrub-status.txt
  real-host-multi-device/
    ...
```

When adding a new real-host fixture:

- Keep the raw command output intact.
- Preserve the matching `*.stderr.txt` files if any command required root or returned partial output.
- Add a short note describing whether the filesystem was single-device, RAID1/10, degraded, or missing a device.
- Update `detect_test.go` to parse the captured output if the fixture exposes a new format edge case.
