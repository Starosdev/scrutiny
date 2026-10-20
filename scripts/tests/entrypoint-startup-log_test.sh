#!/bin/bash
# Verifies that the collector entrypoints say so when startup collection is skipped.
# A silent skip reads as a broken collector: the container starts, nothing is collected
# until the next cron tick, and the dashboard keeps showing the previous run's data.

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

########################################
# Omnibus: run_startup_collector logs both branches
########################################

helper="$ROOT/docker/entrypoint-collector-omnibus-startup.sh"

# Load only the function definitions, not the six invocations at the bottom of the file.
sed -n '1,/^}$/p;/^run_startup_collector() {/,/^}$/p' "$helper" \
    | grep -v '^#!' > "$WORK/helper.sh"

cat > "$WORK/fake-collector" <<'EOF'
#!/bin/bash
echo "collector ran with: $*"
EOF
chmod +x "$WORK/fake-collector"

# shellcheck source=/dev/null
skipped=$(
    source "$WORK/helper.sh"
    COLLECTOR_RUN_STARTUP=false \
        run_startup_collector "COLLECTOR_RUN_STARTUP" "COLLECTOR_RUN_STARTUP_SLEEP" \
        "$WORK/fake-collector" "scrutiny metrics" "metrics" 2>&1
)
assert_contains "$skipped" "startup collection is disabled" "omnibus skip"
assert_contains "$skipped" "COLLECTOR_RUN_STARTUP" "omnibus skip names the variable"
assert_contains "$skipped" "type=metrics" "omnibus skip keeps the component field"
if [[ "$skipped" == *"collector ran with"* ]]; then
    fail "omnibus skip: the collector binary ran even though startup was disabled"
fi

# shellcheck source=/dev/null
ran=$(
    source "$WORK/helper.sh"
    COLLECTOR_RUN_STARTUP=true COLLECTOR_RUN_STARTUP_SLEEP=0 \
        run_startup_collector "COLLECTOR_RUN_STARTUP" "COLLECTOR_RUN_STARTUP_SLEEP" \
        "$WORK/fake-collector" "scrutiny metrics" "metrics" 2>&1
)
assert_contains "$ran" "run-once mode" "omnibus run"
assert_contains "$ran" "collector ran with" "omnibus run invokes the binary"
if [[ "$ran" == *"startup collection is disabled"* ]]; then
    fail "omnibus run: logged the skip message while actually running"
fi

########################################
# Standalone entrypoints: every startup branch has a logged else
########################################
# These scripts rewrite /etc/cron.d and exec cron, so they are checked statically.

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
    assert_contains "$block" "else" "$name has an else branch"
    assert_contains "$block" "startup collection is disabled" "$name logs the skip"
    assert_contains "$block" "CRON_SCHEDULE}" "$name names its cron schedule"
done

if [[ "$failures" -ne 0 ]]; then
    echo "$failures check(s) failed" >&2
    exit 1
fi

echo "all checks passed"
