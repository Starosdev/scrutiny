#!/bin/bash

set -euo pipefail

if ! command -v btrfs >/dev/null 2>&1; then
    echo "btrfs command not found. Install btrfs-progs first." >&2
    exit 1
fi

OUT_DIR="${1:-./btrfs-capture-$(date +%Y%m%d-%H%M%S)}"
mkdir -p "${OUT_DIR}"

run_and_capture() {
    local output_name="$1"
    shift

    echo "$ $*" | tee "${OUT_DIR}/${output_name}.cmd.txt"
    if "$@" >"${OUT_DIR}/${output_name}.txt" 2>"${OUT_DIR}/${output_name}.stderr.txt"; then
        echo "captured ${output_name}"
    else
        status=$?
        echo "command failed with exit code ${status}" >>"${OUT_DIR}/${output_name}.stderr.txt"
        echo "capture failed for ${output_name} (exit ${status})"
    fi
}

{
    echo "captured_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "hostname=$(hostname)"
    echo "kernel=$(uname -a)"
    echo "uid=$(id -u)"
    echo "user=$(id -un)"
    echo "btrfs_path=$(command -v btrfs)"
    btrfs version 2>/dev/null | sed 's/^/btrfs_version=/'
} >"${OUT_DIR}/environment.txt"

run_and_capture proc-mounts cat /proc/mounts
run_and_capture filesystem-show btrfs filesystem show
run_and_capture filesystem-usage-all btrfs filesystem usage --raw /
run_and_capture filesystem-show-all btrfs filesystem show

mapfile -t mount_points < <(awk '$3 == "btrfs" {print $2}' /proc/mounts | sort -u)

if [ "${#mount_points[@]}" -eq 0 ]; then
    echo "No mounted Btrfs filesystems found." | tee "${OUT_DIR}/README.txt"
    exit 0
fi

printf "%s\n" "${mount_points[@]}" >"${OUT_DIR}/mount-points.txt"

for mount_point in "${mount_points[@]}"; do
    safe_name=$(echo "${mount_point}" | sed 's#^/##; s#[^A-Za-z0-9._-]#_#g')
    [ -n "${safe_name}" ] || safe_name="root"

    run_and_capture "${safe_name}-filesystem-show" btrfs filesystem show "${mount_point}"
    run_and_capture "${safe_name}-filesystem-usage" btrfs filesystem usage --raw "${mount_point}"
    run_and_capture "${safe_name}-device-stats" btrfs device stats "${mount_point}"
    run_and_capture "${safe_name}-scrub-status" btrfs scrub status --raw "${mount_point}"
done

cat <<EOF >"${OUT_DIR}/README.txt"
Captured Btrfs host state for Scrutiny validation.

Files in this directory map directly to the parser inputs used by the Btrfs collector:
- filesystem-show*.txt
- filesystem-usage*.txt
- device-stats*.txt
- scrub-status*.txt

Review the matching .stderr.txt files for privilege or runtime failures.
EOF
