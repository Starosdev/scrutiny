# Scrutiny Loop Pilot State

This file is the checked-in control document for the pilot.

## Current status

- Stage: `pilot`
- Scope: PR flow, issue triage, dependency hygiene
- Automation level: triage is scheduled, action loops are manual and draft-only
- Runtime output: GitHub Actions summary plus uploaded markdown artifact

## Human inbox

- Confirm whether triage reports reduce PR and issue queue scanning time
- Review draft-action output before widening any workflow permissions
- Convert repeated safe recommendations into explicit allowlist rules

## Latest known defaults

- No branch mutation from the pilot
- No PR or issue mutation from the pilot
- Dependency updates remain human-reviewed even when classified low risk
