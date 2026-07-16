# Scrutiny Architecture

This page shows how Scrutiny's main runtime components fit together in the two deployment models supported by this repository.

## Omnibus

The `ghcr.io/starosdev/scrutiny:latest-omnibus` image bundles the collector, web/API server, InfluxDB, and SQLite into a single container. The browser frontend is served by the web/API server and talks to it over HTTP.

```mermaid
flowchart LR
    Browser["Frontend / Browser"]

    subgraph Omnibus["Omnibus container (`latest-omnibus`)"]
        Collector["Collector"]
        Web["Web/API server"]
        Influx["InfluxDB"]
        SQLite["SQLite"]
    end

    Collector -->|"uploads SMART metrics"| Web
    Web -->|"writes time-series metrics"| Influx
    Web -->|"stores app metadata and settings"| SQLite
    Browser -->|"HTTP requests"| Web
    Web -->|"HTML, JS, API responses"| Browser
```

- The collector discovers devices and submits SMART data to the local web/API service.
- InfluxDB stores time-series drive metrics and historical readings.
- SQLite stores application metadata such as device records, settings, and UI-managed configuration.
- Device metadata persisted in SQLite includes fields used for model-aware ATA scoring, such as `model_family`.
- The frontend is delivered by the same web/API service, so all browser traffic terminates at the omnibus container.
- Frontend settings are loaded in two stages: the Angular app starts from bundled defaults, then merges persisted settings from `/api/settings`. Browser code should assume a complete default-backed config exists immediately and treat the API fetch as hydration, not first-time initialization.

## Hub/Spoke

The Hub/Spoke deployment separates the collector from the central web/API service. One or more collector containers run on the systems that have access to disks, then send their results to a hub running `latest-web` plus its databases. Use `latest-collector` for SMART-only spokes or `latest-collector-omnibus` when you want one spoke container that can also run the ZFS, MDADM, Btrfs, filesystem, and performance collectors.

```mermaid
flowchart LR
    Browser["Frontend / Browser"]
    CollectorA["Collector (`latest-collector`)"]
    CollectorB["Collector Omnibus (`latest-collector-omnibus`)"]

    subgraph Hub["Hub services"]
        Web["Web/API server (`latest-web`)"]
        Influx["InfluxDB"]
        SQLite["SQLite"]
    end

    CollectorA -->|"uploads SMART metrics over network"| Web
    CollectorB -->|"uploads SMART metrics over network"| Web
    Web -->|"writes time-series metrics"| Influx
    Web -->|"stores app metadata and settings"| SQLite
    Browser -->|"HTTP requests"| Web
    Web -->|"HTML, JS, API responses"| Browser
```

- Collectors run close to the disks they monitor and communicate with the hub over the network.
- The collector-omnibus spoke image bundles the non-web collector binaries and host tools, but still uses the same per-collector env vars and schedules to decide which optional collectors run.
- The hub's web/API service aggregates data from all spokes and serves the frontend.
- InfluxDB remains the time-series store for SMART history.
- SQLite remains the metadata store for device records, settings, and other app state managed by the web/API layer.
- That metadata includes drive identity fields used by ATA consumer-drive profile matching and the global opt-out setting for those overrides.
- The same frontend config rule applies here: client code should render against bundled defaults first, then merge hub-provided settings when the API response arrives.

## Component Roles

- `Collector`: discovers devices, runs `smartctl`, and submits results to the API.
- `Web/API server`: accepts collector uploads, serves the frontend, and provides the API consumed by the UI.
- `InfluxDB`: stores historical SMART and other time-series measurements.
- `SQLite`: stores application metadata and configuration state, including persisted drive identity fields and user-managed settings such as `metrics.consumer_drive_profiles_enabled`.
- `Frontend / Browser`: renders the dashboard and calls the API exposed by the web/API server.

See [CONSUMER_DRIVE_PROFILES.md](./CONSUMER_DRIVE_PROFILES.md) for the ATA consumer-drive profile feature and fallback behavior.
