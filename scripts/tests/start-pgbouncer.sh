#!/usr/bin/env bash
# Starts pgbouncer in transaction pooling mode in front of a PostgreSQL server, for the
# PostgreSQL backend tests (#880). Transaction pooling is the mode that breaks prepared
# statements and session-level locks, so it is the one worth testing.
#
# Usage: start-pgbouncer.sh WORKDIR LISTEN_PORT PG_HOST PG_PORT DATABASE USER PASSWORD
#
# The client connects to 127.0.0.1:LISTEN_PORT/DATABASE as USER with PASSWORD. pgbouncer
# keeps the password in plain text in WORKDIR so it can authenticate to the server with SCRAM.
set -euo pipefail

if [ "$#" -ne 7 ]; then
  echo "usage: $0 WORKDIR LISTEN_PORT PG_HOST PG_PORT DATABASE USER PASSWORD" >&2
  exit 2
fi
workdir=$1 listen_port=$2 pg_host=$3 pg_port=$4 database=$5 user=$6 password=$7

mkdir -p "$workdir"
printf '"%s" "%s"\n' "$user" "$password" > "$workdir/userlist.txt"
cat > "$workdir/pgbouncer.ini" <<EOF
[databases]
$database = host=$pg_host port=$pg_port dbname=$database

[pgbouncer]
listen_addr = 127.0.0.1
listen_port = $listen_port
unix_socket_dir =
auth_type = scram-sha-256
auth_file = $workdir/userlist.txt
pool_mode = transaction
# 0 turns off pgbouncer's own prepared statement support (1.21+), so the test holds for
# older pgbouncer versions too.
max_prepared_statements = 0
# One server connection makes every client share it, so a client-side prepared statement
# collides with or goes missing for the next client, as it does on a busy real pool.
default_pool_size = 1
max_client_conn = 100
ignore_startup_parameters = extra_float_digits
logfile = $workdir/pgbouncer.log
pidfile = $workdir/pgbouncer.pid
EOF

pgbouncer -d "$workdir/pgbouncer.ini"

for _ in $(seq 1 30); do
  if (exec 3<>"/dev/tcp/127.0.0.1/$listen_port") 2>/dev/null; then
    echo "pgbouncer listening on 127.0.0.1:$listen_port (transaction pooling)"
    exit 0
  fi
  sleep 1
done
echo "pgbouncer did not start; log follows" >&2
cat "$workdir/pgbouncer.log" >&2 || true
exit 1
