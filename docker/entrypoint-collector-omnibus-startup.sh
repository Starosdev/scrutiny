#!/command/with-contenv bash

log_info() {
    local component="$1"
    local message="$2"
    printf 'time="%s" level=info msg="%s" type=%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$message" "$component"
}

run_startup_collector() {
    local startup_var="$1"
    local sleep_var="$2"
    local binary="$3"
    local label="$4"
    local component="$5"
    local startup="${!startup_var:-false}"
    local delay="${!sleep_var:-1}"

    if [ "${startup}" != "true" ]; then
        return
    fi

    log_info "${component}" "starting ${label} collector (run-once mode)"
    sleep "${delay}"
    "${binary}" run
}

run_startup_collector "COLLECTOR_RUN_STARTUP" "COLLECTOR_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-metrics" "scrutiny metrics" "metrics"
run_startup_collector "COLLECTOR_ZFS_RUN_STARTUP" "COLLECTOR_ZFS_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-zfs" "scrutiny ZFS" "zfs"
run_startup_collector "COLLECTOR_MDADM_RUN_STARTUP" "COLLECTOR_MDADM_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-mdadm" "scrutiny MDADM" "mdadm"
run_startup_collector "COLLECTOR_BTRFS_RUN_STARTUP" "COLLECTOR_BTRFS_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-btrfs" "scrutiny Btrfs" "btrfs"
run_startup_collector "COLLECTOR_FILESYSTEM_RUN_STARTUP" "COLLECTOR_FILESYSTEM_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-filesystem" "scrutiny filesystem" "filesystem"
run_startup_collector "COLLECTOR_PERF_RUN_STARTUP" "COLLECTOR_PERF_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-performance" "scrutiny performance" "performance"
