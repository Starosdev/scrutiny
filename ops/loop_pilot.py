#!/usr/bin/env python3
"""Generate read-only loop-pilot reports for Scrutiny."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


BOT_LOGINS = {"dependabot[bot]", "renovate[bot]"}
NOW_ISSUE_LABELS = {"needs-info", "ready-for-agent"}
QUEUE_ISSUE_LABELS = {"needs-triage", "ready-for-human"}
MERGE_BLOCKERS = {"BEHIND", "BLOCKED", "DIRTY", "DRAFT", "UNKNOWN"}
DEPENDENCY_DENYLIST = (
    "@angular/",
    "angular",
    "rxjs",
    "typescript",
    "node",
    "golang",
    "breaking",
    "major",
)
DEPENDENCY_FILE_ALLOWLIST = (
    "go.mod",
    "go.sum",
    "webapp/frontend/package.json",
    "webapp/frontend/package-lock.json",
    "webapp/frontend/yarn.lock",
    "webapp/frontend/pnpm-lock.yaml",
    ".github/workflows/",
)


def run_json(command: list[str], allow_failure: bool = False) -> object:
    result = subprocess.run(command, capture_output=True, text=True)
    if result.returncode != 0:
        if allow_failure:
            return []
        raise subprocess.CalledProcessError(
            result.returncode,
            command,
            output=result.stdout,
            stderr=result.stderr,
        )
    return json.loads(result.stdout or "[]")


def parse_timestamp(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def age_in_days(value: str, now: datetime) -> int:
    return max(0, (now - parse_timestamp(value)).days)


def label_names(item: dict) -> set[str]:
    return {label["name"] for label in item.get("labels", []) if label.get("name")}


def is_dependency_pr(pr: dict) -> bool:
    labels = {name.lower() for name in label_names(pr)}
    title = (pr.get("title") or "").lower()
    author = ((pr.get("author") or {}).get("login") or "").lower()
    return (
        author in BOT_LOGINS
        or "dependencies" in labels
        or "dependency" in title
        or "deps" in title
    )


def fetch_open_prs() -> list[dict]:
    return run_json(
        [
            "gh",
            "pr",
            "list",
            "--state",
            "open",
            "--limit",
            "100",
            "--json",
            "author,baseRefName,headRefName,isDraft,labels,mergeStateStatus,number,reviewDecision,title,updatedAt,url",
        ]
    )


def fetch_open_issues() -> list[dict]:
    return run_json(
        [
            "gh",
            "issue",
            "list",
            "--state",
            "open",
            "--limit",
            "100",
            "--json",
            "assignees,author,labels,number,title,updatedAt,url",
        ]
    )


def fetch_pr(pr_number: str) -> dict:
    return run_json(
        [
            "gh",
            "pr",
            "view",
            pr_number,
            "--json",
            "author,baseRefName,files,headRefName,isDraft,labels,mergeStateStatus,number,reviewDecision,title,updatedAt,url",
        ]
    )


def fetch_pr_checks(pr_number: str) -> list[dict]:
    return run_json(
        [
            "gh",
            "pr",
            "checks",
            pr_number,
            "--json",
            "bucket,link,name,state,workflow",
        ],
        allow_failure=True,
    )


def classify_pr(pr: dict, now: datetime) -> tuple[str, list[str]]:
    reasons: list[str] = []
    updated_days = age_in_days(pr["updatedAt"], now)
    review_decision = pr.get("reviewDecision") or "NONE"
    merge_state = pr.get("mergeStateStatus") or "UNKNOWN"

    if pr.get("isDraft"):
        reasons.append("draft")
    if review_decision == "CHANGES_REQUESTED":
        reasons.append("changes requested")
    elif review_decision == "REVIEW_REQUIRED":
        reasons.append("needs review")
    if merge_state in MERGE_BLOCKERS and merge_state != "UNKNOWN":
        reasons.append(f"merge state {merge_state.lower()}")
    if updated_days >= 5 and not pr.get("isDraft"):
        reasons.append(f"stale {updated_days}d")

    if reasons and not pr.get("isDraft"):
        return "now", reasons
    if pr.get("isDraft"):
        return "wait", reasons or ["draft"]
    return "wait", reasons or ["monitor"]


def classify_issue(issue: dict, now: datetime) -> tuple[str, list[str]]:
    labels = label_names(issue)
    updated_days = age_in_days(issue["updatedAt"], now)
    reasons: list[str] = []

    if labels & NOW_ISSUE_LABELS:
        reasons.extend(sorted(labels & NOW_ISSUE_LABELS))
        if updated_days >= 7:
            reasons.append(f"stale {updated_days}d")
        return "now", reasons

    if labels & QUEUE_ISSUE_LABELS:
        reasons.extend(sorted(labels & QUEUE_ISSUE_LABELS))
        if updated_days >= 7:
            reasons.append(f"stale {updated_days}d")
        return "wait", reasons

    return "ignore", []


def bullet_for_pr(pr: dict, reasons: list[str], now: datetime) -> str:
    updated_days = age_in_days(pr["updatedAt"], now)
    return (
        f"- PR #{pr['number']} `{pr['title']}`"
        f" ({', '.join(reasons)}; updated {updated_days}d ago) - {pr['url']}"
    )


def bullet_for_issue(issue: dict, reasons: list[str], now: datetime) -> str:
    updated_days = age_in_days(issue["updatedAt"], now)
    return (
        f"- Issue #{issue['number']} `{issue['title']}`"
        f" ({', '.join(reasons)}; updated {updated_days}d ago) - {issue['url']}"
    )


def file_paths(pr: dict) -> list[str]:
    return [entry.get("path", "") for entry in pr.get("files", []) if entry.get("path")]


def suggested_validation(paths: list[str]) -> list[str]:
    commands: list[str] = []
    if any(path.endswith(".go") or path in {"go.mod", "go.sum"} or path.startswith("collector/") for path in paths):
        commands.append("go test ./...")
    if any(path.startswith("webapp/frontend/") for path in paths):
        commands.append("npm --prefix webapp/frontend test")
        commands.append("npm --prefix webapp/frontend run build")
    if any(path.startswith(".github/workflows/") or path.startswith("docker/") or path.startswith("deploy/") for path in paths):
        commands.append("Review the narrowest relevant workflow or packaging path manually")
    return commands or ["No narrow validation command inferred from changed files"]


def summarize_checks(checks: list[dict]) -> tuple[list[str], list[str]]:
    failing: list[str] = []
    pending: list[str] = []
    for check in checks:
        state = (check.get("state") or "").upper()
        name = check.get("name") or check.get("workflow") or "unknown check"
        if state in {"FAILURE", "ERROR", "TIMED_OUT", "CANCELLED"}:
            failing.append(name)
        elif state in {"PENDING", "QUEUED", "IN_PROGRESS", "WAITING"}:
            pending.append(name)
    return failing, pending


def render_triage_report() -> str:
    now = datetime.now(timezone.utc)
    prs = fetch_open_prs()
    issues = fetch_open_issues()

    attention_prs: list[str] = []
    waiting_prs: list[str] = []
    dependency_items: list[str] = []
    close_or_defer: list[str] = []
    automation_candidates: list[str] = []
    attention_issues: list[str] = []
    queue_issues: list[str] = []

    for pr in prs:
        if is_dependency_pr(pr):
            dependency_items.append(bullet_for_pr(pr, ["dependency PR"], now))
            if age_in_days(pr["updatedAt"], now) >= 14:
                close_or_defer.append(
                    bullet_for_pr(pr, ["dependency PR", "stale candidate"], now)
                )
            else:
                automation_candidates.append(
                    bullet_for_pr(pr, ["manual dependency-sweeper candidate"], now)
                )
            continue

        lane, reasons = classify_pr(pr, now)
        if lane == "now":
            attention_prs.append(bullet_for_pr(pr, reasons, now))
            automation_candidates.append(
                bullet_for_pr(pr, ["manual PR babysitter candidate"], now)
            )
        else:
            waiting_prs.append(bullet_for_pr(pr, reasons, now))
            if pr.get("isDraft") and age_in_days(pr["updatedAt"], now) >= 21:
                close_or_defer.append(
                    bullet_for_pr(pr, ["draft PR", "stale candidate"], now)
                )

    for issue in issues:
        lane, reasons = classify_issue(issue, now)
        if lane == "now":
            attention_issues.append(bullet_for_issue(issue, reasons, now))
            if "needs-info" in label_names(issue) and age_in_days(issue["updatedAt"], now) >= 14:
                close_or_defer.append(
                    bullet_for_issue(issue, ["needs-info", "close/defer candidate"], now)
                )
        elif lane == "wait":
            queue_issues.append(bullet_for_issue(issue, reasons, now))

    sections = [
        "# Scrutiny Loop Pilot Triage",
        "",
        f"- Generated: {now.isoformat()}",
        "- Scope: PR flow, issue triage, dependency hygiene",
        "- Automation level: scheduled triage, manual draft-only action loops",
        "",
        "## Needs attention now",
        *(attention_prs or ["- No non-draft PRs met the immediate-attention threshold."]),
        *(attention_issues or ["- No open issues currently sit in the immediate-attention bucket."]),
        "",
        "## Can wait",
        *(waiting_prs or ["- No waiting PRs were detected."]),
        *(queue_issues or ["- No waiting issue triage queue was detected."]),
        "",
        "## Dependency hygiene",
        *(dependency_items or ["- No open dependency-focused PRs were detected."]),
        "",
        "## Close or defer candidates",
        *(close_or_defer or ["- No obvious close/defer candidates were detected."]),
        "",
        "## Manual action candidates",
        *(automation_candidates or ["- No manual action candidates were detected."]),
        "",
        "## Operator notes",
        "- This workflow does not mutate branches, PRs, labels, comments, or release state.",
        "- Use the manual analyzers for one PR at a time when this report surfaces a clear target.",
    ]
    return "\n".join(sections) + "\n"


def render_pr_babysitter(pr_number: str) -> str:
    now = datetime.now(timezone.utc)
    pr = fetch_pr(pr_number)
    checks = fetch_pr_checks(pr_number)
    paths = file_paths(pr)
    failing_checks, pending_checks = summarize_checks(checks)

    blockers: list[str] = []
    review_decision = pr.get("reviewDecision") or "NONE"
    merge_state = pr.get("mergeStateStatus") or "UNKNOWN"
    if pr.get("isDraft"):
        blockers.append("PR is still a draft")
    if review_decision == "CHANGES_REQUESTED":
        blockers.append("Review state is changes requested")
    elif review_decision == "REVIEW_REQUIRED":
        blockers.append("Review is still required")
    if merge_state in {"BEHIND", "BLOCKED", "DIRTY"}:
        blockers.append(f"Merge state is {merge_state.lower()}")
    if failing_checks:
        blockers.append(f"Failing checks: {', '.join(failing_checks)}")
    if pending_checks:
        blockers.append(f"Pending checks: {', '.join(pending_checks)}")

    validation = suggested_validation(paths)
    changed_files = paths[:12] or ["No changed files reported by GitHub"]
    human_next = (
        "Resolve the listed blockers, then rerun the analyzer for an updated draft plan."
        if blockers
        else "Open a local worktree for the PR branch and prepare the smallest possible fix draft."
    )

    blocker_lines = [f"- {blocker}" for blocker in blockers] or [
        "- No blockers detected from current GitHub state."
    ]

    sections = [
        f"# Scrutiny PR Babysitter Draft for PR #{pr['number']}",
        "",
        f"- Generated: {now.isoformat()}",
        f"- Title: {pr['title']}",
        f"- URL: {pr['url']}",
        f"- Branch: `{pr['headRefName']}` -> `{pr['baseRefName']}`",
        "- Automation level: draft-only",
        "",
        "## Current blockers",
        *blocker_lines,
        "",
        "## Changed files",
        *(f"- `{path}`" for path in changed_files),
        "",
        "## Suggested validation",
        *(f"- `{command}`" for command in validation),
        "",
        "## Draft next step",
        f"- {human_next}",
        "- No commit, push, PR edit, or review action was performed.",
    ]
    return "\n".join(sections) + "\n"


def resolve_dependency_target(pr_number: str | None, dependency_name: str | None) -> str:
    if pr_number:
        return pr_number
    if not dependency_name:
        raise ValueError("Either --pr-number or --dependency-name is required.")

    prs = run_json(
        [
            "gh",
            "pr",
            "list",
            "--state",
            "open",
            "--limit",
            "50",
            "--search",
            dependency_name,
            "--json",
            "number,title,author,labels,url",
        ]
    )
    dependency_name_lower = dependency_name.lower()
    for pr in prs:
        if dependency_name_lower in (pr.get("title") or "").lower():
            return str(pr["number"])
    raise ValueError(f"No open PR matched dependency target: {dependency_name}")


def dependency_blockers(pr: dict) -> list[str]:
    blockers: list[str] = []
    title = (pr.get("title") or "").lower()
    author = ((pr.get("author") or {}).get("login") or "").lower()
    labels = {name.lower() for name in label_names(pr)}
    paths = file_paths(pr)

    if pr.get("isDraft"):
        blockers.append("PR is still a draft")
    if author not in BOT_LOGINS and "dependencies" not in labels:
        blockers.append("Target does not look like a bot-managed dependency PR")
    if any(token in title for token in DEPENDENCY_DENYLIST):
        blockers.append("Title suggests a major or framework-sensitive update")
    unexpected_paths = [
        path
        for path in paths
        if not any(path == allowed or path.startswith(allowed) for allowed in DEPENDENCY_FILE_ALLOWLIST)
    ]
    if unexpected_paths:
        blockers.append(
            "Changed files extend beyond the dependency allowlist: "
            + ", ".join(unexpected_paths[:5])
        )
    return blockers


def render_dependency_sweeper(pr_number: str | None, dependency_name: str | None) -> str:
    now = datetime.now(timezone.utc)
    resolved_pr = resolve_dependency_target(pr_number, dependency_name)
    pr = fetch_pr(resolved_pr)
    blockers = dependency_blockers(pr)
    validation = suggested_validation(file_paths(pr))
    risk = "high" if blockers else "low"
    target_label = dependency_name or f"PR #{pr['number']}"

    blocker_lines = [f"- {blocker}" for blocker in blockers] or [
        "- No stop reasons were detected from current GitHub state."
    ]
    changed_paths = [f"- `{path}`" for path in file_paths(pr)[:12]] or [
        "- No changed files reported by GitHub"
    ]

    sections = [
        f"# Scrutiny Dependency Sweeper Draft for {target_label}",
        "",
        f"- Generated: {now.isoformat()}",
        f"- PR: #{pr['number']} `{pr['title']}`",
        f"- URL: {pr['url']}",
        f"- Risk bucket: `{risk}`",
        "- Automation level: draft-only",
        "",
        "## Stop reasons",
        *blocker_lines,
        "",
        "## Changed files",
        *changed_paths,
        "",
        "## Suggested validation",
        *(f"- `{command}`" for command in validation),
        "",
        "## Draft next step",
        (
            "- Keep this in manual review; widen no permissions until the stop reasons are cleared."
            if blockers
            else "- Safe to review manually as a low-risk dependency candidate, but still do not auto-merge."
        ),
        "- No commit, push, PR edit, or label mutation was performed.",
    ]
    return "\n".join(sections) + "\n"


def write_report(content: str, output: Path | None) -> None:
    if output is not None:
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(content, encoding="utf-8")

    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary_path:
        with open(summary_path, "a", encoding="utf-8") as handle:
            handle.write(content)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    triage = subparsers.add_parser("triage", help="Generate the daily triage report.")
    triage.add_argument("--output", type=Path, default=None, help="Optional markdown output path.")

    babysitter = subparsers.add_parser("pr-babysitter", help="Analyze one PR without mutation.")
    babysitter.add_argument("--pr-number", required=True, help="PR number to inspect.")
    babysitter.add_argument("--output", type=Path, default=None, help="Optional markdown output path.")

    dependency = subparsers.add_parser("dependency-sweeper", help="Analyze one dependency PR without mutation.")
    dependency.add_argument("--pr-number", default=None, help="PR number to inspect.")
    dependency.add_argument("--dependency-name", default=None, help="Dependency name to search for.")
    dependency.add_argument("--output", type=Path, default=None, help="Optional markdown output path.")

    return parser.parse_args()


def main() -> int:
    args = parse_args()

    if args.command == "triage":
        report = render_triage_report()
    elif args.command == "pr-babysitter":
        report = render_pr_babysitter(args.pr_number)
    elif args.command == "dependency-sweeper":
        report = render_dependency_sweeper(args.pr_number, args.dependency_name)
    else:
        raise ValueError(f"Unhandled command: {args.command}")

    write_report(report, args.output)
    if not os.environ.get("GITHUB_STEP_SUMMARY"):
        sys.stdout.write(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
