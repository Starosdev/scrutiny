#!/usr/bin/env bash
# Verifies that the omnibus run-once services honor an explicit *_RUN_STARTUP=false
# while still running when the variable is unset. See issue #1009.

set -euo pipefail

# The services under test use ${VAR//...} and local, so an old /bin/bash would
# fail at runtime and look like a deliberate skip. Say so instead.
(( BASH_VERSINFO[0] >= 4 )) || {
    echo "FAIL: this test needs bash 4 or newer, found ${BASH_VERSION}" >&2
    exit 1
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

# service directory | cron schedule variable | startup variable | collector binary | gating kind
services=(
    "collector-once|COLLECTOR_CRON_SCHEDULE|COLLECTOR_RUN_STARTUP|scrutiny-collector-metrics|always"
    "collector-performance-once|COLLECTOR_PERF_CRON_SCHEDULE|COLLECTOR_PERF_RUN_STARTUP|scrutiny-collector-performance|optional"
    "collector-zfs-once|COLLECTOR_ZFS_CRON_SCHEDULE|COLLECTOR_ZFS_RUN_STARTUP|scrutiny-collector-zfs|optional"
    "collector-mdadm-once|COLLECTOR_MDADM_CRON_SCHEDULE|COLLECTOR_MDADM_RUN_STARTUP|scrutiny-collector-mdadm|optional"
    "collector-btrfs-once|COLLECTOR_BTRFS_CRON_SCHEDULE|COLLECTOR_BTRFS_RUN_STARTUP|scrutiny-collector-btrfs|optional"
    "collector-filesystem-once|COLLECTOR_FILESYSTEM_CRON_SCHEDULE|COLLECTOR_FILESYSTEM_RUN_STARTUP|scrutiny-collector-filesystem|optional"
)

# run_service executes one service script in a sandbox and prints the outcome:
#
#   ran            the collector binary was invoked
#   skipped:gate   the new *_RUN_STARTUP gate declined the startup run
#   skipped:other  the script exited through some other branch
#
# Distinguishing the two skips matters. A broken sandbox, a stale bash, or a
# syntax error all produce "no collector ran", so matching the gate's own log
# line is what keeps a skip assertion from passing for the wrong reason.
run_service() {
    local service="$1" binary="$2" cron_var="$3" cron_value="$4" startup_var="$5" startup_set="$6" startup_value="$7"
    local marker_present="${8:-no}"
    local sandbox stub script output status

    sandbox="$(mktemp -d "${test_root}/sandbox.XXXXXX")"
    stub="${sandbox}/bin"
    mkdir -p "$stub"

    for command in s6-svc s6-svwait curl; do
        printf '#!/usr/bin/env bash\nexit 0\n' > "${stub}/${command}"
        chmod +x "${stub}/${command}"
    done

    printf '#!/usr/bin/env bash\nprintf "%%s\\n" "$@" > %s/collector-ran\n' "$sandbox" > "${stub}/${binary}"
    chmod +x "${stub}/${binary}"

    # The s6 and container paths are absolute, so the sandbox copy redirects
    # /opt/scrutiny/bin at the stub tree, /etc/scrutiny at the committed helper,
    # and /tmp at the sandbox. The /tmp rule must run FIRST: the stub path is
    # itself under /tmp on Linux, so rewriting /tmp afterwards would corrupt the
    # binary path this rule just inserted. The control flow under test stays the
    # committed script's own.
    script="${sandbox}/run"
    sed -e "s|/tmp/|${sandbox}/|g" \
        -e "s|/etc/scrutiny|${repo_root}/rootfs/etc/scrutiny|g" \
        -e "s|/opt/scrutiny/bin|${stub}|g" \
        "${repo_root}/rootfs/etc/services.d/${service}/run" > "$script"

    grep -q "${stub}/${binary}" "$script" \
        || fail "sandbox rewrite lost the collector path for ${service}"

    if [[ "$marker_present" == "yes" ]]; then
        # Mirrors a container restart that kept its writable layer.
        local marker
        marker="$(grep -o -- '-f /tmp/[a-z-]*' "${repo_root}/rootfs/etc/services.d/${service}/run" \
            | head -1 | sed 's|-f /tmp/||')"
        [[ -n "$marker" ]] || fail "could not locate the marker path for ${service}"
        touch "${sandbox}/${marker}"
    fi

    local -a env_args=("PATH=${stub}:${PATH}" "${cron_var}=${cron_value}")
    if [[ "$startup_set" == "set" ]]; then
        env_args+=("${startup_var}=${startup_value}")
        set +e
        output="$(env "${env_args[@]}" bash "$script" 2>&1)"
        status=$?
        set -e
    else
        set +e
        output="$(env -u "$startup_var" "${env_args[@]}" bash "$script" 2>&1)"
        status=$?
        set -e
    fi

    (( status == 0 )) || fail "${service} exited ${status}, output: ${output}"

    if [[ -f "${sandbox}/collector-ran" ]]; then
        local expected=$'run'
        if [[ "$binary" == "scrutiny-collector-metrics" ]]; then
            expected+=$'\n--cron-schedule='
        fi
        [[ "$(cat "${sandbox}/collector-ran")" == "$expected" ]] \
            || fail "${service} used incorrect one-shot arguments"
        echo "ran"
    elif grep -q "skipping startup" <<< "$output"; then
        echo "skipped:gate"
    else
        echo "skipped:other"
    fi
}

assert_outcome() {
    local expected="$1" actual="$2" description="$3"
    [[ "$actual" == "$expected" ]] || fail "${description}: expected ${expected}, got ${actual}"
}

for descriptor in "${services[@]}"; do
    IFS='|' read -r service cron_var startup_var binary kind <<< "$descriptor"

    # The reported defect: a schedule plus an explicit false must not collect at startup.
    assert_outcome skipped:gate \
        "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" set false)" \
        "${service} with ${startup_var}=false"

    # Every spelling strconv.ParseBool treats as false, plus the quotes and
    # whitespace that docker --env-file preserves.
    for spelling in False FALSE f F 0 '"false"' "'false'" ' false '; do
        assert_outcome skipped:gate \
            "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" set "$spelling")" \
            "${service} with ${startup_var}=${spelling}"
    done

    # An unset variable keeps the long-standing startup run, so upgrades do not change.
    assert_outcome ran \
        "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" unset "")" \
        "${service} with ${startup_var} unset"

    # An empty value is not an explicit false either.
    assert_outcome ran \
        "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" set "")" \
        "${service} with ${startup_var} empty"

    # Explicit true spellings always collect at startup.
    for spelling in true True TRUE t 1; do
        assert_outcome ran \
            "$(run_service "$service" "$binary" "$cron_var" "" "$startup_var" set "$spelling")" \
            "${service} with ${startup_var}=${spelling}"
    done

    # The gate sits ahead of the marker check, so a surviving marker still skips
    # without reaching the collector. This is the restart case from #1009.
    assert_outcome skipped:other \
        "$(run_service "$service" "$binary" "$cron_var" "0 0 * * *" "$startup_var" unset "" yes)" \
        "${service} with its init marker already present"

    # The five optional collectors stay off when neither a schedule nor a startup flag is given.
    if [[ "$kind" == "optional" ]]; then
        assert_outcome skipped:other \
            "$(run_service "$service" "$binary" "$cron_var" "" "$startup_var" unset "")" \
            "${service} with no schedule and ${startup_var} unset"
    fi

    echo "ok: ${service}"
done

echo "PASS: all six omnibus run-once services honor an explicit *_RUN_STARTUP=false"
