#!/bin/bash
# Verifies that the collector entrypoints say so when they skip startup collection.
#
# A silent skip reads as a broken collector: the container starts, nothing is collected
# until the next cron tick, and the dashboard keeps showing the previous run's data, so
# an operator who just changed device mappings or capabilities concludes the change had
# no effect.

set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

failures=0

fail() {
    echo "FAIL: $1" >&2
    failures=$((failures + 1))
}

assert_contains() {
    local haystack="$1" needle="$2" label="$3"
    if [[ "$haystack" != *"$needle"* ]]; then
        fail "$label: expected to find '$needle' in: $haystack"
    fi
}

assert_not_contains() {
    local haystack="$1" needle="$2" label="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        fail "$label: did not expect to find '$needle' in: $haystack"
    fi
}

# These entrypoints emit logrus logfmt. An unescaped or escaped inner double quote
# terminates the msg field for a logfmt parser and loses every key after it, so the
# message must carry no double quote of its own.
assert_logfmt_parsable() {
    local line="$1" label="$2"
    local body="${line#*msg=\"}"
    body="${body%\"*}"
    if [[ "$body" == *'"'* ]]; then
        fail "$label: msg field contains a double quote, which breaks logfmt parsing: $line"
    fi
    assert_contains "$line" "level=info" "$label keeps the level field"
}

########################################
# Omnibus: run_startup_collector logs every branch
########################################

helper="$ROOT/docker/entrypoint-collector-omnibus-startup.sh"

# Extract the function definitions by name, so neither their order in the file nor an
# added sibling function changes what is loaded here.
{
    awk '/^log_info\(\) \{/,/^\}$/' "$helper"
    awk '/^run_startup_collector\(\) \{/,/^\}$/' "$helper"
} > "$WORK/helper.sh"

for fn in log_info run_startup_collector; do
    grep -q "^${fn}() {" "$WORK/helper.sh" || fail "helper extraction: $fn was not extracted"
    [[ $(grep -c "^${fn}() {" "$WORK/helper.sh") -eq 1 ]] \
        || fail "helper extraction: $fn was extracted more than once"
done
bash -n "$WORK/helper.sh" || fail "helper extraction: result is not valid bash"

cat > "$WORK/scrutiny-collector-metrics" <<'EOF'
#!/bin/bash
echo "collector ran with: $*"
EOF
chmod +x "$WORK/scrutiny-collector-metrics"

run_helper() {
    # shellcheck source=/dev/null
    source "$WORK/helper.sh"
    run_startup_collector "COLLECTOR_RUN_STARTUP" "COLLECTOR_RUN_STARTUP_SLEEP" \
        "$WORK/scrutiny-collector-metrics" "scrutiny metrics" "metrics" \
        "COLLECTOR_CRON_SCHEDULE" "${1:-}" 2>&1
}

# Startup disabled, cron schedule known: name the schedule, do not run the binary.
skipped=$(COLLECTOR_RUN_STARTUP=false run_helper "0 0 * * *")
assert_contains "$skipped" "startup collection is disabled" "omnibus skip"
assert_contains "$skipped" "COLLECTOR_RUN_STARTUP" "omnibus skip names the variable"
assert_contains "$skipped" "0 0 * * *" "omnibus skip names the cron schedule"
assert_contains "$skipped" "type=metrics" "omnibus skip keeps the component field"
assert_not_contains "$skipped" "collector ran with" "omnibus skip must not run the binary"
assert_logfmt_parsable "$skipped" "omnibus skip"

# Startup disabled and no cron schedule: 50-cron-config comments the cron line out, so
# this collector does not run at all. Saying it will run on cron would be a false promise.
no_cron=$(COLLECTOR_RUN_STARTUP=false run_helper "")
assert_contains "$no_cron" "will not run" "omnibus skip without a schedule"
assert_contains "$no_cron" "COLLECTOR_CRON_SCHEDULE" "omnibus skip without a schedule names the variable to set"
assert_not_contains "$no_cron" "will next run on cron schedule" "omnibus skip without a schedule must not promise a cron run"
assert_logfmt_parsable "$no_cron" "omnibus skip without a schedule"

# An operator-supplied schedule wins over the built-in default.
override=$(COLLECTOR_RUN_STARTUP=false COLLECTOR_CRON_SCHEDULE="55 * * * *" run_helper "0 0 * * *")
assert_contains "$override" "55 * * * *" "omnibus skip prefers the operator schedule"
assert_not_contains "$override" "0 0 * * *" "omnibus skip must not report the default over the operator value"

# Startup enabled: run the binary, and do not log a skip.
ran=$(COLLECTOR_RUN_STARTUP=true COLLECTOR_RUN_STARTUP_SLEEP=0 run_helper "0 0 * * *")
assert_contains "$ran" "run-once mode" "omnibus run"
assert_contains "$ran" "collector ran with: run --cron-schedule=" "omnibus run forces one collection"
assert_not_contains "$ran" "startup collection is disabled" "omnibus run must not log a skip"

# Exercise the SMART invocation from each Docker call site with the same stub.
# The CLI tests cover environment/YAML precedence. These checks catch a missing
# explicit override in a launcher without starting cron or accessing devices.
for relative in docker/entrypoint-collector.sh rootfs/etc/services.d/collector-once/run rootfs/etc/cron.d/scrutiny; do
    invocation=$(sed -n 's|.*\(/opt/scrutiny/bin/scrutiny-collector-metrics run[^>]*\).*|\1|p' "$ROOT/$relative")
    [[ -n "$invocation" ]] || { fail "$relative: missing SMART invocation"; continue; }
    invocation=${invocation/\/opt\/scrutiny\/bin\/scrutiny-collector-metrics/"$WORK\/scrutiny-collector-metrics"}
    output=$(bash -c "$invocation")
    assert_contains "$output" "collector ran with: run --cron-schedule=" "$relative forces one collection"
done

########################################
# Standalone entrypoints: every startup branch has a logged else
########################################
# These scripts rewrite /etc/cron.d and exec cron, so they are inspected rather than run.

for name in \
    entrypoint-collector.sh \
    entrypoint-collector-zfs.sh \
    entrypoint-collector-mdadm.sh \
    entrypoint-collector-btrfs.sh \
    entrypoint-collector-performance.sh
do
    path="$ROOT/docker/$name"
    [[ -f "$path" ]] || { fail "$name: file not found"; continue; }

    bash -n "$path" || fail "$name: bash syntax error"

    block=$(sed -n '/_RUN_STARTUP}" == "true" \]\]; then/,/^fi$/p' "$path")
    if [[ -z "$block" ]]; then
        fail "$name: could not find the RUN_STARTUP block"
        continue
    fi

    # A column-0 else is a sibling of the RUN_STARTUP if. A nested if/else would be
    # indented, so this distinguishes the branch under test from one inside the block.
    if ! grep -q '^else$' <<<"$block"; then
        fail "$name: the RUN_STARTUP if has no top-level else branch"
        continue
    fi

    skip_line=$(sed -n '/^else$/,/^fi$/p' <<<"$block" | grep 'startup collection is disabled' || true)
    if [[ -z "$skip_line" ]]; then
        fail "$name: the else branch does not log the skip"
        continue
    fi
    assert_contains "$skip_line" "_RUN_STARTUP is not set to true" "$name names the variable responsible"

    # The schedule it reports must be a variable this same script defaults, otherwise the
    # message would print an empty schedule and this check would still pass.
    schedule_var=$(grep -o '[A-Z_]*CRON_SCHEDULE' <<<"$skip_line" | head -1)
    if [[ -z "$schedule_var" ]]; then
        fail "$name: the skip message does not report a cron schedule"
        continue
    fi
    grep -q "^${schedule_var}=\${${schedule_var}:-" "$path" \
        || fail "$name: $schedule_var is reported but never defaulted in this script"

    # Same logfmt constraint as the omnibus: no double quote inside the message.
    msg=${skip_line#*log_info \"}
    msg=${msg%\"*}
    if [[ "$msg" == *'\"'* || "$msg" == *'"'* ]]; then
        fail "$name: the skip message embeds a double quote, which breaks logfmt parsing: $skip_line"
    fi
done

if [[ "$failures" -ne 0 ]]; then
    echo "$failures check(s) failed" >&2
    exit 1
fi

echo "all checks passed"
