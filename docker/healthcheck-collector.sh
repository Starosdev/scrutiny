#!/bin/sh
# HEALTHCHECK for the collector image. Healthy means:
#   1. the cron daemon is running, and
#   2. when COLLECTOR_HEALTHCHECK_MAX_AGE_HOURS is set, the last successful collection (or, before the
#      first one, the container start) is newer than that many hours.
#
# The age check is off by default because the schedule is user-defined. Set the limit to about twice the
# interval of COLLECTOR_CRON_SCHEDULE, for example 48 for the default daily schedule.
#
# The cron job and the run-on-startup path touch LAST_RUN after a collection exits 0.
LAST_RUN=/tmp/scrutiny-collector-last-run
STARTED=/tmp/scrutiny-collector-started

cron_running=false
for comm in /proc/[0-9]*/comm; do
    if [ "$(cat "$comm" 2>/dev/null)" = "cron" ]; then
        cron_running=true
        break
    fi
done
if [ "$cron_running" != "true" ]; then
    echo "cron is not running"
    exit 1
fi

max_age_hours="${COLLECTOR_HEALTHCHECK_MAX_AGE_HOURS:-}"
if [ -z "$max_age_hours" ]; then
    exit 0
fi
case "$max_age_hours" in
    *[!0-9]*)
        echo "COLLECTOR_HEALTHCHECK_MAX_AGE_HOURS must be a whole number"
        exit 1
        ;;
esac

reference="$LAST_RUN"
[ -f "$reference" ] || reference="$STARTED"
if [ ! -f "$reference" ]; then
    echo "no run-state file found"
    exit 1
fi
if [ -z "$(find "$reference" -mmin "-$((max_age_hours * 60))" 2>/dev/null)" ]; then
    echo "last successful collection is older than ${max_age_hours}h"
    exit 1
fi
exit 0
