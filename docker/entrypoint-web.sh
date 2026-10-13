#!/bin/sh
# Entrypoint of the web image (hub/spoke). The server does not need root, so when the container starts
# as root this script fixes ownership of the writable directories and then drops to PUID:PGID
# (default 1000:1000) before running the command.
#
# Starting as root keeps upgrades working: volumes created by earlier releases are owned by root, and
# the chown below hands them to the runtime user. When the container already starts as a non-root
# user (docker --user, compose user:, Kubernetes runAsUser), the script runs the command unchanged and
# the mounted directories must already be writable by that user.
set -eu

log() {
    printf 'time="%s" level=%s msg="%s" type=entrypoint\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$1" "$2"
}

if [ "$(id -u)" != "0" ]; then
    exec "$@"
fi

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

case "$PUID$PGID" in
    *[!0-9]*)
        log error "PUID and PGID must be numbers"
        exit 1
        ;;
esac

if [ "$PUID" = "0" ]; then
    log warning "PUID=0: running as root. Remove PUID to run as an unprivileged user"
    exec "$@"
fi

# /opt/scrutiny/config holds the SQLite database and the generated secrets; /opt/scrutiny/reports
# holds PDF reports (web.reports.path).
for dir in /opt/scrutiny/config /opt/scrutiny/reports; do
    mkdir -p "$dir"
    if ! find "$dir" \( ! -user "$PUID" -o ! -group "$PGID" \) -exec chown "$PUID:$PGID" {} + 2>/dev/null; then
        log warning "could not change ownership of $dir to $PUID:$PGID. Make it writable by that user, or set PUID and PGID to its owner"
    fi
done

# Dropping privileges needs CAP_SETUID and CAP_SETGID. With cap_drop: [ALL] it fails, so probe first
# and fall back to the current user rather than refusing to start.
if ! setpriv --reuid="$PUID" --regid="$PGID" --clear-groups true 2>/dev/null; then
    log warning "cannot drop privileges (missing CAP_SETUID or CAP_SETGID): running as root"
    exec "$@"
fi

export HOME=/tmp
exec setpriv --reuid="$PUID" --regid="$PGID" --clear-groups "$@"
