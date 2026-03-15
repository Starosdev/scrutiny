# Release Schedule Design

**Date:** 2026-03-14
**Status:** Implemented
**Issue:** [#366](https://github.com/Starosdev/scrutiny/issues/366) - Limit releases to once a week or when stable

## Problem

Multiple releases per day (60+ in a month) made it difficult for users to track stable versions. Automation checking hourly still missed releases. Only the latest version is supported, but very few releases were stable.

## Solution

A predictable release cadence with a GitHub Project board for tracking what ships when.

### Release Cadence

| When | What | Channel |
| ---- | ---- | ------- |
| Sunday | Bug fixes and stability improvements | Stable (`:latest`) |
| Saturday | New features and experiments | Beta (`:beta`) |
| Monthly | Promote mature beta features to stable | Stable (`:latest`) |
| As needed | Critical hotfixes and urgent security patches | Stable (`:latest`) |

### CI/CD Changes

- **`release.yaml`** -- Removed push triggers to `master`/`beta`. Releases are manual only via `workflow_dispatch`.
- **`docker-build.yaml`** -- No changes. Docker images still auto-build on push to `master` and `develop`.

### GitHub Project Board

**Project:** [Release Schedule](https://github.com/users/Starosdev/projects/1)

**Columns:**

| Column | Purpose |
| ------ | ------- |
| Triage | New issues with no matching label |
| Security | Security vulnerabilities (non-urgent) |
| Hotfix | Critical bugs, urgent security |
| Bug Backlog | Non-critical bugs |
| Next Sunday | Bugs committed for this week's release |
| Beta (Saturday) | Features in progress |
| Next Monthly | Features approved for next stable release |
| Done | Auto-moved on PR merge |

**Built-in automations:**
- "Pull request merged" moves items to Done (enable manually in project settings)
- "Auto-add sub-issues" disabled

### Auto-Routing (`project-router.yaml`)

Issues are automatically added to the project and routed to the correct column based on labels:

| Label(s) | Routes to |
| -------- | --------- |
| `bug` | Bug Backlog |
| `enhancement` | Beta (Saturday) |
| `hotfix` | Hotfix |
| `security` | Security |
| `security` + `priority:urgent` | Hotfix (override) |
| `bug` + `priority:urgent` | Hotfix (override) |
| `bug` + `priority:high` | Hotfix (override) |
| No matching label | Triage |

Triggers on `issues: [opened, labeled]` so re-labeling re-routes.

### Issue Templates (YAML format)

- **Bug Report** (`bug_report.yml`) -- label: `bug`, structured form with required fields
- **Feature Request** (`feature_request.yml`) -- label: `enhancement`, structured form
- **Hotfix** (`hotfix.yml`) -- label: `hotfix`, similar to bug report with urgency dropdown
- **Blank issues** -- disabled via `config.yml`
- **Security** -- uses GitHub Security Advisories (linked from template chooser)

### New Label

- `hotfix` -- Critical bugs requiring immediate fix (red)

### Documentation Updated

- README.md -- Release schedule section added near the top
- CLAUDE.md -- Release schedule, release process, and project board routing rules
- `create-pull-request` command -- Updated to reflect manual-only releases
- `pre-deploy-check` command -- Added manual release reminder

## Manual Setup Required

After merging, go to the Release Schedule project settings and:
1. Enable "Pull request merged" workflow, set status to "Done"
2. Verify "Auto-add sub-issues to project" is disabled
