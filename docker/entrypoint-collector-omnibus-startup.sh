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
    local schedule_var="$6"
    local schedule_default="$7"
    local startup="${!startup_var:-false}"
    local delay="${!sleep_var:-1}"
    # Mirrors the defaulting in /etc/cont-init.d/50-cron-config, which runs in its own
    # process and does not export these. Only an operator-supplied value reaches us
    # through with-contenv, so an empty result means the cron line was commented out
    # and this collector does not run at all.
    local schedule="${!schedule_var:-${schedule_default}}"

    if [ "${startup}" != "true" ]; then
        if [ -n "${schedule}" ]; then
            log_info "${component}" "startup collection is disabled (${startup_var} is not set to true); the ${label} collector will next run on cron schedule ${schedule}"
        else
            log_info "${component}" "startup collection is disabled (${startup_var} is not set to true) and no cron schedule is set, so the ${label} collector will not run; set ${schedule_var} to enable it"
        fi
        return
    fi

    log_info "${component}" "starting ${label} collector (run-once mode)"
    sleep "${delay}"
    "${binary}" run
}

run_startup_collector "COLLECTOR_RUN_STARTUP" "COLLECTOR_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-metrics" "scrutiny metrics" "metrics" \
    "COLLECTOR_CRON_SCHEDULE" "0 0 * * *"
run_startup_collector "COLLECTOR_ZFS_RUN_STARTUP" "COLLECTOR_ZFS_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-zfs" "scrutiny ZFS" "zfs" \
    "COLLECTOR_ZFS_CRON_SCHEDULE" ""
run_startup_collector "COLLECTOR_MDADM_RUN_STARTUP" "COLLECTOR_MDADM_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-mdadm" "scrutiny MDADM" "mdadm" \
    "COLLECTOR_MDADM_CRON_SCHEDULE" ""
run_startup_collector "COLLECTOR_BTRFS_RUN_STARTUP" "COLLECTOR_BTRFS_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-btrfs" "scrutiny Btrfs" "btrfs" \
    "COLLECTOR_BTRFS_CRON_SCHEDULE" ""
run_startup_collector "COLLECTOR_FILESYSTEM_RUN_STARTUP" "COLLECTOR_FILESYSTEM_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-filesystem" "scrutiny filesystem" "filesystem" \
    "COLLECTOR_FILESYSTEM_CRON_SCHEDULE" ""
run_startup_collector "COLLECTOR_PERF_RUN_STARTUP" "COLLECTOR_PERF_RUN_STARTUP_SLEEP" \
    "/opt/scrutiny/bin/scrutiny-collector-performance" "scrutiny performance" "performance" \
    "COLLECTOR_PERF_CRON_SCHEDULE" ""
