#!/usr/bin/env bash

# Run on a Linux Docker host with AppArmor and passwordless sudo.
# Usage: bash docker/tests/apparmor_test.sh IMAGE omnibus|collector
# Images must contain the current source. No host devices or data are mounted.
set -euo pipefail

image="${1:?image required}"
kind="${2:?omnibus or collector required}"
[[ "$kind" == omnibus || "$kind" == collector ]] || exit 2
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
[[ "$(uname -s)" == Linux ]] || { echo 'AppArmor requires Linux' >&2; exit 1; }
docker info --format '{{json .SecurityOptions}}' | grep -q 'name=apparmor' || {
    echo 'Docker must have AppArmor enabled' >&2
    exit 1
}

test_root="$(mktemp -d)"
profile="scrutiny-test-${test_root##*.}"
container="$profile"
loaded=false
cleanup() {
    local status=$?
    if (( status != 0 )); then
        docker logs "$container" >&2 || true
    fi
    docker rm -f "$container" >/dev/null 2>&1 || true
    if [[ "$loaded" == true ]] && ! sudo -n apparmor_parser -R "$test_root/profile"; then
        echo "Could not unload $profile; retained $test_root/profile for cleanup" >&2
        exit 1
    fi
    rm -rf "$test_root"
    exit "$status"
}
trap cleanup EXIT

sed -e "s/profile scrutiny-collector /profile $profile /" \
    -e "s/peer=scrutiny-collector,/peer=$profile,/" \
    "$repo_root/docker/apparmor-profile" > "$test_root/profile"
# Temporary profiles must not read or overwrite the host profile cache.
sudo -n apparmor_parser -r -K "$test_root/profile"
loaded=true

# Positive controls use only a regular file and a private tmpfs in a disposable
# container. A later denial must not be caused by missing Linux capabilities.
docker run --rm --cap-add SYS_ADMIN --security-opt apparmor=unconfined \
    --entrypoint /bin/bash "$image" -ec '
        : > /dev/sdz
        mkdir /tmp/mount-check
        mount -t tmpfs tmpfs /tmp/mount-check
        umount /tmp/mount-check
    '

docker run -d --name "$container" --memory 2g --cpus 2 --pids-limit 256 \
    --cap-add SYS_RAWIO --cap-add SYS_ADMIN \
    --security-opt "apparmor=$profile" \
    -e TZ=Etc/UTC "$image" >/dev/null

ready=false
for (( attempt=0; attempt<120; attempt++ )); do
    if docker exec "$container" /bin/bash -ec '
        for file in /proc/[0-9]*/comm; do
            if read -r name < "$file" && [[ "$name" == cron ]]; then exit 0; fi
        done
        exit 1
    ' 2>/dev/null; then
        if [[ "$kind" == collector ]] || docker exec "$container" \
            curl -fsS http://localhost:8080/api/health >/dev/null 2>&1; then
            ready=true
            break
        fi
    fi
    sleep 1
done
[[ "$ready" == true ]] || { echo 'Startup did not become ready' >&2; exit 1; }

# Prove the processes inherited the enforcing profile, then exercise the real
# shipped executables without passing through disks or starting benchmarks.
docker exec "$container" /bin/bash -ec '
    current=$(cat /proc/self/attr/current)
    [[ "$current" == "$1 (enforce)" ]]
    for binary in /opt/scrutiny/bin/scrutiny-collector-*; do
        "$binary" --version
    done
    smartctl --scan --json
    if [[ -x /opt/scrutiny/bin/apprise ]]; then /opt/scrutiny/bin/apprise --version; fi
    : > /tmp/apparmor-write-control
    cp /bin/true /tmp/scrutiny-unapproved-executable
    if /tmp/scrutiny-unapproved-executable; then
        echo "Execution outside the runtime paths was allowed" >&2
        exit 1
    fi
    if : > /dev/sdz; then echo "Device-file write was allowed" >&2; exit 1; fi
    mkdir /tmp/mount-check
    if mount -t tmpfs tmpfs /tmp/mount-check; then
        umount /tmp/mount-check
        echo "Mount was allowed" >&2
        exit 1
    fi
' -- "$profile"

# s6 containers must shut down without Docker resorting to SIGKILL.
docker stop --time 20 "$container" >/dev/null
[[ "$(docker inspect --format '{{.State.ExitCode}}' "$container")" != 137 ]] || {
    echo 'Container needed SIGKILL to stop' >&2
    exit 1
}

# Ignore only the deliberate /tmp execution probe; other exec denials fail.
# The output file is owned by the test user; only dmesg needs sudo.
# shellcheck disable=SC2024
sudo -n dmesg > "$test_root/kernel.log"
if grep -F "profile=\"$profile\"" "$test_root/kernel.log" |
    grep -Fv 'name="/tmp/scrutiny-unapproved-executable"' | grep -q 'operation="exec"'; then
    grep -F "profile=\"$profile\"" "$test_root/kernel.log" >&2
    echo 'Unexpected AppArmor execution denial' >&2
    exit 1
fi
echo "AppArmor startup, executable, confinement and shutdown checks passed: $image"
