# Scrutiny Agent Instructions

These instructions apply to all automated builders and reviewers working in this repository.

## Session Startup

- Always work in a git worktree. Do not make changes directly in the main repository directory unless the user explicitly authorizes it.
- Read this file before doing any work.
- Check `git status --short --branch` before editing.
- Assume the worktree may contain user changes. Do not revert, overwrite, or clean up changes you did not make unless the user explicitly asks.
- Treat the Starosdev fork as the only source of truth for sync, merge, and release work. Do not compare against or sync from the upstream AnalogJ repository unless the user explicitly asks.

## Communication

- Be direct and factual.
- Act as an involved, methodical collaborator rather than a reflexively agreeable assistant.
- Push back when the requested approach is weak, risky, underspecified, or inconsistent with the repository's standards.
- Push forward responsibly when the next step is clear, but escalate irreversible or high-impact decisions to the user.
- Keep replies concise. Avoid fluff and filler.
- Do not use emojis in commits, pull requests, comments, documentation, or user-facing project text.
- Do not mention the name of any AI assistant in commits, pull request titles, pull request descriptions, comments, or project files. Use role-based terms such as `builder` or `reviewer` when needed.

## Git Workflow

- Prefer branch names with the `codex/` prefix unless the user requests a different naming scheme.
- Keep changes scoped to the requested work. Avoid opportunistic cleanup.
- Before merging, verify the current branch state with `git status --short --branch`.
- Merge into `develop` only when the user explicitly asks for that delivery step.
- Never use destructive git commands such as `git reset --hard` or `git checkout --` unless the user explicitly asks.

## Repository Shape

- `collector/` contains the Go collectors, backend services, and shared packages.
- `webapp/frontend/` contains the Angular frontend.
- `docker/` and the compose files define the container packaging paths used by most deployments.
- `docs/` holds operator and feature documentation. Update docs when behavior, setup, or configuration changes.

## Editing Standards

- Prefer existing local patterns over new abstractions.
- Use `rg` for search and `apply_patch` for manual edits.
- Keep Go, Angular, and Docker changes narrowly targeted to the requested behavior.
- Do not add broad refactors, formatting churn, or unrelated dependency work while fixing a focused issue.
- If a change affects configuration, collectors, notifications, or deployment behavior, call that out clearly in the final handoff.

## Verification

Choose checks based on the files touched:

- Go changes: `go test ./...`
- Frontend changes: `npm --prefix webapp/frontend test`
- Frontend lint and type checks: `npm --prefix webapp/frontend run lint` and `npm --prefix webapp/frontend run build`
- Container or packaging changes: run the narrowest relevant `docker compose` or build command that exercises the edited path

If a command cannot be run, state why and identify the remaining risk.
