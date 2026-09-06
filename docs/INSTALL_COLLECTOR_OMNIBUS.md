# Standalone Collector Omnibus

The standalone collector omnibus is one platform-specific archive containing
the SMART, ZFS, MDADM, Btrfs, filesystem, and performance collectors plus a
manager that schedules and supervises them. It is intended for hub/spoke
installations that do not use Docker.

The manager does not install host tools. Install the tools required by each
enabled collector:

| Collector | Required host tool |
| --- | --- |
| SMART metrics | `smartctl` from smartmontools |
| ZFS | `zpool` |
| MDADM | `mdadm` |
| Btrfs | `btrfs` |
| Filesystem | OS filesystem utilities and access to monitored mounts |
| Performance | `fio` |

## Install

Download the archive matching the host from a Scrutiny GitHub release. Unix
targets use `.tar.gz`; Windows targets use `.zip`.

After extracting, the package contains:

```text
scrutiny-collector-omnibus-<os>-<arch>/
├── bin/
│   ├── scrutiny-collector-omnibus
│   └── scrutiny-collector-*
├── config/
│   ├── collector-omnibus.yaml
│   └── collector*.yaml
├── INSTALL.md
└── LICENSE
```

Edit `config/collector.yaml` with the hub API endpoint and credentials. Enable
optional collectors and set their schedules in
`config/collector-omnibus.yaml`. Each enabled collector must have either a
schedule or `run_on_startup: true`.

Validate the complete installation before starting it:

```bash
./bin/scrutiny-collector-omnibus validate \
  --config ./config/collector-omnibus.yaml
```

Run the manager under a long-lived platform service such as systemd, launchd,
Windows Task Scheduler at startup, or a Windows service wrapper:

```bash
./bin/scrutiny-collector-omnibus run \
  --config ./config/collector-omnibus.yaml
```

The manager uses standard five-field cron expressions in the process's local
timezone. Runs for different collectors may overlap. A second run of the same
collector is skipped while its previous run is still active. A collector
failure is logged without stopping other schedules. `SIGINT` or `SIGTERM`
stops scheduling and cancels active child processes.

## Configuration precedence

The YAML file is the primary configuration surface for fleet-managed
installations. Environment variables override YAML values:

| YAML field | Metrics | Optional collector example |
| --- | --- | --- |
| `enabled` | `COLLECTOR_METRICS_ENABLED` | `COLLECTOR_ZFS_ENABLED` |
| `schedule` | `COLLECTOR_CRON_SCHEDULE` | `COLLECTOR_ZFS_CRON_SCHEDULE` |
| `run_on_startup` | `COLLECTOR_RUN_STARTUP` | `COLLECTOR_ZFS_RUN_STARTUP` |
| `startup_sleep_secs` | `COLLECTOR_RUN_STARTUP_SLEEP` | `COLLECTOR_ZFS_RUN_STARTUP_SLEEP` |
| `config` | `COLLECTOR_METRICS_CONFIG` | `COLLECTOR_ZFS_CONFIG` |

Optional collector prefixes are `COLLECTOR_ZFS`, `COLLECTOR_MDADM`,
`COLLECTOR_BTRFS`, `COLLECTOR_FILESYSTEM`, and `COLLECTOR_PERF`. A non-empty
schedule or a true startup override enables that collector unless its
`*_ENABLED` variable explicitly disables it.

A schedule override is trimmed before it is applied, so a whitespace value
clears the YAML schedule. See "Windows PowerShell schedule overrides" below for
why that matters on Windows.

Use `COLLECTOR_OMNIBUS_CONFIG` instead of `--config` when a service definition
provides configuration through its environment.
`COLLECTOR_OMNIBUS_BINARY_DIR` overrides `binary_dir`.

Collector API, host, logging, and device settings remain in their existing
collector YAML files and environment variables. The manager removes scheduling
variables from child processes so each scheduled child performs exactly one
collection and exits.

## Windows PowerShell schedule overrides

PowerShell deletes an environment variable when it is assigned an empty string.
`$env:COLLECTOR_CRON_SCHEDULE = ""` removes the variable instead of passing an
empty value, so the manager finds no override and the YAML schedule stays
active:

```powershell
# Does not clear the schedule. The manager logs "collector scheduled".
$env:COLLECTOR_CRON_SCHEDULE = ""
.\bin\scrutiny-collector-omnibus.exe run --config .\config\collector-omnibus.yaml
```

For persistent configuration, clear the schedule in YAML and enable the startup
run. This is the preferred form because it does not depend on shell quoting:

```yaml
collectors:
  metrics:
    enabled: true
    schedule: ""
    run_on_startup: true
    config: collector.yaml
```

For a temporary override in one PowerShell session, assign a whitespace value.
The variable survives, and the manager trims the override to an empty schedule:

```powershell
$env:COLLECTOR_CRON_SCHEDULE = " "
$env:COLLECTOR_RUN_STARTUP = "true"
.\bin\scrutiny-collector-omnibus.exe run --config .\config\collector-omnibus.yaml
```

With no collector holding a schedule, the manager runs each startup collector
once and exits.

An enabled collector needs either a schedule or a startup run. Clearing the
schedule while `run_on_startup` is false fails validation with
`collector "metrics" is enabled but has no schedule or startup run`. Set
`run_on_startup: true`, or the matching `*_RUN_STARTUP` variable, alongside the
cleared schedule.

The same rules apply to every optional collector schedule variable:
`COLLECTOR_ZFS_CRON_SCHEDULE`, `COLLECTOR_MDADM_CRON_SCHEDULE`,
`COLLECTOR_BTRFS_CRON_SCHEDULE`, `COLLECTOR_FILESYSTEM_CRON_SCHEDULE`, and
`COLLECTOR_PERF_CRON_SCHEDULE`. Each pairs with its own `*_RUN_STARTUP`
variable.

`cmd.exe` behaves the same way: `set COLLECTOR_CRON_SCHEDULE=` removes the
variable, and `set COLLECTOR_CRON_SCHEDULE= ` assigns a whitespace value that
the manager trims. Unix shells pass an empty value directly, so
`COLLECTOR_CRON_SCHEDULE="" ./bin/scrutiny-collector-omnibus run` needs no
workaround.
