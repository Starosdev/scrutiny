# Scrutiny Image Publishing

This repository owns the Scrutiny image publishing workflows.

For release-version verification details, see [RELEASE_VERSION_VERIFICATION.md](./RELEASE_VERSION_VERIFICATION.md).

## Environment Mapping

| Environment | Branch | Workflow | Published Image | Notes |
| --- | --- | --- | --- | --- |
| Testing | `develop` | `.github/workflows/deploy-testing.yml` | `ghcr.io/starosdev/scrutiny:develop` and `develop-omnibus` | External hosts pull these tags when they want the latest testing build |
| Production | `master` | `.github/workflows/release-and-deploy.yml` | `ghcr.io/starosdev/scrutiny:latest` and `latest-omnibus` | External hosts pull these tags when they want the latest production build |

## What The Workflows Do

- Check out the repo
- Normalize the GHCR image name to lowercase
- Build the omnibus image for `linux/amd64` and `linux/arm64`
- Build the default `web` and `collector-performance` images for `linux/amd64` and `linux/arm64`
- Exclude `webapp/backend/pkg/version/version.go` from the Docker workflow path trigger so release-version sync commits do not rebuild images on their own
- Push the published tags to GHCR

They do not SSH to Zeus, join NetBird, or restart any remote stack.

## Required GitHub Secrets

- `GITHUB_TOKEN`

The workflows use the built-in GitHub token to push images to `ghcr.io`.

## Host Rollout

Environment rollout is outside GitHub Actions.

If Zeus should move to a new image, do that from the host by pulling the published tags and restarting the compose project there. The current Zeus mapping is still:

- develop image path: `ghcr.io/starosdev/scrutiny:develop-omnibus`
- production image path: `ghcr.io/starosdev/scrutiny:latest`
- develop port: `8680`
- production port: `8580`

The `deploy/` and `ops/` files in this repo remain available if you want repo-owned host scripts, but they are not invoked by GitHub Actions anymore.
