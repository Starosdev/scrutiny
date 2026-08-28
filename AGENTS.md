# Scrutiny Builder Instructions

<!-- staros-agents-baseline: Staros-Labs/infra AGENTS.md -->

This repository adopts the Staros organization agent baseline by reference.
Rules below cover Scrutiny-specific workflow and validation.

## Working Rules

- Read this file and `CONTRIBUTING.md` before making changes.
- Sync the main checkout with `origin`, then work in an isolated git worktree.
- Preserve user changes and existing linked worktrees. Never reset, restore, or
  clean work you did not create.
- Keep this maintained fork easy to compare with `AnalogJ/scrutiny`; avoid
  unrelated rewrites of upstream-owned code.
- Never commit API keys, tokens, credentials, runtime config, DB files, or local
  agent/session state.
- Do not use emojis or assistant-product attribution in commits, PRs, comments,
  or docs.

## Branch Model

- `master` is production and release source.
- `develop` is normal feature and fix integration source.
- Normal implementation bootstrap must pass `--base origin/develop`.
- Use `--base origin/master` only for explicit hotfix, release, or workflow
  enrollment work.
- Shared Linear branches use `<repo>/<issue-key>-<slug>`; PR target still follows
  this branch model.

## Repository Shape

- `webapp/backend/`: Go API and web server.
- `webapp/frontend/`: Angular frontend.
- `collector/`: SMART, ZFS, mdadm, Btrfs, and performance collectors.
- `.github/workflows/`: CI, security, image, testing, and release automation.
- `docs/`: maintained fork and operator documentation.

## Validation

- Backend: `go test ./...`
- Focused backend: `go test -v ./path/to/pkg/...`
- Frontend: `cd webapp/frontend && npm test -- --watch=false`
- Use focused checks for documentation or workflow-only changes.
- Never run deployment or production mutations without explicit approval.
