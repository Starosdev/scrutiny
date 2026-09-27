#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
preflight="${repo_root}/rootfs/usr/local/bin/scrutiny-influxdb-upgrade-preflight"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

assert_file_exists() {
    [[ -f "$1" ]] || fail "expected file to exist: $1"
}

assert_file_missing() {
    [[ ! -e "$1" ]] || fail "expected file to be absent: $1"
}

run_preflight() {
    local data_dir="$1"
    local confirmation="${2:-false}"
    INFLUXD_CONFIG_PATH="$data_dir" \
        SCRUTINY_INFLUXDB_29_BACKUP_CONFIRMED="$confirmation" \
        bash "$preflight"
}

fresh_dir="${test_root}/fresh"
fresh_output="$(run_preflight "$fresh_dir")"
assert_file_exists "${fresh_dir}/.scrutiny-influxdb-2.9-preflight-complete"
grep -q "confirmation is not required" <<< "$fresh_output" || fail "fresh install message missing"

blocked_dir="${test_root}/blocked"
mkdir -p "$blocked_dir"
touch "${blocked_dir}/influxd.bolt"
set +e
blocked_output="$(run_preflight "$blocked_dir" 2>&1)"
blocked_status=$?
set -e
[[ "$blocked_status" -eq 78 ]] || fail "existing data should exit 78, got $blocked_status"
assert_file_missing "${blocked_dir}/.scrutiny-influxdb-2.9-preflight-complete"
grep -q "SCRUTINY_INFLUXDB_29_BACKUP_CONFIRMED=true" <<< "$blocked_output" || fail "acknowledgement instructions missing"

invalid_dir="${test_root}/invalid"
mkdir -p "$invalid_dir"
touch "${invalid_dir}/influxd.sqlite"
set +e
run_preflight "$invalid_dir" yes >/dev/null 2>&1
invalid_status=$?
set -e
[[ "$invalid_status" -eq 78 ]] || fail "only exact true should acknowledge the backup"
assert_file_missing "${invalid_dir}/.scrutiny-influxdb-2.9-preflight-complete"

confirmed_output="$(run_preflight "$blocked_dir" true)"
assert_file_exists "${blocked_dir}/.scrutiny-influxdb-2.9-preflight-complete"
grep -q "backup confirmation accepted" <<< "$confirmed_output" || fail "confirmation message missing"

marker_output="$(run_preflight "$blocked_dir")"
[[ -z "$marker_output" ]] || fail "persistent marker should bypass repeated confirmation"

engine_dir="${test_root}/engine"
mkdir -p "${engine_dir}/engine/data"
touch "${engine_dir}/engine/data/shard"
set +e
run_preflight "$engine_dir" >/dev/null 2>&1
engine_status=$?
set -e
[[ "$engine_status" -eq 78 ]] || fail "existing engine data should require confirmation"

upgraded_dir="${test_root}/upgraded"
mkdir -p "$upgraded_dir"
touch "${upgraded_dir}/influxd.bolt" "${upgraded_dir}/influxd.bolt.pre-v2.9.1-upgrade.backup"
upgraded_output="$(run_preflight "$upgraded_dir")"
assert_file_exists "${upgraded_dir}/.scrutiny-influxdb-2.9-preflight-complete"
grep -q "already upgraded to 2.9" <<< "$upgraded_output" || fail "upgrade backup should complete the preflight"

old_backup_dir="${test_root}/old-backup"
mkdir -p "$old_backup_dir"
touch "${old_backup_dir}/influxd.bolt" "${old_backup_dir}/influxd.bolt.pre-v2.2.0-upgrade.backup"
set +e
run_preflight "$old_backup_dir" >/dev/null 2>&1
old_backup_status=$?
set -e
[[ "$old_backup_status" -eq 78 ]] || fail "a pre-2.9 upgrade backup should not complete the preflight"

# The run script must act on the preflight result: a caller that ignores it lets influxd
# migrate data the preflight blocked (#900). Run a copy with the preflight, influxd, and sleep
# replaced by stubs that record how they were called.
run_script="${repo_root}/rootfs/etc/services.d/influxdb/run"
run_service() {
    local preflight_status="$1"
    local dir="${test_root}/run-${preflight_status}"
    mkdir -p "${dir}/bin" "${dir}/influxdb"
    printf '#!/usr/bin/env bash\nexit %s\n' "$preflight_status" > "${dir}/bin/preflight"
    printf '#!/usr/bin/env bash\necho "influxd $*" >> %q\n' "${dir}/calls" > "${dir}/bin/influxd"
    printf '#!/usr/bin/env bash\necho "sleep $*" >> %q\n' "${dir}/calls" > "${dir}/bin/sleep"
    chmod +x "${dir}/bin/"*
    sed -e "s|/usr/local/bin/scrutiny-influxdb-upgrade-preflight|${dir}/bin/preflight|" \
        -e "s|/opt/scrutiny/influxdb|${dir}/influxdb|g" \
        "$run_script" > "${dir}/run"
    PATH="${dir}/bin:${PATH}" bash "${dir}/run" >/dev/null 2>&1
    cat "${dir}/calls"
}

[[ "$(run_service 78)" == "sleep infinity" ]] || fail "run script must not start influxd when the preflight blocks"
[[ "$(run_service 0)" == "influxd run" ]] || fail "run script must start influxd when the preflight passes"

echo "InfluxDB upgrade preflight tests passed"
