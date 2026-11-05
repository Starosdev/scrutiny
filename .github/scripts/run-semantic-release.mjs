import fs from "node:fs";
import process from "node:process";
import semanticRelease from "semantic-release";

// A dry run resolves the next version and renders the notes without tagging,
// committing or publishing, so a change to the release toolchain can be checked
// before it ships. semantic-release skips its prepare and publish steps itself;
// this script additionally reports the run as unpublished so the build and
// upload jobs stay skipped, and prints the notes that would have been written.
const dryRun = process.env.SEMANTIC_RELEASE_DRY_RUN === "true";

const result = await semanticRelease({ cwd: process.cwd(), dryRun });
const output = process.env.GITHUB_OUTPUT;
if (!output) {
  throw new Error("GITHUB_OUTPUT is required");
}

const version = result?.nextRelease?.version ?? "";
const published = result && !dryRun ? "true" : "false";
fs.appendFileSync(output, "new_release_published=" + published + "\n");
fs.appendFileSync(output, "new_release_version=" + version + "\n");

if (!dryRun) {
  process.exit(0);
}

const notes = result?.nextRelease?.notes ?? "";
const summary = process.env.GITHUB_STEP_SUMMARY;
const report = result
  ? "## Dry run: " + version + "\n\n" + (notes.trim() || "_No notes were rendered._") + "\n"
  : "## Dry run\n\nNo release would be published from these commits.\n";

process.stdout.write(report);
if (summary) {
  fs.appendFileSync(summary, report);
}

// An empty notes body next to a resolved version is the failure this guards
// against: the release would publish, and the changelog and release commit
// would record nothing but a heading.
if (result && !notes.trim()) {
  console.error("::error::semantic-release resolved " + version + " but rendered empty notes.");
  process.exit(1);
}
