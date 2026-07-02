# 0005. Manual-only releases; omnibus owns the bare image tags

Status: accepted
Date: 2026-07-02

## Context

This fork ships to real deployments on a weekly cadence. Auto-releasing on
every master push would couple "merge" to "ship" and remove the human
gate on version bumps.

## Decision

- Releases happen only via `workflow_dispatch` on `release.yaml`
  (semantic-release). Pushing to master builds `:latest` Docker images but
  releases nothing. Cadence: Sunday bugfixes, Saturday beta, monthly feature
  promotion.
- `webapp/backend/pkg/version/version.go` is the single version source,
  bumped by a sed in `.releaserc.json` and parsed by grep in `sonarqube.yaml`
  and by packagr. The literal shape `const VERSION = "x.y.z"` is load-bearing
  for all three; the file is release-managed and never hand-edited.
- Release binaries build from the tagged release commit (`ref: v${version}`),
  not the workspace, so embedded version banners match the tag.
- The omnibus image owns the bare `:latest`/`:beta`/`:develop` tags; split
  images carry `-web`, `-collector`, `-collector-*` suffixes. Channel deploy
  workflows (`release-and-deploy.yml`, `deploy-beta.yml`,
  `deploy-testing.yml`) exist separately from `docker-build.yaml`; production
  deploy uses `cancel-in-progress: false` so a prod publish is never aborted.
- `sync-develop.yaml` auto-merges master into beta into develop after every
  master push, opening a PR on conflict.

## Consequences

- Both `docker-build.yaml` and the channel deploy workflows push the same
  bare omnibus tags on the same events with separate caches; the published
  digest is last-writer-wins. Known redundancy; removing either workflow
  changes which build produces `:latest`.
- `docker-build.yaml`'s path filter excludes `version.go`, so the release
  bump commit alone triggers no image build; the `v*.*.*` tag push does.
  The tag trigger must never be removed.
- Reformatting the VERSION const line (quotes, spacing) silently breaks the
  release sed and ships the previous version.
- Adding a new collector binary means updating the Makefile `binary-all`
  list, the omnibus Dockerfile COPY block, the artifact globs in both
  `release.yaml` and `ci.yaml`, and the s6/cron files under `rootfs/` with
  their chmod entries. Missing one produces a binary that builds but never
  ships, or ships but is absent from the omnibus.
