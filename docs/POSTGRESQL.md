# PostgreSQL Backend

Scrutiny keeps two kinds of data:

- **Time series** (SMART attributes, temperatures) in InfluxDB.
- **Relational data** (devices, settings, overrides, notification URLs, ZFS, mdadm, and Btrfs metadata) in SQLite by default.

The relational data can live in PostgreSQL instead. InfluxDB is still required either way.

## When to use PostgreSQL

SQLite is the right choice for a single Scrutiny instance with its database on local disk. It needs no setup.

Use PostgreSQL when:

- You want to run more than one Scrutiny web instance for high availability.
- Your other services already use a managed PostgreSQL server, and you want one backup and operations story.
- The Scrutiny config directory is on network storage (NFS or SMB). SQLite file locking is unreliable there. Moving the SQLite file to local disk also fixes this.

Only PostgreSQL is supported. MariaDB and MySQL are not.

## Configuration

| Key | Environment variable | Default | Description |
| --- | --- | --- | --- |
| `web.database.type` | `SCRUTINY_WEB_DATABASE_TYPE` | `sqlite` | `sqlite` or `postgres` |
| `web.database.dsn` | `SCRUTINY_WEB_DATABASE_DSN` | empty | PostgreSQL connection string. Required when the type is `postgres`. |
| `web.database.max_open_conns` | `SCRUTINY_WEB_DATABASE_MAX_OPEN_CONNS` | `10` | Maximum open connections per Scrutiny process. PostgreSQL only. |
| `web.database.max_idle_conns` | `SCRUTINY_WEB_DATABASE_MAX_IDLE_CONNS` | `5` | Maximum idle connections per Scrutiny process. PostgreSQL only. |

`web.database.location` and `web.database.journal_mode` apply to SQLite only.

The DSN is a standard PostgreSQL URL:

```
postgres://scrutiny:PASSWORD@db.example.com:5432/scrutiny?sslmode=require
```

The DSN contains the database password. Scrutiny removes it from its debug log. Keep it out of files that other people can read. An environment variable or a secret file works well.

Scrutiny opens several connections per process, one pool for each background job. Size `max_connections` on the server, or the pool in pgbouncer, for `max_open_conns` times the number of pools times the number of replicas. The defaults are enough for one or two replicas.

### Example: Docker Compose

```yaml
services:
  postgres:
    image: postgres:16
    restart: unless-stopped
    environment:
      POSTGRES_USER: scrutiny
      POSTGRES_PASSWORD: change-me
      POSTGRES_DB: scrutiny
    volumes:
      - './postgres:/var/lib/postgresql/data'
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U scrutiny -d scrutiny']
      interval: 5s
      timeout: 5s
      retries: 10

  web:
    image: 'ghcr.io/starosdev/scrutiny:latest-web'
    environment:
      SCRUTINY_WEB_INFLUXDB_HOST: 'influxdb'
      SCRUTINY_WEB_DATABASE_TYPE: 'postgres'
      SCRUTINY_WEB_DATABASE_DSN: 'postgres://scrutiny:change-me@postgres:5432/scrutiny?sslmode=disable'
    depends_on:
      postgres:
        condition: service_healthy
      influxdb:
        condition: service_healthy
```

The rest of the file (InfluxDB and the collectors) is the same as [example.hubspoke.docker-compose.yml](../docker/example.hubspoke.docker-compose.yml).

### pgbouncer

Scrutiny works behind pgbouncer in session and transaction pooling modes. It uses the simple query protocol and does not prepare statements, and its migration lock lasts only for one transaction. CI runs the database tests through pgbouncer in transaction mode with a single server connection. That is the strictest setup.

## First start on a new database

Create an empty database and a user that owns it. On first start, Scrutiny creates the tables and the default settings. No manual schema step is needed.

If several replicas start at the same time, one of them sets up the database and the others wait for it. An advisory lock serializes the migrations.

## Moving an existing install from SQLite

1. Stop Scrutiny. The import must see a database that nothing else is writing.
2. Make sure the SQLite database has been run by the Scrutiny version you are importing with. If the version is newer, start it once on SQLite, then stop it.
3. Create a new, empty PostgreSQL database.
4. Set `web.database.type: postgres` and `web.database.dsn`.
5. Run the import:

   ```
   scrutiny db import-sqlite --from /opt/scrutiny/config/scrutiny.db
   ```

   Add `--config /path/to/scrutiny.yaml` if the config file is not in the default location. In Docker, run it in the web container:

   ```
   docker compose run --rm web /opt/scrutiny/bin/scrutiny db import-sqlite --from /opt/scrutiny/config/scrutiny.db
   ```

6. Start Scrutiny. It now uses PostgreSQL.

The import prints the rows it copied for each table. It opens the SQLite file read-only and does not change it, so keep the file as a backup. The import runs in one transaction: if it fails, PostgreSQL is left empty and you can run it again.

The import refuses to run when:

- the SQLite database is missing a migration that this version knows, or
- the PostgreSQL database already holds data.

InfluxDB is not copied. Point the PostgreSQL install at the same InfluxDB.

## Running more than one replica

With PostgreSQL, several Scrutiny web instances can serve the same data behind a load balancer. All replicas accept collector uploads and API requests. They share the work like this:

- **Background jobs run on one replica.** One replica holds a leader lease in the database and is the only one that runs the missed-ping, heartbeat, and Uptime Kuma monitors and the report scheduler. If that replica stops, another one takes over within about 30 seconds.
- **Notifications are sent by the leader.** A replica that receives an upload records the notification in the database. The leader sends it, so rate limits, quiet hours, and duplicate suppression apply once for all replicas. Notifications that wait more than one hour for a leader are discarded, and the discard is logged.
- **Each scheduled report is sent once**, even during a leader change.

Known limits:

- **InfluxDB is still a single instance.** Scrutiny does not replicate it. If it is down, uploads and charts fail on every replica.
- **Temperature alerts are decided on the replica that receives the upload.** The "hot for N minutes" timer lives in that replica's memory. If uploads from one device go to different replicas, the alert can be late. If the leader later drops that alert (for example, for the rate limit), it is not retried.
- **Prometheus metrics are per replica.** Each replica reports the uploads it received. Scrape every replica, or route collectors to one.
- **MQTT is published by every replica** that receives an upload. The messages carry the same state, so Home Assistant sees no difference.
- **Replica clocks must agree** to well under 30 seconds. The leader lease uses each replica's own clock. Run NTP.
- **During a rolling upgrade**, replicas that still run a version older than the leader lease send notifications themselves. Expect a few duplicate alerts during the upgrade.

## Backups

Back up both stores:

- PostgreSQL, for example with `pg_dump`.
- InfluxDB, with `influx backup`, or by copying its data directory while InfluxDB is stopped.

## Writing migrations (contributors)

Migrations added after PostgreSQL support must run on both SQLite and PostgreSQL. Use `tx.Migrator()` and `tx.AutoMigrate` or portable SQL. Do not use SQLite-only features such as `rowid`, `AUTOINCREMENT`, or table rebuilds.

A new PostgreSQL database does not replay the old migrations. It builds its schema from `schemaModels` in `webapp/backend/pkg/database/schema_models.go`. A migration that adds a table must also add its model there. `TestSchemaModelsMatchMigrationHistory` fails if the two drift.

The PostgreSQL tests run when `SCRUTINY_TEST_POSTGRES_DSN` is set to a `postgres://` URL, and the pgbouncer test runs when `SCRUTINY_TEST_PGBOUNCER_DSN` is set. `scripts/tests/start-pgbouncer.sh` starts a matching pgbouncer.
