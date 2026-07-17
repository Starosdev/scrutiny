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

echo "InfluxDB upgrade preflight tests passed"
