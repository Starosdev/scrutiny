import fs from "node:fs";
import process from "node:process";
import semanticRelease from "semantic-release";

// A dry run resolves the next version and renders the notes without tagging,
// committing or publishing, so a change to the release toolchain can be checked
// before it ships. semantic-release skips its prepare and publish steps itself;
// this script additionally reports the run as unpublished so the build and
// upload jobs stay skipped, and prints the notes that would have been written.
const dryRun = process.env.SEMANTIC_RELEASE_DRY_RUN === "true";

// semantic-release returns false on any ref outside its configured branches, and
// a dry run that returns false renders no notes to check. Dispatched from a
// feature branch it would therefore report success while verifying nothing, so
// resolve the ref against the configuration before running anything.
if (dryRun) {
  const config = JSON.parse(fs.readFileSync(".releaserc.json", "utf8"));
  const releaseBranches = (config.branches ?? []).map((branch) =>
    typeof branch === "string" ? branch : branch.name,
  );
  const ref = process.env.GITHUB_REF_NAME ?? "";
  if (!releaseBranches.includes(ref)) {
    console.error(
      "::error::A dry run must be dispatched from a release branch (" +
        releaseBranches.join(", ") +
        "). Ref " +
        (ref || "<unknown>") +
        " resolves no release, so the notes it would check are never rendered.",
    );
    process.exit(1);
  }
}

const result = await semanticRelease({ cwd: process.cwd(), dryRun });
const output = process.env.GITHUB_OUTPUT;
if (!output) {
  throw new Error("GITHUB_OUTPUT is required");
}

const version = result?.nextRelease?.version ?? "";
const notes = result?.nextRelease?.notes ?? "";
const published = result && !dryRun ? "true" : "false";
fs.appendFileSync(output, "new_release_published=" + published + "\n");
fs.appendFileSync(output, "new_release_version=" + version + "\n");

// Empty notes beside a resolved version is the defect this guards against: the
// release publishes, and the changelog and release commit record nothing but a
// heading. A real release cannot be failed here, because the tag and the GitHub
// release already exist by this point, so it is reported as a warning and the
// run continues to build and upload binaries.
if (result && !notes.trim()) {
  const message =
    "semantic-release resolved " +
    version +
    " but rendered empty notes. CHANGELOG.md and the release commit carry only a heading.";
  if (dryRun) {
    console.error("::error::" + message);
  } else {
    console.error("::warning::" + message);
  }
}

if (!dryRun) {
  process.exit(0);
}

const report = result
  ? "## Dry run: " + version + "\n\n" + (notes.trim() || "_No notes were rendered._") + "\n"
  : "## Dry run\n\nNo release would be published from these commits.\n";

process.stdout.write(report);
const summary = process.env.GITHUB_STEP_SUMMARY;
if (summary) {
  fs.appendFileSync(summary, report);
}

if (result && !notes.trim()) {
  process.exit(1);
}
