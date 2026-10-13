#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_suffix="${BASHPID:-$$}"
omnibus_image="scrutiny-frontend-permissions-omnibus:${test_suffix}"
web_image="scrutiny-frontend-permissions-web:${test_suffix}"
container_name="scrutiny-frontend-permissions-${test_suffix}"
config_volume="scrutiny-frontend-permissions-config-${test_suffix}"
host_port="${TEST_HOST_PORT:-18080}"

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

cleanup() {
    docker rm --force "$container_name" >/dev/null 2>&1 || true
    docker volume rm --force "$config_volume" >/dev/null 2>&1 || true
    docker image rm --force "$omnibus_image" "$web_image" >/dev/null 2>&1 || true
}

trap cleanup EXIT

build_image() {
    local dockerfile="$1"
    local image="$2"

    docker build --file "$repo_root/$dockerfile" --tag "$image" "$repo_root"
}

assert_searchable_directory() {
    local image="$1"
    local path="$2"
    local mode

    mode="$(docker run --rm --entrypoint stat "$image" -c '%a' "$path")"
    (( 10#$mode & 111 )) || fail "$path is not searchable in $image (mode $mode)"
}

assert_readable_file() {
    local image="$1"
    local path="$2"
    local mode

    mode="$(docker run --rm --entrypoint stat "$image" -c '%a' "$path")"
    (( 10#$mode & 444 )) || fail "$path is not readable in $image (mode $mode)"
}

wait_for_health() {
    local attempt

    for attempt in {1..60}; do
        if curl --silent --show-error --fail --head "http://127.0.0.1:${host_port}/api/health" >/dev/null; then
            return
        fi
        sleep 1
    done

    docker logs "$container_name" >&2 || true
    fail "omnibus API did not become healthy"
}

build_image docker/Dockerfile "$omnibus_image"
build_image docker/Dockerfile.web "$web_image"

for image in "$omnibus_image" "$web_image"; do
    assert_searchable_directory "$image" /opt/scrutiny/web
    assert_searchable_directory "$image" /opt/scrutiny/docs
    assert_readable_file "$image" /opt/scrutiny/web/index.html
done

# The web image starts as root only to fix volume ownership, then runs the server as 1000:1000. A
# config volume left root-owned by an earlier release must become writable without a manual chown.
web_uid="$(docker run --rm "$web_image" id -u)"
[[ "$web_uid" == "1000" ]] || fail "web image runs as uid $web_uid, expected 1000"

docker volume create "$config_volume" >/dev/null
docker run --rm --volume "${config_volume}:/data" --entrypoint sh "$web_image" \
    -c 'chown 0:0 /data && touch /data/scrutiny.db && chown 0:0 /data/scrutiny.db'
migrated_owner="$(docker run --rm --volume "${config_volume}:/opt/scrutiny/config" "$web_image" \
    sh -c 'touch /opt/scrutiny/config/.write-check && stat -c %u /opt/scrutiny/config/scrutiny.db')"
[[ "$migrated_owner" == "1000" ]] || fail "root-owned config volume was not handed to uid 1000 (owner $migrated_owner)"

docker run --detach \
    --name "$container_name" \
    --cap-drop ALL \
    --cap-add SYS_RAWIO \
    --publish "${host_port}:8080" \
    "$omnibus_image" >/dev/null

wait_for_health

dashboard="$(curl --silent --show-error --fail "http://127.0.0.1:${host_port}/web/")"
grep -q '<app-root' <<< "$dashboard" || fail "/web/ did not serve frontend index"
grep -q 'Index of' <<< "$dashboard" && fail "/web/ served directory listing"

spa_route="$(curl --silent --show-error --fail "http://127.0.0.1:${host_port}/web/dashboard")"
grep -q '<app-root' <<< "$spa_route" || fail "/web/dashboard did not serve frontend index"

curl --silent --show-error --fail --head "http://127.0.0.1:${host_port}/api/health" >/dev/null

echo "Docker frontend permission tests passed"
