#!/bin/bash

# Cron runs in its own isolated environment (usually using only /etc/environment )
# So when the container starts up, we will do a dump of the runtime environment into a .env file that we
# will then source into the crontab file (/etc/cron.d/scrutiny-mdadm)
(set -o posix; export -p) > /env.sh

# adding ability to customize the cron schedule.
COLLECTOR_MDADM_CRON_SCHEDULE=${COLLECTOR_MDADM_CRON_SCHEDULE:-"*/15 * * * *"}
COLLECTOR_MDADM_RUN_STARTUP=${COLLECTOR_MDADM_RUN_STARTUP:-"false"}
COLLECTOR_MDADM_RUN_STARTUP_SLEEP=${COLLECTOR_MDADM_RUN_STARTUP_SLEEP:-"1"}

# if the cron schedule has been overridden via env variable (eg docker-compose) we should make sure to strip quotes
[[ "${COLLECTOR_MDADM_CRON_SCHEDULE}" == \"*\" || "${COLLECTOR_MDADM_CRON_SCHEDULE}" == \'*\' ]] && COLLECTOR_MDADM_CRON_SCHEDULE="${COLLECTOR_MDADM_CRON_SCHEDULE:1:-1}"

# replace placeholder with correct value
sed -i 's|{COLLECTOR_MDADM_CRON_SCHEDULE}|'"${COLLECTOR_MDADM_CRON_SCHEDULE}"'|g' /etc/cron.d/scrutiny-mdadm

if [[ "${COLLECTOR_MDADM_RUN_STARTUP}" == "true" ]]; then
    sleep ${COLLECTOR_MDADM_RUN_STARTUP_SLEEP}
    echo "starting scrutiny MDADM collector (run-once mode. subsequent calls will be triggered via cron service)"
    COLLECTOR_CRON_SCHEDULE= COLLECTOR_MDADM_RUN_STARTUP= /opt/scrutiny/bin/scrutiny-collector-mdadm run
fi


# now that we have the env start cron in the foreground
echo "starting cron"
exec su -c "cron -f -L 15" root
