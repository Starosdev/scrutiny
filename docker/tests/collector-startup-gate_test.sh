#!/usr/bin/env bash
# Verifies that the omnibus run-once services honor an explicit *_RUN_STARTUP=false
# while still running when the variable is unset. See issue #1009.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

# Each service is described by its directory, its cron schedule variable, its
# startup variable, and the collector binary it invokes.
services=(
    "collector-once|COLLECTOR_CRON_SCHEDULE|COLLECTOR_RUN_STARTUP|scrutiny-collector-metrics|always"
    "collector-performance-once|COLLECTOR_PERF_CRON_SCHEDULE|COLLECTOR_PERF_RUN_STARTUP|scrutiny-collector-performance|optional"
    "collector-zfs-once|COLLECTOR_ZFS_CRON_SCHEDULE|COLLECTOR_ZFS_RUN_STARTUP|scrutiny-collector-zfs|optional"
    "collector-mdadm-once|COLLECTOR_MDADM_CRON_SCHEDULE|COLLECTOR_MDADM_RUN_STARTUP|scrutiny-collector-mdadm|optional"
    "collector-btrfs-once|COLLECTOR_BTRFS_CRON_SCHEDULE|COLLECTOR_BTRFS_RUN_STARTUP|scrutiny-collector-btrfs|optional"
    "collector-filesystem-once|COLLECTOR_FILESYSTEM_CRON_SCHEDULE|COLLECTOR_FILESYSTEM_RUN_STARTUP|scrutiny-collector-filesystem|optional"
)

# run_service executes one service script in a sandbox and prints "ran" when the
# collector binary was invoked, "skipped" otherwise.
#
# The s6 and container paths are absolute, so the sandbox copy redirects
# /opt/scrutiny/bin at a stub tree and /tmp at the sandbox. The control flow
# under test is the committed script's own.
run_service() {
    local service="$1" binary="$2" cron_var="$3" cron_value="$4" startup_var="$5" startup_set="$6" startup_value="$7"
    local sandbox stub script
    sandbox="$(mktemp -d "${test_root}/sandbox.XXXXXX")"
    stub="${sandbox}/bin"
    mkdir -p "$stub"

    for command in s6-svc s6-svwait curl; do
        printf '#!/usr/bin/env bash\nexit 0\n' > "${stub}/${command}"
        chmod +x "${stub}/${command}"
    done

    printf '#!/usr/bin/env bash\ntouch %s/collector-ran\n' "$sandbox" > "${stub}/${binary}"
    chmod +x "${stub}/${binary}"

    script="${sandbox}/run"
    sed -e "s|/opt/scrutiny/bin|${stub}|g" -e "s|/tmp/|${sandbox}/|g" \
        "${repo_root}/rootfs/etc/services.d/${service}/run" > "$script"

    local -a env_args=("PATH=${stub}:${PATH}" "${cron_var}=${cron_value}")
    if [[ "$startup_set" == "set" ]]; then
        env_args+=("${startup_var}=${startup_value}")
    fi

    env -u "$startup_var" "${env_args[@]}" bash "$script" >/dev/null 2>&1 || true

    if [[ -f "${sandbox}/collector-ran" ]]; then
        echo "ran"
    else
        echo "skipped"
    fi
}

assert_outcome() {
    local expected="$1" actual="$2" description="$3"
    [[ "$actual" == "$expected" ]] || fail "${description}: expected ${expected}, got ${actual}"
}

for descriptor in "${services[@]}"; do
    IFS='|' read -r service cron_var startup_var binary kind <<< "$descriptor"

    # The reported defect: a schedule plus an explicit false must not collect at startup.
    assert_outcome skipped \
        "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" set false)" \
        "${service} with ${startup_var}=false"

    # Mixed case must behave the same as lowercase.
    assert_outcome skipped \
        "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" set False)" \
        "${service} with ${startup_var}=False"

    # An unset variable keeps the long-standing startup run, so upgrades do not change.
    assert_outcome ran \
        "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" unset "")" \
        "${service} with ${startup_var} unset"

    # An explicit true always collects at startup.
    assert_outcome ran \
        "$(run_service "$service" "$binary" "$cron_var" "" "$startup_var" set true)" \
        "${service} with ${startup_var}=true"

    # The five optional collectors stay off when neither a schedule nor a startup flag is given.
    if [[ "$kind" == "optional" ]]; then
        assert_outcome skipped \
            "$(run_service "$service" "$binary" "$cron_var" "" "$startup_var" unset "")" \
            "${service} with no schedule and ${startup_var} unset"
    fi

    echo "ok: ${service}"
done

echo "PASS: all six omnibus run-once services honor an explicit *_RUN_STARTUP=false"
