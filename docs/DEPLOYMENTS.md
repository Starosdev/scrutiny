# Scrutiny Deployments

This repository owns the Scrutiny deployment definitions and rollout workflows.

## Environment Mapping

| Environment | Branch | Workflow | Default Image | Compose File |
| --- | --- | --- | --- | --- |
| Testing | `develop` | `.github/workflows/deploy-testing.yml` | `ghcr.io/starosdev/scrutiny:develop-omnibus` | `deploy/testing/docker-compose.yml` |
| Production | `master` | `.github/workflows/release-and-deploy.yml` | `ghcr.io/starosdev/scrutiny:latest` | `deploy/production/docker-compose.yml` |

## Remote Host Layout

Both workflows assume the remote host keeps a checked-out Scrutiny repo at:

```text
/mnt/user/appdata/scrutiny/repo
```

And environment files at:

```text
/mnt/user/appdata/scrutiny/testing.env
/mnt/user/appdata/scrutiny/production.env
```

This matches the current Zeus layout:

- production container: `scrutiny` on `8580`
- develop container: `scrutiny-dev` on `8680`
- appdata root: `/mnt/user/appdata/scrutiny`
- no reverse proxy is configured for Scrutiny on Zeus right now

The deploy scripts reset the remote checkout to the target branch, pull the declared image, restart the compose project, and run smoke tests from the host itself against the configured `SCRUTINY_BASE_URL`.

## Required GitHub Secrets

### Testing

- `NETBIRD_SETUP_KEY`
- `SCRUTINY_TESTING_HOST`
- `SCRUTINY_TESTING_USER`
- `SCRUTINY_TESTING_SSH_KEY`

### Production

- `NETBIRD_SETUP_KEY`
- `SCRUTINY_PRODUCTION_HOST`
- `SCRUTINY_PRODUCTION_USER`
- `SCRUTINY_PRODUCTION_SSH_KEY`

Production should also use the GitHub `production` environment with required reviewers.

## NetBird Transport

These workflows assume GitHub-hosted runners join the NetBird mesh before SSH.

- Zeus NetBird IP: `100.66.106.240`
- The workflow installs NetBird on the runner, starts the service, and runs `netbird up --setup-key ...`
- The setup key should be created in NetBird with ephemeral peers enabled
- Limit the setup key to the smallest practical scope and usage count

Without a valid `NETBIRD_SETUP_KEY`, the workflows cannot reach Zeus.

## URL Reality

Scrutiny is currently accessed on Zeus by direct port bindings, not by reverse-proxied hostnames.

- production default: `http://127.0.0.1:8580`
- develop default: `http://127.0.0.1:8680`

If you later add a reverse proxy, update `SCRUTINY_BASE_URL` in the corresponding env file.

## First-Time Host Setup

1. Clone this repo to `/mnt/user/appdata/scrutiny/repo` on Zeus.
2. Copy the matching `.env.example` file and fill in the host-specific values:
   - `deploy/testing/.env.example` -> `/mnt/user/appdata/scrutiny/testing.env`
   - `deploy/production/.env.example` -> `/mnt/user/appdata/scrutiny/production.env`
3. Create the config and data directories referenced by the env file.
4. Run the deploy script once manually on the host:

```bash
bash /mnt/user/appdata/scrutiny/repo/ops/deploy-testing.sh
bash /mnt/user/appdata/scrutiny/repo/ops/deploy-production.sh
```

## Verification

Both deploy scripts call `ops/smoke_test.sh` against the configured base URL and expect:

- `GET /api/health` -> `200`
- `GET /` -> `200`
