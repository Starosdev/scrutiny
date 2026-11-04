from pathlib import Path

import yaml


WORKFLOW = Path(__file__).parents[2] / ".github/workflows/release.yaml"


def _steps(workflow: dict) -> dict:
    return {step["name"]: step for step in workflow["jobs"]["release"]["steps"]}


def test_release_notes_failure_does_not_skip_assets() -> None:
    # v1.79.1 shipped with 2 assets because a GitHub API timeout in this step failed
    # the job after the tag existed, which skipped Build Binaries and Upload Release Assets.
    steps = _steps(yaml.safe_load(WORKFLOW.read_text(encoding="utf-8")))
    assert steps["Generate release notes from PRs"]["continue-on-error"] is True


def test_rebuild_tag_reuses_existing_release_without_tagging() -> None:
    workflow = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))
    triggers = workflow.get("on", workflow.get(True, {}))
    assert "rebuild_tag" in triggers["workflow_dispatch"]["inputs"]

    steps = _steps(workflow)
    assert steps["Reuse existing release tag (asset rebuild only)"]["if"] == "inputs.rebuild_tag != ''"
    assert steps["Semantic Release"]["if"] == "inputs.rebuild_tag == ''"
    assert "inputs.rebuild_tag == ''" in steps["Force release commit (manual trigger only)"]["if"]

    outputs = workflow["jobs"]["release"]["outputs"]
    for key in ("new_release_published", "new_release_version"):
        assert f"steps.rebuild.outputs.{key} || steps.semantic.outputs.{key}" in outputs[key]
