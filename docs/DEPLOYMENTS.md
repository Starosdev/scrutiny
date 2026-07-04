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
- Build the default `web`, `collector-zfs`, `collector-mdadm`, `collector-btrfs`, and `collector-performance` images for `linux/amd64` and `linux/arm64`
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

If Zeus should move to a new image, do that from the host by pulling the published tags and restarting the compose project there. The current Zeus mapping is still:

- develop image path: `ghcr.io/starosdev/scrutiny:develop-omnibus`
- beta image path: `ghcr.io/starosdev/scrutiny:beta-omnibus`
- production image path: `ghcr.io/starosdev/scrutiny:latest`
- develop port: `8680`
- beta port: choose a separate host port for side-by-side validation if you run it alongside testing or production
- production port: `8580`
- production appdata root: `/mnt/user/appdata/scrutiny`
- Zeus testing appdata root: `/mnt/user/appdata/scrutiny-dev`
- production compose file: `/mnt/user/appdata/scrutiny/docker-compose.yml`
- testing compose file: `/mnt/user/appdata/scrutiny-dev/docker-compose.yml`

## Current Zeus Host Layout

Zeus does not currently run testing and production from the same appdata tree.

- Production uses `/mnt/user/appdata/scrutiny`
- Testing uses `/mnt/user/appdata/scrutiny-dev`

This repo now treats `beta` as an optional pre-release channel for changes that need validation before `master`.

- `develop` is the integration branch and testing image source
- `beta` is the optional pre-release branch and beta image source
- `master` is the stable branch and latest image source

That distinction matters for both manual host rollouts and the helper scripts in `ops/`:

- `ops/deploy-production.sh` should target `/mnt/user/appdata/scrutiny/docker-compose.yml` with compose project `scrutiny`
- `ops/deploy-testing.sh` should target `/mnt/user/appdata/scrutiny-dev/docker-compose.yml` with compose project `scrutiny-dev`

If you point the testing deploy helper at `/mnt/user/appdata/scrutiny`, you will be operating on the production environment instead of Zeus testing.

The `deploy/` compose files in this repo remain available as repo-owned examples, but the Zeus helpers default to the live appdata-root compose files because those are what the host actually runs today.

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
