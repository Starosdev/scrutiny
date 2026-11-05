import fs from "node:fs";
import process from "node:process";
import semanticRelease from "semantic-release";

// A dry run resolves the next version and renders the notes without tagging,
// committing or publishing, so a change to the release toolchain can be checked
// before it ships. semantic-release skips its prepare and publish steps itself;
// this script additionally reports the run as unpublished so the build and
// upload jobs stay skipped, and prints the notes that would have been written.
const dryRun = process.env.SEMANTIC_RELEASE_DRY_RUN === "true";
const ref = process.env.GITHUB_REF_NAME ?? "";

// semantic-release returns false on any ref outside its configured branches, and
// a dry run that returns false renders no notes to check. Dispatched from a
// feature branch it would therefore report success while verifying nothing, so
// resolve the ref against the configuration before running anything.
if (dryRun) {
  let config;
  try {
    config = JSON.parse(fs.readFileSync(".releaserc.json", "utf8"));
  } catch (error) {
    console.error("::error::Could not read .releaserc.json: " + error.message);
    process.exit(1);
  }

  // Literal comparison, which is all the current configuration needs: master and
  // beta. semantic-release also accepts globs and {name, range} maintenance
  // branches; adding one of those means matching it here too, or a legitimate
  // dry run from it fails this check.
  const releaseBranches = (config.branches ?? []).map((branch) =>
    typeof branch === "string" ? branch : branch.name,
  );
  if (releaseBranches.length === 0) {
    console.error("::error::.releaserc.json declares no branches, so no ref can be verified.");
    process.exit(1);
  }
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

// The regression this guards against renders a heading and nothing else, so an
// emptiness test would pass straight through it: every release from 1.69.0 to
// 1.79.2 carried "## [version](compare-link) (date)" and no body. A working
// renderer emits the commits as Markdown list items under a section heading, so
// that is what gets checked.
const hasRenderedCommits = /^\s*[*-] /m.test(notes);

// A real release cannot be failed here, because the tag and the GitHub release
// already exist by this point and the binaries still have to upload, so it is
// reported as a warning and the run continues.
if (result && !hasRenderedCommits) {
  const message =
    "semantic-release resolved " +
    version +
    " but rendered no commit entries. CHANGELOG.md and the release commit carry only a heading.";
  console.error((dryRun ? "::error::" : "::warning::") + message);
}

if (!dryRun) {
  process.exit(0);
}

const report = result
  ? "## Dry run: " + version + "\n\n" + (notes.trim() || "_No notes were rendered._") + "\n"
  : "## Dry run: no release\n\nNo release resolves from " + ref + ".\n";

process.stdout.write(report);
const summary = process.env.GITHUB_STEP_SUMMARY;
if (summary) {
  fs.appendFileSync(summary, report);
}

// Nothing to render means nothing was verified. Every release leaves a
// chore(release) commit at the tip of the release branch, and chore does not
// trigger a release, so this is the state of master between promotions.
if (!result) {
  console.error(
    "::error::No release resolves from " +
      ref +
      ", so no notes were rendered and the toolchain was not verified. Dry run against a ref carrying releasable commits.",
  );
  process.exit(1);
}

if (!hasRenderedCommits) {
  process.exit(1);
}
