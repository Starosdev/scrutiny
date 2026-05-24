# Docker Image Channels

> TL;DR: use `develop` for testing, `beta` for pre-release validation, and `latest` for stable production.

The CI script used to orchestrate the Docker image builds lives in `.github/workflows/docker-build.yaml`.

Scrutiny now uses three branch-backed image channels:

- `develop` branch -> `develop` and `develop-omnibus`
- `beta` branch -> `beta` and `beta-omnibus`
- `master` branch -> `latest` and `latest-omnibus`

Typical flow is `develop -> master`, with optional `develop -> beta -> master` promotion when a feature needs pre-release validation.

Use cases:

- `develop-*` for integration and maintainer testing
- `beta-*` for release-candidate validation
- `latest-*` for stable production deployments

# Running Docker `rootless`

To avoid that the container(s) restart when you installed Docker as `rootless` you need to isssue the following commands to allow the session to stay alive even after you close your (SSH) sesssion:

`sudo loginctl enable-linger $(whoami)`

`systemctl --user enable docker`
