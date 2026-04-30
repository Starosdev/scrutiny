# Testing Guide

This document covers how to test Scrutiny changes end-to-end, from unit tests through
full-stack Docker validation. It is written for contributors preparing a PR back to the
main repository.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Reference](#quick-reference)
- [1. Unit Tests](#1-unit-tests)
  - [Backend (Go)](#backend-go)
  - [Frontend (Angular)](#frontend-angular)
- [2. Linting](#2-linting)
- [3. Building Docker Images Locally](#3-building-docker-images-locally)
- [4. Full-Stack Docker Testing](#4-full-stack-docker-testing)
  - [Omnibus Image (Simplest)](#omnibus-image-simplest)
  - [Hub/Spoke Images](#hubspoke-images)
- [5. Populating Test Data](#5-populating-test-data)
- [6. Manual API Testing](#6-manual-api-testing)
- [7. Frontend Verification](#7-frontend-verification)
- [8. Testing Specific Features](#8-testing-specific-features)
  - [ZFS Pool Monitoring](#zfs-pool-monitoring)
  - [Performance Benchmarking](#performance-benchmarking)
  - [Notifications](#notifications)
  - [Authentication](#authentication)
  - [MQTT / Home Assistant](#mqtt--home-assistant)
  - [Prometheus Metrics](#prometheus-metrics)
  - [SMART Attribute Overrides](#smart-attribute-overrides)
- [9. Pre-PR Checklist](#9-pre-pr-checklist)
- [10. CI Expectations](#10-ci-expectations)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Backend compilation and tests |
| Node.js | 22+ (LTS) | Frontend build and tests |
| Docker | 20+ | Container builds and full-stack testing |
| Docker Compose | v2+ | Multi-container orchestration |
| smartmontools | any | Required inside collector containers |
| curl | any | API testing |

> **Note**: The Go module is `go 1.25.0` and the Dockerfiles use `golang:1.26-trixie`.
> The frontend uses Angular 21 with TypeScript ~5.9.

---

## Quick Reference

```bash
# Run all backend tests
docker run -p 8086:8086 -d --rm influxdb:2.2   # InfluxDB required
go test ./...

# Run all frontend tests
cd webapp/frontend && npm ci --legacy-peer-deps && npm test -- --watch=false

# Lint backend
golangci-lint run ./...

# Lint frontend
cd webapp/frontend && npm run lint

# Build omnibus Docker image
docker build -f docker/Dockerfile -t scrutiny:local .

# Build and run full stack
docker build -f docker/Dockerfile -t scrutiny:local .
docker run -p 8080:8080 -p 8086:8086 scrutiny:local

# Populate test data
go run webapp/backend/pkg/models/testdata/helper.go
```

---

## 1. Unit Tests

### Backend (Go)

Backend tests require a running InfluxDB instance. Start one before running tests:

```bash
# Start InfluxDB with test credentials
docker run -p 8086:8086 -d --rm \
  --name scrutiny-test-influxdb \
  -e DOCKER_INFLUXDB_INIT_MODE=setup \
  -e DOCKER_INFLUXDB_INIT_USERNAME=admin \
  -e DOCKER_INFLUXDB_INIT_PASSWORD=password12345 \
  -e DOCKER_INFLUXDB_INIT_ORG=scrutiny \
  -e DOCKER_INFLUXDB_INIT_BUCKET=metrics \
  -e DOCKER_INFLUXDB_INIT_ADMIN_TOKEN=my-super-secret-auth-token \
  influxdb:2.2
```

Then run tests:

```bash
# All tests
go test ./...

# Verbose output
go test -v ./...

# Specific package
go test -v ./webapp/backend/pkg/web/...
go test -v ./webapp/backend/pkg/database/...
go test -v ./webapp/backend/pkg/models/measurements/...
go test -v ./collector/pkg/...

# Specific test by name
go test -v -run TestFunctionName ./path/to/pkg/

# With coverage report
go test -coverprofile=coverage.txt -covermode=atomic ./...
go tool cover -html=coverage.txt    # View in browser
```

Clean up when done:

```bash
docker stop scrutiny-test-influxdb
```

### Frontend (Angular)

```bash
cd webapp/frontend

# Install dependencies (--legacy-peer-deps required due to Angular Material version pinning)
npm ci --legacy-peer-deps

# Run tests with watch mode (interactive development)
npm test

# Run tests once (CI mode)
npm test -- --watch=false

# Run tests headless (no browser window)
npm test -- --watch=false --browsers=ChromeHeadless

# Run with coverage
npx ng test --watch=false --code-coverage
# Coverage report generated in webapp/frontend/coverage/
```

---

## 2. Linting

### Backend (Go)

The project uses [golangci-lint](https://golangci-lint.run/) v2 with the config in
`.golangci.yml`. Enabled linters: `gocritic`, `gosec`, `misspell`, `nolintlint`.

```bash
# Install golangci-lint (if not already installed)
# See: https://golangci-lint.run/welcome/install/

# Run linting
golangci-lint run ./...

# Run with vendor mode (matches CI)
go mod vendor
golangci-lint run ./...
```

### Frontend (Angular/TypeScript)

```bash
cd webapp/frontend
npm run lint          # ESLint + Angular linting rules
npm run lint:fix      # Auto-fix what it can
```

### Code Style Rules (from CONTRIBUTING.md)

- No emojis in code, commits, comments, or documentation
- Follow existing code patterns and formatting
- Commit messages: `type(scope): description` (conventional commits)

---

## 3. Building Docker Images Locally

The project produces five Docker images. Build whichever you need to test:

```bash
# Omnibus (web + all collectors + InfluxDB, all-in-one)
make docker-omnibus
# -> ghcr.io/starosdev/scrutiny-dev:omnibus

# Web server only (API + frontend)
make docker-web
# -> ghcr.io/starosdev/scrutiny-dev:web

# SMART collector only
make docker-collector
# -> ghcr.io/starosdev/scrutiny-dev:collector

# ZFS collector only
make docker-collector-zfs
# -> ghcr.io/starosdev/scrutiny-dev:collector-zfs

# Performance collector only
make docker-collector-performance
# -> ghcr.io/starosdev/scrutiny-dev:collector-performance
```

Each `make docker-*` target compiles Go binaries inside a multi-stage Docker build
(including the Angular frontend for web/omnibus), so you do **not** need Go or Node
installed locally to build Docker images.

> **Tip**: For faster iteration on backend-only changes, build the binary directly:
> ```bash
> go mod vendor
> make binary-web
> ```
> Then test with a local InfluxDB container instead of doing a full Docker rebuild.

---

## 4. Full-Stack Docker Testing

This is the primary way to validate changes as they will run in production.

### Omnibus Image (Simplest)

Build and run the omnibus image which bundles everything:

```bash
# Build
docker build -f docker/Dockerfile -t scrutiny:local .

# Run (you MUST pass real block devices for collector to find drives)
docker run -it --rm \
  -p 8080:8080 \
  -p 8086:8086 \
  -v /run/udev:/run/udev:ro \
  --cap-add SYS_RAWIO \
  --cap-add SYS_ADMIN \
  --device=/dev/sda \
  scrutiny:local
```

Open http://localhost:8080 and verify:
1. The dashboard loads without errors
2. Navigation works (Dashboard, ZFS, Workload, Settings)
3. The collector runs and populates drives (check container logs)

**Trigger a manual collection run** (don't wait for cron):

```bash
docker exec <container_id> scrutiny-collector-metrics run
```

### Hub/Spoke Images

For testing changes to individual components in isolation:

```bash
# 1. Start InfluxDB
docker run -d --name influxdb \
  -p 8086:8086 \
  influxdb:2.2

# 2. Build and start the web server
docker build -f docker/Dockerfile.web -t scrutiny-web:local .
docker run -d --name scrutiny-web \
  -p 8080:8080 \
  -e SCRUTINY_WEB_INFLUXDB_HOST=host.docker.internal \
  scrutiny-web:local

# 3. Build and start the collector
docker build -f docker/Dockerfile.collector -t scrutiny-collector:local .
docker run --rm \
  -v /run/udev:/run/udev:ro \
  --cap-add SYS_RAWIO \
  --cap-add SYS_ADMIN \
  --device=/dev/sda \
  -e COLLECTOR_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_RUN_STARTUP=true \
  scrutiny-collector:local
```

> **Note**: Use `host.docker.internal` on Docker Desktop. On Linux, use `--network host`
> or create a Docker network and use container names.

### Using Docker Compose

For a more production-like test, use the provided compose files:

```bash
# Create a test compose override pointing to your local images
cat > docker-compose.test.yaml << 'EOF'
services:
  influxdb:
    image: influxdb:2.2
    ports:
      - '8086:8086'
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8086/health"]
      interval: 5s
      timeout: 10s
      retries: 20

  web:
    image: scrutiny-web:local
    ports:
      - '8080:8080'
    environment:
      SCRUTINY_WEB_INFLUXDB_HOST: 'influxdb'
      SCRUTINY_LOG_LEVEL: 'DEBUG'
    depends_on:
      influxdb:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
      interval: 5s
      timeout: 10s
      retries: 20
      start_period: 10s
EOF

docker compose -f docker-compose.test.yaml up
```

---

## 5. Populating Test Data

Once the web server and InfluxDB are running (either via Docker or locally), you can
populate the database with synthetic SMART data. This is essential for testing frontend
changes without real drives.

### Using the Test Data Helper (Recommended)

The helper script registers mock devices and submits 30 days of historical SMART data:

```bash
# Server must be running on localhost:8080
go run webapp/backend/pkg/models/testdata/helper.go
```

This registers 5 test devices (ATA, NVMe, SCSI) and submits SMART data with timestamps
adjusted to be within InfluxDB's retention window.

### Manual API Calls

For targeted testing, submit data manually:

```bash
# Register devices
curl -X POST -H "Content-Type: application/json" \
  -d @webapp/backend/pkg/web/testdata/register-devices-req.json \
  http://localhost:8080/api/devices/register

# Submit SMART data for a specific device (ATA)
curl -X POST -H "Content-Type: application/json" \
  -d @webapp/backend/pkg/models/testdata/smart-ata.json \
  http://localhost:8080/api/device/0x5000cca264eb01d7/smart

# Submit NVMe SMART data
curl -X POST -H "Content-Type: application/json" \
  -d @webapp/backend/pkg/models/testdata/smart-nvme.json \
  http://localhost:8080/api/device/0x5002538e40a22954/smart

# Submit SCSI SMART data
curl -X POST -H "Content-Type: application/json" \
  -d @webapp/backend/pkg/models/testdata/smart-scsi.json \
  http://localhost:8080/api/device/0x5000cca252c859cc/smart

# Submit failing device data
curl -X POST -H "Content-Type: application/json" \
  -d @webapp/backend/pkg/models/testdata/smart-fail2.json \
  http://localhost:8080/api/device/0x5000cca264ec3183/smart
```

### Available Test Data Files

| File | Protocol | Description |
|------|----------|-------------|
| `smart-ata.json` | ATA | Standard healthy ATA drive |
| `smart-ata-date.json` | ATA | ATA drive with different timestamp |
| `smart-ata-date2.json` | ATA | ATA drive with another timestamp |
| `smart-ata-full.json` | ATA | ATA drive with all attributes populated |
| `smart-ata-farm.json` | ATA | ATA drive with Seagate FARM data |
| `smart-ata-failed-scrutiny.json` | ATA | ATA drive with Scrutiny threshold failure |
| `smart-fail2.json` | ATA | ATA drive with SMART failure |
| `smart-nvme.json` | NVMe | Standard healthy NVMe drive |
| `smart-nvme-failed.json` | NVMe | NVMe drive with failures |
| `smart-scsi.json` | SCSI | Standard SCSI drive |
| `smart-scsi2.json` | SCSI | SCSI drive (variant) |
| `smart-scsi-failed.json` | SCSI | SCSI drive with failures |
| `smart-scsi-sas-env-temp.json` | SCSI | SCSI SAS drive with environment temp |
| `smart-megaraid0.json` | ATA | MegaRAID virtual disk |
| `smart-sat.json` | ATA | SAT (SCSI-to-ATA Translation) device |

> **Important**: InfluxDB data older than the retention period (default 15 days for raw)
> will be silently discarded. The helper script handles this by adjusting timestamps. If
> submitting manually, ensure `local_time.time_t` in the JSON is within the last 2 weeks.

---

## 6. Manual API Testing

Key API endpoints to verify after changes:

```bash
BASE=http://localhost:8080

# Health check (should return 200)
curl -s $BASE/api/health | jq .

# Dashboard summary (all devices with status)
curl -s $BASE/api/summary | jq .

# Temperature history
curl -s "$BASE/api/summary/temp?duration_key=week" | jq .

# Device details (replace with actual device ID or WWN)
curl -s $BASE/api/device/0x5000cca264eb01d7/details | jq .

# Workload insights
curl -s "$BASE/api/summary/workload?duration_key=week" | jq .

# Settings
curl -s $BASE/api/settings | jq .

# Attribute overrides
curl -s $BASE/api/settings/overrides | jq .

# Notification URLs
curl -s $BASE/api/settings/notify-urls | jq .

# Test notifications
curl -X POST $BASE/api/health/notify

# Prometheus metrics (if enabled)
curl -s $BASE/api/metrics
```

### Testing with Authentication Enabled

```bash
# Enable auth via environment
docker run ... \
  -e SCRUTINY_WEB_AUTH_ENABLED=true \
  -e SCRUTINY_WEB_AUTH_TOKEN=test-token \
  -e SCRUTINY_WEB_AUTH_ADMIN_PASSWORD=testpass \
  scrutiny:local

# Login
TOKEN=$(curl -s -X POST $BASE/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"testpass"}' | jq -r '.token')

# Use token for authenticated requests
curl -s -H "Authorization: Bearer $TOKEN" $BASE/api/summary | jq .

# Collector auth (uses API token, not JWT)
curl -X POST -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d @webapp/backend/pkg/web/testdata/register-devices-req.json \
  $BASE/api/devices/register
```

---

## 7. Frontend Verification

### Development Mode (Mock Data)

When running the frontend without a backend, API calls return mock data from
`webapp/frontend/src/app/data/mock/`. This is controlled by the Angular environment:

- `environment.ts` (`production: false`) -- mock data interceptors active
- `environment.prod.ts` (`production: true`) -- real API calls

```bash
cd webapp/frontend
npm install --legacy-peer-deps
npm run start -- --serve-path="/web/" --port 4200
# Open http://localhost:4200/web
```

### Production Mode (Against Real Backend)

To test frontend changes against a real backend:

```bash
cd webapp/frontend
npm install --legacy-peer-deps
npm run build:prod -- --watch --output-path=../../dist

# In another terminal, start the backend:
go run webapp/backend/cmd/scrutiny/scrutiny.go start --config ./scrutiny.yaml
# Open http://localhost:8080/web
```

### Frontend Verification Checklist

- [ ] Dashboard loads and displays device cards with status indicators
- [ ] Device detail page shows SMART attribute table with history charts
- [ ] Temperature history chart renders on dashboard
- [ ] ZFS pools page loads (even if empty)
- [ ] Workload page loads (even if empty)
- [ ] Settings page opens, changes save and persist across reload
- [ ] Notification URL management (add, edit, test, delete)
- [ ] SMART attribute overrides (add, delete, verify status recalculation)
- [ ] Mobile layout renders correctly (resize browser below 960px width)
- [ ] Top navigation links all work
- [ ] No console errors in browser DevTools

---

## 8. Testing Specific Features

### ZFS Pool Monitoring

```bash
# Build and run ZFS collector image
docker build -f docker/Dockerfile.collector-zfs -t scrutiny-collector-zfs:local . && \
docker run --rm \
  -e COLLECTOR_ZFS_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_ZFS_RUN_STARTUP=true \
  scrutiny-collector-zfs:local

# Verify pools appear
curl -s http://localhost:8080/api/zfs/summary | jq .
```

### MDADM Software RAID Monitoring

```bash
# Build and run MDADM collector image (requires /dev mapping and SYS_ADMIN)
docker build -f docker/Dockerfile.collector-mdadm -t scrutiny-collector-mdadm:local . && \
docker run --rm \
  --cap-add SYS_ADMIN \
  -v /dev:/dev \
  -e COLLECTOR_MDADM_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_MDADM_RUN_STARTUP=true \
  scrutiny-collector-mdadm:local

# Verify arrays appear
curl -s http://localhost:8080/api/mdadm/summary | jq .
```

### Performance Benchmarking

```bash
# Build and run performance collector image for a quick benchmark
docker build -f docker/Dockerfile.collector-performance -t scrutiny-collector-performance:local . && \
docker run --rm \
  --cap-add SYS_RAWIO --cap-add SYS_ADMIN \
  --device=/dev/sda \
  -e COLLECTOR_PERF_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_PERF_RUN_STARTUP=true \
  -e COLLECTOR_PERF_PROFILE=quick \
  scrutiny-collector-performance:local

# Verify results
curl -s http://localhost:8080/api/device/<device_id>/performance | jq .
```

### Notifications

```bash
# Test notification delivery (sends a test message to all configured URLs)
curl -X POST http://localhost:8080/api/health/notify

# Test a specific notification URL
curl -X POST http://localhost:8080/api/settings/notify-urls/1/test

# Verify missed ping detection (check diagnostic status)
curl -s http://localhost:8080/api/health/missed-ping-status | jq .
```

### Authentication

```bash
# Check auth status (works whether auth is enabled or not)
curl -s http://localhost:8080/api/auth/status | jq .
# Returns: {"enabled": false} or {"enabled": true, "authenticated": false}

# When auth is enabled, unauthenticated API calls should return 401:
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/summary
# Should return: 401
```

### MQTT / Home Assistant

```bash
# Start a test MQTT broker
docker run -d --name mosquitto -p 1883:1883 eclipse-mosquitto:2 \
  mosquitto -c /mosquitto-no-auth.conf

# Run Scrutiny with MQTT enabled
docker run ... \
  -e SCRUTINY_WEB_MQTT_ENABLED=true \
  -e SCRUTINY_WEB_MQTT_BROKER=tcp://host.docker.internal:1883 \
  scrutiny:local

# Subscribe to MQTT topics to verify discovery messages
docker exec mosquitto mosquitto_sub -t 'homeassistant/#' -v

# Force a re-sync
curl -X POST http://localhost:8080/api/health/mqtt-sync
```

### Prometheus Metrics

```bash
# Metrics are enabled by default; verify the endpoint returns data
curl -s http://localhost:8080/api/metrics | head -20

# With a metrics token configured:
curl -s -H "Authorization: Bearer your-metrics-token" \
  http://localhost:8080/api/metrics
```

### SMART Attribute Overrides

```bash
# List current overrides
curl -s http://localhost:8080/api/settings/overrides | jq .

# Add an override (ignore ATA attribute 199 globally)
curl -X POST http://localhost:8080/api/settings/overrides \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "ATA",
    "attribute_id": "199",
    "action": "ignore"
  }'

# Add a device-specific override (force attribute 5 to failed)
curl -X POST http://localhost:8080/api/settings/overrides \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "ATA",
    "attribute_id": "5",
    "action": "force_status",
    "status": "failed",
    "device_wwn": "0x5000cca264eb01d7"
  }'

# Verify: re-submit SMART data and check that device status reflects the override
curl -X POST -H "Content-Type: application/json" \
  -d @webapp/backend/pkg/models/testdata/smart-ata.json \
  http://localhost:8080/api/device/0x5000cca264eb01d7/smart

curl -s http://localhost:8080/api/device/0x5000cca264eb01d7/details | jq '.data.device.device_status'

# Delete an override
curl -X DELETE http://localhost:8080/api/settings/overrides/1
```

---

## 9. Pre-PR Checklist

Before submitting a pull request, ensure all of the following pass:

### Code Quality

- [ ] `go test ./...` -- all backend tests pass (with InfluxDB running)
- [ ] `golangci-lint run ./...` -- no lint errors
- [ ] `cd webapp/frontend && npm run lint` -- no frontend lint errors
- [ ] `cd webapp/frontend && npm test -- --watch=false` -- all frontend tests pass

### Build Verification

- [ ] `make binary-all` -- all four Go binaries compile
- [ ] `docker build -f docker/Dockerfile .` -- omnibus image builds successfully
- [ ] If you changed collector code: `docker build -f docker/Dockerfile.collector .`
- [ ] If you changed web/API code: `docker build -f docker/Dockerfile.web .`
- [ ] If you changed ZFS collector: `docker build -f docker/Dockerfile.collector-zfs .`
- [ ] If you changed perf collector: `docker build -f docker/Dockerfile.collector-performance .`

### Full-Stack Smoke Test

- [ ] Start the omnibus container (or hub/spoke setup)
- [ ] Populate test data with `helper.go` or trigger a real collector run
- [ ] Dashboard loads and shows devices
- [ ] Device detail page renders without errors
- [ ] Settings page loads and saves
- [ ] No new console errors in browser DevTools

### Contribution Standards (from CONTRIBUTING.md)

- [ ] Branch created from `develop` (not `master`)
- [ ] Branch naming: `feature/SCR-{id}-description` or `fix/SCR-{id}-description`
- [ ] Commit messages follow `type(scope): description` convention
- [ ] No emojis in code, commits, comments, or documentation
- [ ] PR targets `develop` branch (or `master` for hotfixes only)
- [ ] Tests added for new functionality
- [ ] Existing tests updated if behavior changed

### If You Added a Database Migration

- [ ] Migration has a unique timestamp directory in `webapp/backend/pkg/database/migrations/`
- [ ] Migration is registered in `scrutiny_repository_migrations.go`
- [ ] Tested with a fresh database (delete `scrutiny.db` and restart)
- [ ] Tested with an existing database (migration runs without errors on upgrade)

### If You Added a New API Endpoint

- [ ] Route registered in `webapp/backend/pkg/web/server.go`
- [ ] Handler file created in `webapp/backend/pkg/web/handler/`
- [ ] `DeviceRepo` interface updated if new data access is needed
- [ ] Implementation added to the appropriate `scrutiny_repository_*.go` file
- [ ] Frontend service updated to call the new endpoint (if applicable)

---

## 10. CI Expectations

The GitHub Actions CI pipeline (`.github/workflows/`) runs the following on every PR:

1. **Go tests** (`go test ./...`) with a sidecar InfluxDB container
2. **Go lint** (`golangci-lint run ./...`)
3. **Frontend tests** (`npm test -- --watch=false --browsers=ChromeHeadless`)
4. **Docker build** (at minimum the omnibus image)

To avoid CI surprises, always run the following locally before pushing:

```bash
# Backend
docker run -p 8086:8086 -d --rm \
  -e DOCKER_INFLUXDB_INIT_MODE=setup \
  -e DOCKER_INFLUXDB_INIT_USERNAME=admin \
  -e DOCKER_INFLUXDB_INIT_PASSWORD=password12345 \
  -e DOCKER_INFLUXDB_INIT_ORG=scrutiny \
  -e DOCKER_INFLUXDB_INIT_BUCKET=metrics \
  -e DOCKER_INFLUXDB_INIT_ADMIN_TOKEN=my-super-secret-auth-token \
  influxdb:2.2

go mod vendor
go test ./...
golangci-lint run ./...

# Frontend
cd webapp/frontend
npm ci --legacy-peer-deps
npm run lint
npm test -- --watch=false --browsers=ChromeHeadless
```

---

## Troubleshooting

### InfluxDB Connection Errors in Tests

If tests fail with InfluxDB connection errors:

```bash
# Verify InfluxDB is running and healthy
curl -s http://localhost:8086/health | jq .

# Check it was initialized with the correct credentials
curl -s -H "Authorization: Token my-super-secret-auth-token" \
  http://localhost:8086/api/v2/buckets | jq '.buckets[].name'
```

### Docker Build Fails on Frontend

The omnibus and web Dockerfiles build the Angular frontend inside the container.
If the Node build stage fails:

```bash
# Verify the frontend builds locally first
cd webapp/frontend
npm ci --legacy-peer-deps
npm run build:prod
```

Common issues:
- TypeScript errors that only surface in `--configuration production` mode
  (stricter checks than dev mode)
- Memory issues: the build may need `--max_old_space_size=6144`

### "No Devices Found" After Collector Run

- Ensure you passed `--device=/dev/sdX` to the Docker container
- Ensure `--cap-add SYS_RAWIO` (and `SYS_ADMIN` for NVMe) are set
- On AppArmor systems (Ubuntu, Debian, TrueNAS SCALE): either load the custom
  profile from `docker/apparmor-profile` or use `--security-opt apparmor=unconfined`
- Check collector logs: `docker logs <collector_container>`

### Test Data Timestamps Too Old

InfluxDB discards data outside the retention window. If `helper.go` data doesn't appear:

```bash
# The helper adjusts timestamps automatically, but if you're using manual curl:
# Update the local_time.time_t field in the JSON to a recent Unix timestamp
date +%s   # Current Unix timestamp
```

### Frontend Shows Mock Data Instead of Real Data

If running `npm run start` (dev mode), mock interceptors are active. To use real API data:
- Build with `npm run build:prod` and serve from the Go backend, OR
- Proxy API calls by configuring Angular's `proxy.conf.json` to forward `/api/*` to
  `http://localhost:8080`

### Port Conflicts

Before starting Scrutiny containers, it's recommended to check which ports are currently in use on your development machine to avoid startup failures.

You can check if Scrutiny's required ports are active using either `ss` or `lsof`:

```bash
# Using ss (preferred on modern Linux)
ss -tuln | grep -E ':(8080|8086|4200|1883)'

# Or using lsof (works on Linux and macOS)
sudo lsof -i -P -n | grep LISTEN | grep -E '8080|8086|4200|1883'
```

Default ports used by Scrutiny:
- `8080` -- Web server / API
- `8086` -- InfluxDB
- `4200` -- Angular dev server (if running frontend separately)
- `1883` -- MQTT broker (if testing HA integration)

If these conflict with existing services on your host, adjust the port mappings on the fly:

**For Docker run:**
Adjust the `-p <host-port>:<container-port>` mapping to use free ports on your host.
```bash
docker run -p 9090:8080 -p 9086:8086 scrutiny:local
```

**For internal configuration overrides:**
If you need to change the internal ports the applications bind to, override via environment variables:

```bash
# Change web server internal port
-e SCRUTINY_WEB_LISTEN_PORT=9090

# Change InfluxDB internal port
-e SCRUTINY_WEB_INFLUXDB_PORT=9086
```

---

## 11. Appendix: Combined Docker Build & Run Commands

For quick copy-pasting, here are all the combinations to freshly build a Docker image from your local code and immediately run it.

### Omnibus (All-in-one)

```bash
docker build -f docker/Dockerfile -t scrutiny:local . && \
docker run -it --rm \
  -p 8080:8080 \
  -p 8086:8086 \
  -v /run/udev:/run/udev:ro \
  --cap-add SYS_RAWIO \
  --cap-add SYS_ADMIN \
  --device=/dev/sda \
  scrutiny:local
```

### Hub/Spoke Web Server

```bash
docker build -f docker/Dockerfile.web -t scrutiny-web:local . && \
docker run -d --rm --name scrutiny-web \
  -p 8080:8080 \
  -e SCRUTINY_WEB_INFLUXDB_HOST=host.docker.internal \
  scrutiny-web:local
```

### Core SMART Collector

```bash
docker build -f docker/Dockerfile.collector -t scrutiny-collector:local . && \
docker run --rm \
  -v /run/udev:/run/udev:ro \
  --cap-add SYS_RAWIO \
  --cap-add SYS_ADMIN \
  --device=/dev/sda \
  -e COLLECTOR_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_RUN_STARTUP=true \
  scrutiny-collector:local
```

### ZFS Policy Collector

```bash
docker build -f docker/Dockerfile.collector-zfs -t scrutiny-collector-zfs:local . && \
docker run --rm \
  -e COLLECTOR_ZFS_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_ZFS_RUN_STARTUP=true \
  scrutiny-collector-zfs:local
```

### MDADM Software RAID Collector

```bash
docker build -f docker/Dockerfile.collector-mdadm -t scrutiny-collector-mdadm:local . && \
docker run --rm \
  --cap-add SYS_ADMIN \
  -v /dev:/dev \
  -e COLLECTOR_MDADM_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_MDADM_RUN_STARTUP=true \
  scrutiny-collector-mdadm:local
```

### Performance Collector

```bash
docker build -f docker/Dockerfile.collector-performance -t scrutiny-collector-performance:local . && \
docker run --rm \
  --cap-add SYS_RAWIO --cap-add SYS_ADMIN \
  --device=/dev/sda \
  -e COLLECTOR_PERF_API_ENDPOINT=http://host.docker.internal:8080 \
  -e COLLECTOR_PERF_RUN_STARTUP=true \
  -e COLLECTOR_PERF_PROFILE=quick \
  scrutiny-collector-performance:local
```
