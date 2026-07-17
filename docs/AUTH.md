# Authentication

Scrutiny supports opt-in API authentication to secure access to the dashboard and API endpoints. Authentication is **disabled by default**, so existing deployments are completely unaffected by this feature.

When enabled, all API endpoints except `/api/health` and `/api/auth/*` require a valid Bearer token in the `Authorization` header. The web UI provides a login page that issues JWT session tokens, and collectors authenticate using the master API token.

## Quick Start

Add the following to your `scrutiny.yaml`:

```yaml
web:
    auth:
        enabled: true
        token: 'your-secret-api-token-here'
```

Then configure each collector with the same token. See [Collector Authentication](#collector-authentication) for details.

## Configuration Reference

### Web Server (`scrutiny.yaml`)

| Config Key | Environment Variable | Default | Description |
|---|---|---|---|
| `web.auth.enabled` | `SCRUTINY_WEB_AUTH_ENABLED` | `false` | Enable API authentication |
| `web.auth.token` | `SCRUTINY_WEB_AUTH_TOKEN` | (empty) | Master API token. Required when auth is enabled. Used by collectors and for token-based login. |
| `web.auth.jwt_secret` | `SCRUTINY_WEB_AUTH_JWT_SECRET` | (empty) | Secret key for signing JWT session tokens. If empty, a random secret is generated at startup (sessions will not survive restarts). See [JWT Session Persistence](#jwt-session-persistence). |
| `web.auth.jwt_expiry_hours` | `SCRUTINY_WEB_AUTH_JWT_EXPIRY_HOURS` | `24` | JWT token expiry in hours |
| `web.auth.admin_username` | `SCRUTINY_WEB_AUTH_ADMIN_USERNAME` | `admin` | Admin username for password-based login |
| `web.auth.admin_password` | `SCRUTINY_WEB_AUTH_ADMIN_PASSWORD` | (empty) | Admin password. When set, enables the username/password login method in addition to token login. |
| `web.docs.public` | `SCRUTINY_WEB_DOCS_PUBLIC` | `false` | Expose `/docs/api` and `/api/docs/openapi.yaml` without authentication. Leave `false` for production-facing deployments. |
| `web.metrics.token` | `SCRUTINY_WEB_METRICS_TOKEN` | (empty) | Independent bearer token for the Prometheus `/api/metrics` endpoint. See [Prometheus Metrics Authentication](#prometheus-metrics-authentication). |

### Collector configuration files

| Config Key | Environment Variable | Default | Description |
|---|---|---|---|
| `api.token` (metrics) | `COLLECTOR_METRICS_API_TOKEN` (falls back to `COLLECTOR_API_TOKEN`) | (empty) | API token for the metrics collector. Falls back to `COLLECTOR_API_TOKEN` if not set. |
| `api.token` (performance) | `COLLECTOR_PERF_API_TOKEN` (falls back to `COLLECTOR_API_TOKEN`) | (empty) | API token for the performance collector. Falls back to `COLLECTOR_API_TOKEN` if not set. |
| `api.token` (zfs) | `COLLECTOR_ZFS_API_TOKEN` (falls back to `COLLECTOR_API_TOKEN`) | (empty) | API token for the ZFS collector. Falls back to `COLLECTOR_API_TOKEN` if not set. |
| `api.token` (btrfs) | `COLLECTOR_BTRFS_API_TOKEN` (falls back to `COLLECTOR_API_TOKEN`) | (empty) | API token for the Btrfs collector. Falls back to `COLLECTOR_API_TOKEN` if not set. |
| `api.token` (mdadm) | `COLLECTOR_MDADM_API_TOKEN` (falls back to `COLLECTOR_API_TOKEN`) | (empty) | API token for the MDADM collector. Falls back to `COLLECTOR_API_TOKEN` if not set. |
| `api.token` (filesystem) | `COLLECTOR_FILESYSTEM_API_TOKEN` (falls back to `COLLECTOR_API_TOKEN`) | (empty) | API token for the filesystem collector. Falls back to `COLLECTOR_API_TOKEN` if not set. |

## Public Endpoints

The following endpoints never require authentication, even when auth is enabled:

| Method | Path | Description |
|---|---|---|
| GET | `/api/health` | Health check for load balancers and monitoring tools |
| GET | `/api/auth/status` | Returns whether auth is enabled and which login methods are available |
| POST | `/api/auth/login` | Authenticate with token or username/password to obtain a JWT |

## API Docs Access

The served API docs routes are protected by default:

| Path | Default Behavior |
|---|---|
| `/docs/api` | Requires the master API token or a valid JWT when `web.auth.enabled=true` |
| `/api/docs/openapi.yaml` | Requires the master API token or a valid JWT when `web.auth.enabled=true` |

To expose both routes publicly for local or testing environments, set:

```yaml
web:
    docs:
        public: true
```

Recommended usage:

- Keep `web.docs.public: false` for internet-facing or shared deployments.
- Set `web.docs.public: true` only when you intentionally want anonymous access to the OpenAPI contract and Swagger UI.

## Web UI Login

When authentication is enabled, the web UI displays a login page. Two login methods are supported:

### Token Login (always available)

Enter the master API token (the value of `web.auth.token`) directly into the login form. This method is always available when auth is enabled.

### Password Login (optional)

When `web.auth.admin_password` is configured, a username/password form is also available. To enable password login, add the following to your `scrutiny.yaml`:

```yaml
web:
    auth:
        enabled: true
        token: 'your-secret-api-token-here'
        admin_username: 'admin'
        admin_password: 'your-admin-password-here'
```

Both login methods issue a JWT session token that the browser stores and sends with subsequent requests. The JWT expires after the configured `jwt_expiry_hours` (default 24 hours).

## Collector Authentication

When authentication is enabled on the server, each collector must be configured with the same API token set in `web.auth.token`. The token is sent as a `Bearer` token in the `Authorization` header on every API request.

### Metrics Collector (scrutiny-collector-metrics)

Configure the token using any of these methods (in order of precedence):

1. **CLI flag**: `--api-token your-secret-api-token-here`
2. **Environment variable**: `COLLECTOR_METRICS_API_TOKEN=your-secret-api-token-here` (falls back to `COLLECTOR_API_TOKEN`)
3. **Config file** (`collector.yaml`):
    ```yaml
    api:
        token: 'your-secret-api-token-here'
    ```

### Performance Collector (scrutiny-collector-performance)

Configure the token using any of these methods (in order of precedence):

1. **CLI flag**: `--api-token your-secret-api-token-here`
2. **Environment variable**: `COLLECTOR_PERF_API_TOKEN=your-secret-api-token-here` (falls back to `COLLECTOR_API_TOKEN`)
3. **Config file** (`collector-performance.yaml` or `collector.yaml`):
    ```yaml
    api:
        token: 'your-secret-api-token-here'
    ```

### ZFS Collector (scrutiny-collector-zfs)

Configure the token using any of these methods (in order of precedence):

1. **CLI flag**: `--api-token your-secret-api-token-here`
2. **Environment variable**: `COLLECTOR_ZFS_API_TOKEN=your-secret-api-token-here` (falls back to `COLLECTOR_API_TOKEN`)
3. **Config file** (`collector-zfs.yaml` or `collector.yaml`):
    ```yaml
    api:
        token: 'your-secret-api-token-here'
    ```

### Btrfs Collector (scrutiny-collector-btrfs)

Configure the token using any of these methods (in order of precedence):

1. **CLI flag**: `--api-token your-secret-api-token-here`
2. **Environment variable**: `COLLECTOR_BTRFS_API_TOKEN=your-secret-api-token-here` (falls back to `COLLECTOR_API_TOKEN`)
3. **Config file** (`collector-btrfs.yaml` or `collector.yaml`):
    ```yaml
    api:
        token: 'your-secret-api-token-here'
    ```

### Backward Compatibility

When authentication is disabled on the server (the default), collectors work without any token configuration. No changes are required to existing collector setups until you explicitly enable auth.

## Prometheus Metrics Authentication

The Prometheus `/api/metrics` endpoint supports an independent authentication token via `web.metrics.token`. This allows you to secure Prometheus scraping without enabling full API authentication, or to use a separate token for metrics access.

### Configuration

Set the metrics token in `scrutiny.yaml` or via environment variable:

```yaml
web:
    metrics:
        enabled: true
        token: 'your-metrics-token-here'
```

Or: `SCRUTINY_WEB_METRICS_TOKEN=your-metrics-token-here`

### Prometheus Scrape Configuration

Using an inline token:

```yaml
scrape_configs:
    - job_name: scrutiny
      metrics_path: '/api/metrics'
      bearer_token: 'your-metrics-token-here'
      static_configs:
          - targets: ['localhost:8080']
```

Using a token file:

```yaml
scrape_configs:
    - job_name: scrutiny
      metrics_path: '/api/metrics'
      bearer_token_file: '/etc/prometheus/scrutiny-metrics-token'
      static_configs:
          - targets: ['localhost:8080']
```

### Behavior Matrix

| `web.auth.enabled` | `web.metrics.token` | `/api/metrics` access |
|---|---|---|
| `false` | (empty) | Open, no authentication required |
| `false` | set | Requires the metrics token |
| `true` | (empty) | Requires the master API token or a valid JWT |
| `true` | set | Accepts the metrics token **or** the master API token / JWT |

### Exported Metric Families

The Prometheus exporter is a current-state surface for scraping and alerting. Historical analysis remains in InfluxDB and the workload or ZFS dashboards built on top of it.

The `/api/metrics` endpoint now exposes:

- Existing SMART and device inventory metrics.
- ZFS pool metrics for size, allocated and free bytes, capacity, fragmentation, aggregate error counts, and scrub progress.
- Workload metrics for daily read and write activity, total read and write bytes, read/write ratio, calculation time span, and data point count.
- Optional workload endurance and spike metrics when those values can be derived from the available SMART history.

Prometheus workload metrics use the same `week` duration window as the backend workload summary endpoint. They are derived current-state estimates, not raw point-in-time counters and not full historical exports.

### Categorical Metrics

Categorical values are exported in two forms:

- One-hot gauges for label-friendly Prometheus queries.
- Numeric code gauges for compact alert rules or dashboards that prefer enum mappings.
- Unknown or empty ZFS status and scrub values are exported as one-hot samples with the label value `unknown`, and as numeric code `0`.

#### ZFS Status Codes

| Value | Code |
|---|---|
| unknown or empty | `0` |
| `ONLINE` | `1` |
| `DEGRADED` | `2` |
| `FAULTED` | `3` |
| `OFFLINE` | `4` |
| `REMOVED` | `5` |
| `UNAVAIL` | `6` |

#### ZFS Scrub State Codes

| Value | Code |
|---|---|
| unknown or empty | `0` |
| `none` | `1` |
| `scanning` | `2` |
| `finished` | `3` |
| `canceled` | `4` |

#### Workload Intensity Codes

| Value | Code |
|---|---|
| `unknown` | `0` |
| `idle` | `1` |
| `light` | `2` |
| `medium` | `3` |
| `heavy` | `4` |

## JWT Session Persistence

When authentication is enabled, the server signs JWT session tokens using `web.auth.jwt_secret`. If this value is not set, the server generates a random secret at startup. This means that all existing JWT sessions are invalidated whenever the server restarts.

To preserve sessions across restarts, set a stable `jwt_secret`:

```yaml
web:
    auth:
        enabled: true
        token: 'your-secret-api-token-here'
        jwt_secret: 'a-stable-64-character-hex-string'
```

Generate a suitable secret with:

```bash
openssl rand -hex 32
```

## Docker Deployment

### Omnibus (single container)

```yaml
version: '3.5'

services:
    scrutiny:
        image: ghcr.io/starosdev/scrutiny:latest-omnibus
        cap_add:
            - SYS_RAWIO
            - SYS_ADMIN
        ports:
            - '8080:8080'
            - '8086:8086'
        volumes:
            - /run/udev:/run/udev:ro
            - ./scrutiny-config:/opt/scrutiny/config
            - ./influxdb:/opt/scrutiny/influxdb
        devices:
            - /dev/sda
            - /dev/sdb
        environment:
            SCRUTINY_WEB_AUTH_ENABLED: 'true'
            SCRUTINY_WEB_AUTH_TOKEN: 'your-secret-api-token-here'
            SCRUTINY_WEB_AUTH_ADMIN_PASSWORD: 'your-admin-password-here'
            # Optional: stable JWT secret for persistent sessions
            SCRUTINY_WEB_AUTH_JWT_SECRET: 'a-stable-64-character-hex-string'
            # Optional: independent Prometheus metrics token
            SCRUTINY_WEB_METRICS_TOKEN: 'your-metrics-token-here'
            # All embedded collectors read this automatically
            COLLECTOR_API_TOKEN: 'your-secret-api-token-here'
```

### Hub/Spoke (separate containers)

```yaml
version: '3.5'

services:
    influxdb:
        image: influxdb:2.9-alpine
        ports:
            - '8086:8086'
        volumes:
            - ./influxdb-data:/var/lib/influxdb2
        environment:
            DOCKER_INFLUXDB_INIT_MODE: setup
            DOCKER_INFLUXDB_INIT_USERNAME: admin
            DOCKER_INFLUXDB_INIT_PASSWORD: password12345
            DOCKER_INFLUXDB_INIT_ORG: scrutiny
            DOCKER_INFLUXDB_INIT_BUCKET: metrics
            DOCKER_INFLUXDB_INIT_ADMIN_TOKEN: my-super-secret-auth-token
            INFLUXD_USE_HASHED_TOKENS: "false"

    web:
        image: ghcr.io/starosdev/scrutiny:latest-web
        ports:
            - '8080:8080'
        volumes:
            - ./scrutiny-config:/opt/scrutiny/config
        environment:
            SCRUTINY_WEB_INFLUXDB_HOST: influxdb
            SCRUTINY_WEB_AUTH_ENABLED: 'true'
            SCRUTINY_WEB_AUTH_TOKEN: 'your-secret-api-token-here'
            SCRUTINY_WEB_AUTH_ADMIN_PASSWORD: 'your-admin-password-here'
            SCRUTINY_WEB_AUTH_JWT_SECRET: 'a-stable-64-character-hex-string'
        depends_on:
            - influxdb

    collector:
        image: ghcr.io/starosdev/scrutiny:latest-collector
        cap_add:
            - SYS_RAWIO
        volumes:
            - /run/udev:/run/udev:ro
        environment:
            COLLECTOR_API_ENDPOINT: http://web:8080
            COLLECTOR_API_TOKEN: 'your-secret-api-token-here'
        devices:
            - /dev/sda
            - /dev/sdb
        depends_on:
            - web
```

## Migration Guide

For existing users who want to add authentication to a running Scrutiny deployment.

### Step 0: No Action Required

Authentication is disabled by default. If you do not need auth, no changes are necessary. Your existing deployment continues to work as before.

### Step 1: Enable Server Authentication

Add `web.auth.enabled` and `web.auth.token` to your `scrutiny.yaml` or set the corresponding environment variables:

```yaml
web:
    auth:
        enabled: true
        token: 'your-secret-api-token-here'
```

Restart the Scrutiny web server. The web UI will now show a login page and API endpoints will require a Bearer token.

### Step 2: Configure Collectors

Set the same token on every collector so they can continue sending data. The simplest method is an environment variable:

```bash
COLLECTOR_API_TOKEN='your-secret-api-token-here'
```

This works for all collectors, including metrics, performance, ZFS, Btrfs, MDADM, and filesystem collectors. See [Collector Authentication](#collector-authentication) for per-collector details.

### Step 3 (Optional): Enable Password Login

If you prefer a username/password form instead of pasting the API token, set `web.auth.admin_password`:

```yaml
web:
    auth:
        enabled: true
        token: 'your-secret-api-token-here'
        admin_password: 'your-admin-password-here'
```

### Step 4 (Optional): Secure Prometheus Metrics

If you expose `/api/metrics` to Prometheus and want to restrict access independently:

```yaml
web:
    metrics:
        token: 'your-metrics-token-here'
```

Then update your Prometheus scrape configuration to include the token (see [Prometheus Metrics Authentication](#prometheus-metrics-authentication)).

## Troubleshooting

### Collector returns 401 Unauthorized

The collector's `api.token` does not match the server's `web.auth.token`. Verify that both values are identical. Check for trailing whitespace or quoting differences. The collector logs this error as:

```text
Authentication failed (HTTP 401). Check that api.token in collector.yaml matches web.auth.token in scrutiny.yaml.
```

### Web UI login fails

- **Token login**: Verify you are entering the exact value of `web.auth.token`.
- **Password login**: Verify that `web.auth.admin_password` is set in the server config. If it is not set, only token login is available. Check that both `admin_username` and `admin_password` match what you are entering.

### Prometheus scraping returns 401

- If `web.metrics.token` is set, Prometheus must send it as a Bearer token. See [Prometheus Scrape Configuration](#prometheus-scrape-configuration).
- If `web.metrics.token` is not set but `web.auth.enabled` is `true`, Prometheus must send the master API token (`web.auth.token`) as a Bearer token.
- If both auth and metrics token are disabled, `/api/metrics` is open and no token is needed.

### Sessions lost on server restart

The server generates a random JWT secret at startup when `web.auth.jwt_secret` is not configured. Set a stable value to preserve sessions across restarts. See [JWT Session Persistence](#jwt-session-persistence).

### Health check returns 401

The `/api/health` endpoint is always public and never requires authentication. If you are receiving a 401, verify that you are requesting the correct URL path. Common mistakes include a missing `/api/` prefix or hitting a different endpoint.
