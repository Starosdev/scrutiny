# Scrutiny Loop Pilot

This pilot is split into one scheduled report and two manual draft-only analyzers.

## Active loops

- `loop-pilot-triage`: daily scheduled triage plus manual reruns
- `loop-pilot-pr-babysitter`: manual PR blocker analysis
- `loop-pilot-dependency-sweeper`: manual dependency PR risk analysis

## Cadence

- Triage schedule: daily at 13:20 UTC
- Manual trigger: `workflow_dispatch` for all loops

## Allowed actions

- Read GitHub PR, issue, check, and review state
- Publish workflow summaries and upload markdown artifacts
- Produce draft-only next-step plans for a single PR or dependency target

## Denylisted actions

- No autonomous commits, pushes, rebases, merges, or branch deletion
- No PR label, assignee, milestone, reviewer, or comment mutation
- No release, deploy, or environment actions
- No automatic worktree creation from GitHub Actions

## Collision rules

- Manual analyzers operate on one PR at a time
- If the target PR already has an active human owner or overlapping change in flight, the loop reports state only
- Runtime output stays in workflow summaries and artifacts; it does not commit state back to the branch

## Kill conditions

- Repeated false-positive blocker summaries
- Draft-action plans that imply unsafe mutation beyond repo policy
- Inability to classify PR risk without widening permissions or writing back to the repo
