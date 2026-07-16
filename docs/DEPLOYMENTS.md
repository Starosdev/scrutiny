# Scrutiny Image Publishing

This repository owns the Scrutiny image publishing workflows.

For release-version verification details, see [RELEASE_VERSION_VERIFICATION.md](./RELEASE_VERSION_VERIFICATION.md).

## Environment Mapping

| Environment | Branch | Workflow | Published Image | Notes |
| --- | --- | --- | --- | --- |
| Testing | `develop` | `.github/workflows/deploy-testing.yml` | `ghcr.io/starosdev/scrutiny:develop` and `develop-omnibus` | External hosts pull these tags when they want the latest testing build |
| Beta | `beta` | `.github/workflows/deploy-beta.yml` | `ghcr.io/starosdev/scrutiny:beta` and `beta-omnibus` | External hosts pull these tags when they want a pre-release candidate ahead of stable |
| Production | `master` | `.github/workflows/release-and-deploy.yml` | `ghcr.io/starosdev/scrutiny:latest` and `latest-omnibus` | External hosts pull these tags when they want the latest production build |

## What The Workflows Do

- Check out the repo
- Normalize the GHCR image name to lowercase
- Build the omnibus image for `linux/amd64` and `linux/arm64`
- Build the default `collector-omnibus`, `web`, `collector-zfs`, `collector-mdadm`, `collector-btrfs`, and `collector-performance` images for `linux/amd64` and `linux/arm64`
- Build the base `collector` image for `linux/amd64`, `linux/arm64`, and `linux/arm/v7`
- Exclude `webapp/backend/pkg/version/version.go` from the Docker workflow path trigger so release-version sync commits do not rebuild images on their own
- Push the published tags to GHCR

They do not SSH to Zeus, join NetBird, or restart any remote stack.

## Manual Release Workflow

Production releases are created manually through `.github/workflows/release.yaml` via `workflow_dispatch`.

- Semantic versioning still comes from conventional commits and `semantic-release`.
- Raw release notes are generated deterministically from merged pull requests between the previous tag and the new tag.
- The generator uses merged PR metadata as the source of truth, renders note content from each PR's `## Summary` block plus linked issues, and validates that no extracted summary items were dropped before it emits notes.
- OpenAI polishing is optional and wording-only. If the polish step changes the entry structure or drops sub-bullets, the workflow falls back to the raw deterministic notes.

## Loop Pilot Workflows

This repo also ships non-deploy workflow automation for PR flow, issue triage, and dependency hygiene.

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| `.github/workflows/loop-pilot-triage.yaml` | Daily cron + manual | Read-only triage report covering open PRs, issues, and dependency hygiene candidates |
| `.github/workflows/loop-pilot-pr-babysitter.yaml` | Manual | Draft-only blocker analysis for one PR |
| `.github/workflows/loop-pilot-dependency-sweeper.yaml` | Manual | Draft-only dependency PR risk analysis for one target |

These workflows do not publish images, deploy environments, SSH to Zeus, or mutate PR state. They only generate markdown summaries and uploaded artifacts.

## Required GitHub Secrets

- `GITHUB_TOKEN`

The workflows use the built-in GitHub token to push images to `ghcr.io`.

## Host Rollout

Environment rollout is outside GitHub Actions.

If Zeus should move to a new image, do that from the host by pulling the published tags and restarting the compose project there. The current Zeus mapping is:

- develop image path: `ghcr.io/starosdev/scrutiny:develop-omnibus`
- beta image path: `ghcr.io/starosdev/scrutiny:beta-omnibus`
- production image path: `ghcr.io/starosdev/scrutiny:latest`
- develop compose project: `scrutiny-develop`
- beta compose project: `scrutiny-beta`
- production compose project: `scrutiny`
- develop port: `8780`
- beta port: `8680`
- production port: `8580`
- develop appdata root: `/mnt/user/appdata/scrutiny-develop`
- beta appdata root: `/mnt/user/appdata/scrutiny-beta`
- production appdata root: `/mnt/user/appdata/scrutiny`
- develop compose file: `/mnt/user/appdata/scrutiny-develop/docker-compose.yml`
- beta compose file: `/mnt/user/appdata/scrutiny-beta/docker-compose.yml`
- production compose file: `/mnt/user/appdata/scrutiny/docker-compose.yml`

## Current Zeus Host Layout

Zeus currently runs three separate appdata trees and compose projects side by side.

- Develop uses `/mnt/user/appdata/scrutiny-develop`
- Beta uses `/mnt/user/appdata/scrutiny-beta`
- Production uses `/mnt/user/appdata/scrutiny`

This repo now treats `beta` as an optional pre-release channel for changes that need validation before `master`.

- `develop` is the integration branch and testing image source
- `beta` is the optional pre-release branch and beta image source
- `master` is the stable branch and latest image source

That distinction matters for both manual host rollouts and the helper scripts in `ops/`:

- `ops/deploy-production.sh` should target `/mnt/user/appdata/scrutiny/docker-compose.yml` with compose project `scrutiny`
- `ops/deploy-testing.sh` should target `/mnt/user/appdata/scrutiny-develop/docker-compose.yml` with compose project `scrutiny-develop`

There is no repo-owned beta deploy helper today. Beta rollouts on Zeus are still host-side operations against `/mnt/user/appdata/scrutiny-beta/docker-compose.yml`.

If you point the develop deploy helper at `/mnt/user/appdata/scrutiny`, you will be operating on the production environment instead of the development environment.

The `deploy/` compose files in this repo remain available as repo-owned examples, but the Zeus helpers default to the live appdata-root compose files because those are what the host actually runs today.

### Zeus Develop Rebuild Preflight

- **Problem:** Unraid can renumber `/dev/sd*` devices. A stale compose `devices:` source prevents Docker from recreating the container and leaves it exited with an error such as `error gathering device information ... no such file or directory`.
- **Approach:** Compare every configured device source with the current `lsblk` output. Map a missing path to the same physical disk by model and serial before updating the develop compose file.
- **Dead end:** Re-running `docker compose up` without correcting a missing device path repeats the exit-128 failure; restarting the old container does not repair the compose configuration.
- **Rule:** Every device source in `/mnt/user/appdata/scrutiny-develop/docker-compose.yml` must exist before a forced recreate.

The deploy helper requires a checkout at `/mnt/user/appdata/scrutiny-develop/repo`. If that checkout is absent, use the live compose project directly:

```bash
ROOT=/mnt/user/appdata/scrutiny-develop

docker compose \
  -p scrutiny-develop \
  -f "$ROOT/docker-compose.yml" \
  --env-file "$ROOT/testing.env" \
  pull

docker compose \
  -p scrutiny-develop \
  -f "$ROOT/docker-compose.yml" \
  --env-file "$ROOT/testing.env" \
  up -d --force-recreate --remove-orphans
```

Before an embedded InfluxDB upgrade, back up the complete `influxdb` directory and follow the rollback guidance in [TROUBLESHOOTING_INFLUXDB.md](./TROUBLESHOOTING_INFLUXDB.md).

### Omnibus InfluxDB 2.9 Upgrade Preflight

The Omnibus image blocks InfluxDB 2.9.1 from starting when it detects existing data without a completed upgrade preflight. This prevents an unattended image update from migrating data before the operator has a rollback copy.

For an existing Docker Compose installation:

1. Stop Scrutiny.
2. Back up the complete host directory mounted at `/opt/scrutiny/influxdb`. With the example Compose file, copy `./influxdb` to storage outside the active mount.
3. Set `SCRUTINY_INFLUXDB_29_BACKUP_CONFIRMED=true` in the environment or `.env` file.
4. Start Scrutiny and confirm InfluxDB and the Scrutiny health endpoint are healthy.

For Unraid, stop the container and back up the complete Database path shown in the template. Then change **InfluxDB 2.9 Backup Confirmed** to `true` before starting the updated container.

Fresh installations start without acknowledgement because no existing InfluxDB data is present. After a fresh start or confirmed upgrade, Scrutiny writes a persistent preflight marker in the InfluxDB data directory, so later restarts do not require the variable.

If startup is blocked, the container log identifies the mounted data path and the exact acknowledgement variable. Do not set the variable until the backup is complete.

## Zeus MDADM Testing Notes

For actual deployment and troubleshooting steps, see [MDADM_MONITORING.md](./MDADM_MONITORING.md).

Zeus is a valid host for MDADM deploy-path verification:

- pull and restart the `develop-omnibus` image there
- bind-mount `/proc/mdstat` into the container as `/host/proc/mdstat`
- verify authenticated API routes such as `POST /api/collectors/run` and `GET /api/mdadm/summary`

Zeus is not a reliable host for end-to-end MDADM array-ingestion validation.

- Zeus runs Unraid
- Unraid's `/proc/mdstat` content does not match the standard Linux `mdadm` array-line format that the current detector parses
- the collector can see `/host/proc/mdstat` and still report `No MDADM arrays found`

Use Zeus to verify image rollout, auth, route availability, and container mount wiring.
Use a standard Linux host with real `mdadm` arrays to validate actual MDADM discovery and summary population.
