# 0007. Static CGO-free builds and committed vendor/

Status: accepted
Date: 2026-07-02

## Context

Scrutiny ships binaries for 10 GOOS/GOARCH targets (linux amd64/arm
v5-v7/arm64, darwin, freebsd, windows) and Docker images on slim Debian
bases. Dynamic linking breaks the slim runtime; module fetches inside Docker
builds are slow and non-reproducible.

## Decision

- All shipped builds set `STATIC=true`, which drives `CGO_ENABLED=0`,
  `-extldflags=-static`, and `-tags "static netgo"` in the Makefile. The
  SQLite driver is the pure-Go one; nothing may introduce a CGO dependency.
- `vendor/` is committed. `binary-dep: go mod vendor` is a prerequisite of
  every build target, and CI re-vendors before lint to catch drift.
- The release binary matrix in `release.yaml` and the CI matrix in `ci.yaml`
  list the same 10 targets and move in lockstep.
- CI compiles with the Go version from `go.mod`; Dockerfiles pin their own
  `golang` base image (currently one minor ahead). The two drift by design
  but a deliberate Go bump touches both.

## Consequences

- Dropping `STATIC: true` from a CI job silently re-enables CGO and produces
  binaries that fail the `ldd`/self-exec smoke checks and the slim runtime.
- Any dependency change is a two-file-plus-vendor diff (`go.mod`, `go.sum`,
  `vendor/`); hand-editing vendor/ or skipping `go mod vendor` fails lint.
- New build targets or collectors must be added to both workflow matrices
  and the Makefile together.
- arm/v7 Docker images exist only for the base collector; the omnibus S6
  arch map handles only amd64/arm64, so adding other omnibus architectures
  requires extending that map first or `/init` breaks.
