<p align="center">
  <a href="https://github.com/Starosdev/scrutiny">
  <img width="300" alt="scrutiny_view" src="webapp/frontend/src/assets/images/logo/scrutiny-logo-dark.png">
  </a>
</p>


# Scrutiny

[![CI](https://github.com/Starosdev/scrutiny/workflows/CI/badge.svg?branch=master)](https://github.com/Starosdev/scrutiny/actions?query=workflow%3ACI)
[![GitHub license](https://img.shields.io/github/license/Starosdev/scrutiny.svg?style=flat-square)](https://github.com/Starosdev/scrutiny/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Starosdev/scrutiny?style=flat-square)](https://goreportcard.com/report/github.com/Starosdev/scrutiny)
[![GitHub release](http://img.shields.io/github/release/Starosdev/scrutiny.svg?style=flat-square)](https://github.com/Starosdev/scrutiny/releases)
[![Docker Pulls](https://img.shields.io/badge/docker-ghcr.io%2Fstarosdev%2Fscrutiny-blue?style=flat-square&logo=docker)](https://github.com/Starosdev/scrutiny/pkgs/container/scrutiny)

**Hard Drive Health Dashboard & Monitoring for S.M.A.R.T metrics**

[![](docs/dashboard.png)](https://imgur.com/a/5k8qMzS)

# Why This Fork?

This fork exists to keep Scrutiny alive and growing. The original [AnalogJ/scrutiny](https://github.com/AnalogJ/scrutiny) project development slowed significantly in 2024, while community contributions and feature requests continued to grow. This fork picks up where the original left off, merging pending community PRs and adding new features.

Full credit for the original vision and architecture goes to [AnalogJ](https://github.com/AnalogJ). I started this fork as a learning project, so contributions from more experienced developers are greatly appreciated. Full disclosure: I use Claude to assist with development, but all code is manually reviewed by me before merging.

| | Original | This Fork |
|---|---|---|
| **Latest Version** | v0.8.1 (Apr 2024) | [![GitHub release](https://img.shields.io/github/v/release/Starosdev/scrutiny?label=&style=flat-square)](https://github.com/Starosdev/scrutiny/releases) |
| **Frontend** | Angular 13 | Modern Angular |
| **Status** | Minimal updates | Actively maintained |
| **Community PRs** | Many pending | Merged |

### What's New in This Fork

- **ZFS Pool Monitoring** - Monitor ZFS pool health alongside individual drives
- **Prometheus Metrics** - Export metrics to Prometheus for advanced monitoring
- **Device Archiving** - Hide decommissioned drives without deleting history
- **Per-Device Notification Control** - Mute notifications for specific devices
- **Device Labels** - Add custom labels to drives for easier identification
- **Day-Resolution Temperature Graphs** - More granular temperature history
- **SAS Temperature Support** - Proper temperature readings for SAS drives
- **SCT Temperature History Toggle** - Control SCT ERC settings per drive
- **S.M.A.R.T Attribute Overrides** - Override manufacturer thresholds via UI or config
- **Improved Dashboard Layout** - Sidebar navigation moved to top for better attribute visibility
- **Enhanced Mobile UI** - Optimized layout for mobile devices
- **Performance Benchmarking** - Run fio benchmarks and track drive throughput, IOPS, and latency over time
- **Enhanced Seagate Drive Support** - Better timeout handling for Seagate drives
- **SHA256 Checksums** - Verify release binary integrity

# Introduction

If you run a server with more than a couple of hard drives, you're probably already familiar with S.M.A.R.T and the `smartd` daemon. If not, it's an incredible open source project described as the following:

> smartd is a daemon that monitors the Self-Monitoring, Analysis and Reporting Technology (SMART) system built into many ATA, IDE and SCSI-3 hard drives. The purpose of SMART is to monitor the reliability of the hard drive and predict drive failures, and to carry out different types of drive self-tests.

These S.M.A.R.T hard drive self-tests can help you detect and replace failing hard drives before they cause permanent data loss. However, there's a couple issues with `smartd`:

- There are more than a hundred S.M.A.R.T attributes, however `smartd` does not differentiate between critical and informational metrics
- `smartd` does not record S.M.A.R.T attribute history, so it can be hard to determine if an attribute is degrading slowly over time.
- S.M.A.R.T attribute thresholds are set by the manufacturer. In some cases these thresholds are unset, or are so high that they can only be used to confirm a failed drive, rather than detecting a drive about to fail.
- `smartd` is a command line only tool. For head-less servers a web UI would be more valuable.

**Scrutiny is a Hard Drive Health Dashboard & Monitoring solution, merging manufacturer provided S.M.A.R.T metrics with real-world failure rates.**

# Features

### Core Features
- Web UI Dashboard - focused on Critical metrics
- `smartd` integration (no re-inventing the wheel)
- Auto-detection of all connected hard-drives
- S.M.A.R.T metric tracking for historical trends
- Customized thresholds using real world failure rates
- Temperature tracking with configurable resolution
- Provided as an all-in-one Docker image (but can be installed manually)
- Configurable Alerting/Notifications via Webhooks

### Extended Features (This Fork)
- **ZFS Pool Monitoring** - Track pool health, capacity, and status
- **Prometheus Metrics Endpoint** - `/api/metrics` for Grafana integration
- **Device Archiving** - Archive old drives to declutter the dashboard
- **Per-Device Notification Muting** - Control which drives trigger alerts
- **Custom Device Labels** - Add meaningful names to your drives
- **Day-Resolution Graphs** - View temperature trends at daily granularity
- **SAS Drive Support** - Full temperature support for SAS devices
- **S.M.A.R.T Attribute Overrides** - Override thresholds per device via UI
- **Improved UI Layout** - Top navigation for better S.M.A.R.T attribute visibility
- **Mobile-Optimized Interface** - Better experience on mobile devices
- **API Timeout Configuration** - Adjust timeouts for slow storage systems
- **Performance Benchmarking** - fio-based benchmarks for throughput, IOPS, and latency with historical tracking
- **Heartbeat Notifications** - Periodic "all clear" alerts for uptime monitoring integration

# Migration from AnalogJ/scrutiny

If you're currently using the original AnalogJ/scrutiny, migrating is straightforward:

1. **Update your image reference** from `ghcr.io/analogj/scrutiny` to `ghcr.io/starosdev/scrutiny`
2. **Data is compatible** - Your existing SQLite database and InfluxDB data will work without changes
3. **Config files are compatible** - No changes needed to `scrutiny.yaml` or `collector.yaml`

That's it! The fork maintains full backwards compatibility with the original project.

# Getting Started

## RAID/Virtual Drives

Scrutiny uses `smartctl --scan` to detect devices/drives.

- All RAID controllers supported by `smartctl` are automatically supported by Scrutiny.
    - While some RAID controllers support passing through the underlying SMART data to `smartctl` others do not.
    - In some cases `--scan` does not correctly detect the device type, returning incomplete SMART data.
    Scrutiny supports overriding detected device type via the config file: see [example.collector.yaml](example.collector.yaml)
- If you use docker, you **must** pass though the RAID virtual disk to the container using `--device` (see below)
    - This device may be in `/dev/*` or `/dev/bus/*`.
    - If you're unsure, run `smartctl --scan` on your host, and pass all listed devices to the container.

See [docs/TROUBLESHOOTING_DEVICE_COLLECTOR.md](./docs/TROUBLESHOOTING_DEVICE_COLLECTOR.md) for help

## Docker

If you're using Docker, getting started is as simple as running the following command:

> See [docker/example.omnibus.docker-compose.yml](docker/example.omnibus.docker-compose.yml) for a docker-compose file.

```bash
docker run -p 8080:8080 -p 8086:8086 --restart unless-stopped \
  -v `pwd`/scrutiny:/opt/scrutiny/config \
  -v `pwd`/influxdb2:/opt/scrutiny/influxdb \
  -v /run/udev:/run/udev:ro \
  --cap-add SYS_RAWIO \
  --device=/dev/sda \
  --device=/dev/sdb \
  --name scrutiny \
  ghcr.io/starosdev/scrutiny:latest-omnibus
```

- `/run/udev` is necessary to provide the Scrutiny collector with access to your device metadata
- `--cap-add SYS_RAWIO` is necessary to allow `smartctl` permission to query your device SMART data
    - NOTE: If you have **NVMe** drives, you must add `--cap-add SYS_ADMIN` as well.
- `--device` entries are required to ensure that your hard disk devices are accessible within the container.
- `ghcr.io/starosdev/scrutiny:latest-omnibus` is an omnibus image, containing both the webapp server (frontend & api) as well as the S.M.A.R.T metric collector. (see below)

### Hub/Spoke Deployment

In addition to the Omnibus image (available under the `latest` tag) you can deploy in Hub/Spoke mode, which requires 3
other Docker images:

- `ghcr.io/starosdev/scrutiny:latest-collector` - Contains the Scrutiny data collector, `smartctl` binary and cron-like
  scheduler. You can run one collector on each server.
- `ghcr.io/starosdev/scrutiny:latest-collector-zfs` - ZFS pool collector for monitoring ZFS health.
  Run alongside or instead of the standard collector if you use ZFS. See [docs/ZFS_POOL_MONITORING.md](./docs/ZFS_POOL_MONITORING.md) for setup instructions.
- `ghcr.io/starosdev/scrutiny:latest-collector-performance` - Performance benchmark collector using fio.
  Runs periodic benchmarks and tracks throughput, IOPS, and latency over time. See [Performance Benchmarking](#performance-benchmarking) for details.
- `ghcr.io/starosdev/scrutiny:latest-web` - Contains the Web UI and API. Only one container necessary
- `influxdb:2.2` - InfluxDB image, used by the Web container to persist SMART data. Only one container necessary.
  See [docs/TROUBLESHOOTING_INFLUXDB.md](./docs/TROUBLESHOOTING_INFLUXDB.md)

> See [docker/example.hubspoke.docker-compose.yml](docker/example.hubspoke.docker-compose.yml) for a docker-compose file.

```bash
docker run -p 8086:8086 --restart unless-stopped \
  -v `pwd`/influxdb2:/var/lib/influxdb2 \
  --name scrutiny-influxdb \
  influxdb:2.2

docker run -p 8080:8080 --restart unless-stopped \
  -v `pwd`/scrutiny:/opt/scrutiny/config \
  --name scrutiny-web \
  ghcr.io/starosdev/scrutiny:latest-web

docker run --restart unless-stopped \
  -v /run/udev:/run/udev:ro \
  --cap-add SYS_RAWIO \
  --device=/dev/sda \
  --device=/dev/sdb \
  -e COLLECTOR_API_ENDPOINT=http://SCRUTINY_WEB_IPADDRESS:8080 \
  --name scrutiny-collector \
  ghcr.io/starosdev/scrutiny:latest-collector
```

## Manual Installation (without-Docker)

While the easiest way to get started with Scrutiny is using Docker (see above),
it is possible to run it manually without much work. You can even mix and match, using Docker for one component and
a manual installation for the other.

See [docs/INSTALL_MANUAL.md](docs/INSTALL_MANUAL.md) for instructions.

## Usage

Once scrutiny is running, you can open your browser to `http://localhost:8080` and take a look at the dashboard.

If you're using the omnibus image, the collector should already have run, and your dashboard should be populate with every
drive that Scrutiny detected. The collector is configured to run once a day, but you can trigger it manually by running the command below.

For users of the docker Hub/Spoke deployment or manual install: initially the dashboard will be empty.
After the first collector run, you'll be greeted with a list of all your hard drives and their current smart status.

```bash
docker exec scrutiny /opt/scrutiny/bin/scrutiny-collector-metrics run
```

# Configuration
By default Scrutiny looks for its YAML configuration files in `/opt/scrutiny/config`

There are four configuration files available:

- Webapp/API config via `scrutiny.yaml` - [example.scrutiny.yaml](example.scrutiny.yaml).
- Collector config via `collector.yaml` - [example.collector.yaml](example.collector.yaml).
- ZFS Collector config via `collector-zfs.yaml` - [example.collector-zfs.yaml](example.collector-zfs.yaml). See [docs/ZFS_POOL_MONITORING.md](./docs/ZFS_POOL_MONITORING.md) for setup instructions.
- Performance Collector config via `collector-performance.yaml` - [example.collector-performance.yaml](example.collector-performance.yaml). Falls back to `collector.yaml` if not found.

None of these files are required, however if provided, they allow you to configure how Scrutiny functions.

## Cron Schedule
Unfortunately the Cron schedule cannot be configured via the `collector.yaml` (as the collector binary needs to be triggered by a scheduler/cron).
However, if you are using the official `ghcr.io/starosdev/scrutiny:latest-collector` or `ghcr.io/starosdev/scrutiny:latest-omnibus` docker images,
you can use the `COLLECTOR_CRON_SCHEDULE` environmental variable to override the default cron schedule (daily @ midnight - `0 0 * * *`).

`docker run -e COLLECTOR_CRON_SCHEDULE="0 0 * * *" ...`

## Prometheus Metrics

Scrutiny exposes a Prometheus metrics endpoint at `/api/metrics`. You can scrape this endpoint to integrate with Grafana or other monitoring tools.

Example Prometheus scrape config:
```yaml
scrape_configs:
  - job_name: 'scrutiny'
    static_configs:
      - targets: ['scrutiny:8080']
    metrics_path: '/api/metrics'
```

## Performance Benchmarking

Scrutiny can run periodic [fio](https://fio.readthedocs.io/) benchmarks on your drives and track performance over time. This helps detect drive degradation before S.M.A.R.T failures appear -- a drive that is suddenly 50% slower may be failing even if S.M.A.R.T attributes look normal.

### What's Measured

| Metric | Description |
| ------ | ----------- |
| Sequential Read/Write | Throughput in bytes/sec (large block sequential I/O) |
| Random Read/Write IOPS | Input/output operations per second (4K random I/O) |
| Read/Write Latency | Average, P50, P95, P99 latency in nanoseconds |
| Mixed Read/Write IOPS | Combined random read+write performance |

### How It Works

1. The **performance collector** (`scrutiny-collector-performance`) runs fio benchmarks on configured devices
2. Results are uploaded to the Scrutiny API and stored as time-series data in InfluxDB
3. The **web UI** displays performance history charts and summary cards on the device detail page
4. A **baseline** is computed from the last 5 results, and current results are compared against it
5. **Degradation detection** flags warnings (>20% throughput drop or >30% latency increase) and failures (>40% / >60%)

### Deployment

The performance collector is available as a separate Docker image:

```bash
docker run --restart unless-stopped \
  --device=/dev/sda \
  --device=/dev/sdb \
  -e COLLECTOR_API_ENDPOINT=http://SCRUTINY_WEB_IPADDRESS:8080 \
  --name scrutiny-perf-collector \
  ghcr.io/starosdev/scrutiny:latest-collector-performance
```

The collector requires direct device access (not virtualized). Running benchmarks will temporarily increase I/O on the target drives, so schedule accordingly.

### Viewing Results

Performance data appears on the device detail page when benchmark results are available. The UI shows:

- **Summary cards** with latest values and baseline comparison badges
- **Throughput chart** -- sequential read/write bandwidth over time
- **IOPS chart** -- random read/write and mixed IOPS over time
- **Latency chart** -- read latency (average, P95, P99) over time

Use the duration selector to view day, week, month, or year ranges.

## Notifications

Scrutiny supports sending SMART device failure notifications via the following services:
- Custom Script (data provided via environmental variables)
- Email
- Webhooks
- Discord
- Gotify
- Hangouts
- IFTTT
- Join
- Mattermost
- ntfy
- Pushbullet
- Pushover
- Slack
- Teams
- Telegram
- Tulip

Check the `notify.urls` section of [example.scrutiny.yml](example.scrutiny.yaml) for examples.

For more information and troubleshooting, see the [TROUBLESHOOTING_NOTIFICATIONS.md](./docs/TROUBLESHOOTING_NOTIFICATIONS.md) file

### Heartbeat Notifications

Scrutiny can send periodic "all clear" heartbeat notifications to confirm the monitoring system is running and all drives are healthy. This is useful for integration with uptime monitoring tools like Uptime Kuma.

- **Disabled by default** -- enable via Settings in the web UI or the `/api/settings` API
- **Configurable interval** -- defaults to every 24 hours
- **Suppressed during failures** -- heartbeat is not sent if any drive has active failures (failure notifications take priority)

### Per-Device Notification Control

You can mute notifications for specific devices through the web UI. This is useful for drives that are known to have issues but are being monitored before replacement.

### Testing Notifications

You can test that your notifications are configured correctly by posting an empty payload to the notifications health check API.

```bash
curl -X POST http://localhost:8080/api/health/notify
```

# Debug mode & Log Files
Scrutiny provides various methods to change the log level and generate log files.
The web server and collector have **independent** log configurations and can be set separately.

## Valid Log Levels

The following log levels are supported (case-insensitive), listed from highest to lowest severity:

| Level | Description |
| --- | --- |
| `PANIC` | Calls panic after logging |
| `FATAL` | Calls os.Exit(1) after logging |
| `ERROR` | Error conditions |
| `WARN` | Warning conditions (also accepts `WARNING`) |
| `INFO` | General operational messages **(default)** |
| `DEBUG` | Verbose diagnostic information |
| `TRACE` | Very fine-grained diagnostic information |

Setting a level includes all messages at that level **and above** (higher severity).
For example, setting `WARN` will show WARN, ERROR, FATAL, and PANIC messages, but not INFO, DEBUG, or TRACE.

## Web Server/API

You can use environmental variables to enable debug logging and/or log files for the web server:

```bash
DEBUG=true
SCRUTINY_LOG_FILE=/tmp/web.log
```

You can configure the log level and log file in the config file:

```yml
log:
  file: '/tmp/web.log'
  level: DEBUG
```

Or if you're not using docker, you can pass CLI arguments to the web server during startup:

```bash
scrutiny start --debug --log-file /tmp/web.log
```

### Web Server Environment Variable Overrides

Any web server configuration key can be overridden via environment variables using the `SCRUTINY_` prefix.
Dots and dashes in key names become underscores.

| Config Key | Environment Variable | Default Value |
| --- | --- | --- |
| `web.listen.port` | `SCRUTINY_WEB_LISTEN_PORT` | `8080` |
| `web.listen.host` | `SCRUTINY_WEB_LISTEN_HOST` | `0.0.0.0` |
| `web.listen.basepath` | `SCRUTINY_WEB_LISTEN_BASEPATH` | `` |
| `web.listen.read_timeout_seconds` | `SCRUTINY_WEB_LISTEN_READ_TIMEOUT_SECONDS` | `10` |
| `web.listen.write_timeout_seconds` | `SCRUTINY_WEB_LISTEN_WRITE_TIMEOUT_SECONDS` | `30` |
| `web.listen.idle_timeout_seconds` | `SCRUTINY_WEB_LISTEN_IDLE_TIMEOUT_SECONDS` | `60` |
| `web.database.location` | `SCRUTINY_WEB_DATABASE_LOCATION` | `/opt/scrutiny/config/scrutiny.db` |
| `web.database.journal_mode` | `SCRUTINY_WEB_DATABASE_JOURNAL_MODE` | `WAL` |
| `web.src.frontend.path` | `SCRUTINY_WEB_SRC_FRONTEND_PATH` | `/opt/scrutiny/web` |
| `web.influxdb.scheme` | `SCRUTINY_WEB_INFLUXDB_SCHEME` | `http` |
| `web.influxdb.host` | `SCRUTINY_WEB_INFLUXDB_HOST` | `localhost` |
| `web.influxdb.port` | `SCRUTINY_WEB_INFLUXDB_PORT` | `8086` |
| `web.influxdb.org` | `SCRUTINY_WEB_INFLUXDB_ORG` | `scrutiny` |
| `web.influxdb.bucket` | `SCRUTINY_WEB_INFLUXDB_BUCKET` | `metrics` |
| `web.influxdb.token` | `SCRUTINY_WEB_INFLUXDB_TOKEN` | `scrutiny-default-admin-token` |
| `web.influxdb.init_username` | `SCRUTINY_WEB_INFLUXDB_INIT_USERNAME` | `admin` |
| `web.influxdb.init_password` | `SCRUTINY_WEB_INFLUXDB_INIT_PASSWORD` | `password12345` |
| `web.influxdb.tls.insecure_skip_verify` | `SCRUTINY_WEB_INFLUXDB_TLS_INSECURE_SKIP_VERIFY` | `false` |
| `web.influxdb.retention_policy` | `SCRUTINY_WEB_INFLUXDB_RETENTION_POLICY` | `true` |
| `web.influxdb.retention.daily` | `SCRUTINY_WEB_INFLUXDB_RETENTION_DAILY` | `1296000` (15 days) |
| `web.influxdb.retention.weekly` | `SCRUTINY_WEB_INFLUXDB_RETENTION_WEEKLY` | `5443200` (9 weeks) |
| `web.influxdb.retention.monthly` | `SCRUTINY_WEB_INFLUXDB_RETENTION_MONTHLY` | `65318400` (25 months) |
| `web.metrics.enabled` | `SCRUTINY_WEB_METRICS_ENABLED` | `true` |
| `log.level` | `SCRUTINY_LOG_LEVEL` | `INFO` |
| `log.file` | `SCRUTINY_LOG_FILE` | `` |
| `notify.urls` | `SCRUTINY_NOTIFY_URLS` | `` |
| `failures.transient.ata` | `SCRUTINY_FAILURES_TRANSIENT_ATA` | `[195]` |
| `failures.ignored.ata` | `SCRUTINY_FAILURES_IGNORED_ATA` | `[]` |
| `failures.ignored.devstat` | `SCRUTINY_FAILURES_IGNORED_DEVSTAT` | `[]` |
| `failures.ignored.nvme` | `SCRUTINY_FAILURES_IGNORED_NVME` | `[]` |
| `failures.ignored.scsi` | `SCRUTINY_FAILURES_IGNORED_SCSI` | `[]` |

Environment variables take precedence over config file values. This is useful for containerized
deployments where you want to override specific settings without modifying the config file.

Example:

```bash
docker run -e SCRUTINY_WEB_LISTEN_PORT=9090 \
  -e SCRUTINY_WEB_INFLUXDB_HOST=influxdb.local \
  -e SCRUTINY_LOG_LEVEL=DEBUG \
  ghcr.io/starosdev/scrutiny:web
```

## Collector

You can use environmental variables to enable debug logging and/or log files for the collector:

```bash
DEBUG=true
COLLECTOR_LOG_FILE=/tmp/collector.log
```

Or if you're not using docker, you can pass CLI arguments to the collector during startup:

```bash
scrutiny-collector-metrics run --debug --log-file /tmp/collector.log
```

### Collector Environment Variable Overrides

Any collector configuration key can be overridden via environment variables using the `COLLECTOR_` prefix.
Dots and dashes in key names become underscores.

| Config Key | Environment Variable | Default Value |
| --- | --- | --- |
| `host.id` | `COLLECTOR_HOST_ID` | `` |
| `api.endpoint` | `COLLECTOR_API_ENDPOINT` | `http://localhost:8080` |
| `api.timeout` | `COLLECTOR_API_TIMEOUT` | `60` |
| `commands.metrics_smartctl_bin` | `COLLECTOR_COMMANDS_METRICS_SMARTCTL_BIN` | `smartctl` |
| `commands.metrics_scan_args` | `COLLECTOR_COMMANDS_METRICS_SCAN_ARGS` | `--scan --json` |
| `commands.metrics_info_args` | `COLLECTOR_COMMANDS_METRICS_INFO_ARGS` | `--info --json` |
| `commands.metrics_smart_args` | `COLLECTOR_COMMANDS_METRICS_SMART_ARGS` | `--xall --json` |
| `commands.metrics_smartctl_wait` | `COLLECTOR_COMMANDS_METRICS_SMARTCTL_WAIT` | `0` |
| `allow_listed_devices` | `COLLECTOR_ALLOW_LISTED_DEVICES` | `[]` |
| `log.level` | `COLLECTOR_LOG_LEVEL` | `INFO` |
| `log.file` | `COLLECTOR_LOG_FILE` | `` |

Environment variables take precedence over config file values. This is useful for containerized
deployments where you want to override specific settings without modifying the config file.

Example:

```bash
docker run -e COLLECTOR_COMMANDS_METRICS_SMART_ARGS="--xall --json -T permissive" \
  -e COLLECTOR_API_ENDPOINT=http://scrutiny-web:8080 \
  ghcr.io/starosdev/scrutiny:collector
```

### Docker-Only Environment Variables

These environment variables are only available when running the collector in Docker containers (handled by the entrypoint script, not Viper configuration):

| Environment Variable | Default Value | Description |
| --- | --- | --- |
| `COLLECTOR_CRON_SCHEDULE` | `0 0 * * *` | Cron schedule for SMART data collection |
| `COLLECTOR_RUN_STARTUP` | `false` | Run collector immediately on container start |
| `COLLECTOR_RUN_STARTUP_SLEEP` | `1` | Delay in seconds before startup collection |

## Performance Collector

The performance collector is a separate binary (`scrutiny-collector-performance`) that runs fio benchmarks. It can use its own config file (`collector-performance.yaml`) or fall back to the main `collector.yaml`.

```bash
DEBUG=true
COLLECTOR_PERF_LOG_FILE=/tmp/performance.log
```

Or via CLI:

```bash
scrutiny-collector-performance run --debug --log-file /tmp/performance.log --profile quick
```

### Performance Collector Environment Variable Overrides

The performance collector checks `COLLECTOR_PERF_` prefixed variables first, then falls back to `COLLECTOR_` prefixed variables.

| Config Key | Environment Variable | Default Value |
| --- | --- | --- |
| `host.id` | `COLLECTOR_PERF_HOST_ID` or `COLLECTOR_HOST_ID` | `` |
| `api.endpoint` | `COLLECTOR_PERF_API_ENDPOINT` or `COLLECTOR_API_ENDPOINT` | `http://localhost:8080` |
| `performance.profile` | `COLLECTOR_PERF_PROFILE` | `quick` |
| `performance.enabled` | `COLLECTOR_PERFORMANCE_ENABLED` | `true` |
| `performance.temp_file_size` | `COLLECTOR_PERFORMANCE_TEMP_FILE_SIZE` | `256M` |
| `commands.performance_fio_bin` | `COLLECTOR_COMMANDS_PERFORMANCE_FIO_BIN` | `fio` |
| `log.level` | `COLLECTOR_PERF_DEBUG` or `COLLECTOR_DEBUG` | `INFO` |
| `log.file` | `COLLECTOR_PERF_LOG_FILE` or `COLLECTOR_LOG_FILE` | `` |

### Performance Collector Docker-Only Environment Variables

| Environment Variable | Default Value | Description |
| --- | --- | --- |
| `COLLECTOR_PERF_CRON_SCHEDULE` | `0 2 * * 0` | Cron schedule (default: Sunday 2 AM) |
| `COLLECTOR_PERF_RUN_STARTUP` | `false` | Run benchmark immediately on container start |
| `COLLECTOR_PERF_RUN_STARTUP_SLEEP` | `1` | Delay in seconds before startup run |

Example:

```bash
docker run --restart unless-stopped \
  --device=/dev/sda \
  --device=/dev/sdb \
  -e COLLECTOR_PERF_API_ENDPOINT=http://scrutiny-web:8080 \
  -e COLLECTOR_PERF_PROFILE=quick \
  -e COLLECTOR_PERF_CRON_SCHEDULE="0 2 * * 0" \
  ghcr.io/starosdev/scrutiny:latest-collector-performance
```

# Supported Architectures

| Architecture Name | Binaries | Docker |
| --- | --- | --- |
| linux-amd64 | :white_check_mark: | :white_check_mark: |
| linux-arm-5 | :white_check_mark: |  |
| linux-arm-6 | :white_check_mark: |  |
| linux-arm-7 | :white_check_mark: | web/collector only |
| linux-arm64 | :white_check_mark: | :white_check_mark: |
| freebsd-amd64 | :white_check_mark: |  |
| macos-amd64 | :white_check_mark: | :white_check_mark: |
| macos-arm64 | :white_check_mark: | :white_check_mark: |
| windows-amd64 | :white_check_mark: | WIP |
| windows-arm64 | :white_check_mark: |  |


# Contributing

Please see the [CONTRIBUTING.md](CONTRIBUTING.md) for instructions for how to develop and contribute to the scrutiny codebase.

Work your magic and then submit a pull request. We love pull requests!

If you find the documentation lacking, help us out and update this README.md. If you don't have the time to work on Scrutiny, but found something we should know about, please submit an issue.

# Versioning

We use SemVer for versioning. For the versions available, see the tags on this repository.

# Credits

**Original Author:** Jason Kulatunga ([@AnalogJ](https://github.com/AnalogJ)) -- Created Scrutiny and built the foundation this fork builds upon.

**Fork Maintainer:** [@Starosdev](https://github.com/Starosdev) -- Maintaining this fork with continued development and community contributions.

# License

- MIT
- Logo: [Glasses by matias porta lezcano](https://thenounproject.com/term/glasses/775232)
