#!/usr/bin/env node
// Runs `npm audit` in the current directory and fails on any finding except the
// allowlisted advisories below. CI runs it for the root release tooling and for
// webapp/frontend; any finding at any severity outside the allowlist fails.
//
// The allowlist covers dependencies bundled inside the `npm` package itself
// (node_modules/npm/node_modules/...). semantic-release installs npm through
// @semantic-release/npm, but .releaserc.json does not load that plugin, so the
// bundled npm never runs. A bundled dependency cannot be fixed with a lockfile
// update or an override; only a new npm release can fix it. See issue 927.
//
// Each entry is allowed only while every affected path is inside the bundled npm
// tree, and only until ALLOWLIST_EXPIRES. After that date the step fails so the
// allowlist is reviewed instead of silently living forever.
//
// UNPATCHED_ADVISORIES is the second, narrower exception: an advisory with no
// patched release at all, so neither a lockfile update nor an override can fix it.
// Its values are objects read as `.pkg`, not bare strings like the two maps
// around it. Each entry names the one package it applies to, shares the same expiry, and
// records why the release tooling's use of that package is not exposed. A package that npm audit
// flags only because a dependency is vulnerable (via names, no advisory of its
// own) is allowed when every dependency it is flagged through is allowed.
//
// DEV_ONLY_ADVISORIES is the third exception: an advisory whose only patched
// release is a breaking major that no consumer supports yet, for a package that
// nothing in the published artifact depends on. The dev-only condition is not
// taken on trust: the script runs a second `npm audit --omit=dev`, and an entry
// applies only to a package that audit does not report, so a package that
// becomes a production dependency loses the exception automatically. These
// entries share ALLOWLIST_EXPIRES, which is the point at which the ecosystem is
// re-checked for a non-breaking upgrade path.
//
// Unlike the bundled npm, build and test tooling does execute, in CI, with
// repository credentials. That is the whole reason this gate audits
// devDependencies at all instead of running `npm audit --omit=dev`. So the
// exception is capped at DEV_ONLY_MAX_SEVERITY: a high or critical advisory in
// dev tooling fails the gate however awkward the upgrade, and gets an explicit
// decision rather than a one-line allowlist entry.
import { execFileSync } from 'node:child_process';

const ALLOWLIST_EXPIRES = '2026-12-15';
const BUNDLED_NPM_PREFIX = 'node_modules/npm/node_modules/';
// Keyed by advisory to the bare package name it is allowed for, as
// DEV_ONLY_ADVISORIES is. UNPATCHED_ADVISORIES below keys to an object read as
// `.pkg`, so do not copy an entry between them unchanged. A bare set of IDs
// would allow an advisory for any package it is ever amended to list, provided
// that package's paths were all bundled, which is wider than any evidence here
// supports.
const ALLOWED_ADVISORIES = new Map([
  // npm bundles ip-address 10.7.2 and undici 6.29.0, unchanged from 11.21.1
  // through 12.2.1, the latest release as of 2026-10-30. No released npm fixes
  // them, and the nested copies cannot be overridden: raising the npm override
  // to ^12.2.1 and regenerating the lockfile reported the same two packages.
  //
  // The fix is imminent rather than distant. npm/cli's default branch already
  // bundles undici 6.29.1 and ip-address 10.7.5, which patch every advisory
  // here, and npm ships every few weeks. That is why the expiry is December
  // rather than months out, and why removal needs the npm override in
  // package.json raised past 11.x once the release lands.
  //
  // GHSA-2m5v-pch6-74hx is a TLS certificate validation bypass, not the
  // denial-of-service class every earlier entry in this list held. The
  // exception rests entirely on the bundled npm never executing, so the cost of
  // that argument being wrong is now confidentiality, which argues for the
  // short expiry above. The configured release plugins do not execute npm.
  // See docs/DEPENDENCIES.md for the audit and upstream release evidence.
  ['GHSA-rxp7-6hxx-x8q5', 'ip-address'], // zone parsing
  ['GHSA-4qv8-2p9j-2685', 'undici'], // EventSource parser
  ['GHSA-xjj6-f33q-929c', 'undici'], // content encodings
  ['GHSA-2m5v-pch6-74hx', 'undici'], // TLS certificate validation bypass
  ['GHSA-qrxm-fqhp-jwvm', 'undici'], // proxy authorization header leak
  ['GHSA-9fm3-97px-rjf6', 'undici'], // PONG frame memory use
  // npm bundles glob 13.x and minimatch 10.x. Both advisories below are patched
  // only in a major the bundled range cannot reach: glob 14.0.2 and minimatch
  // 11.0.1, both published 2026-10-27. npm 11.21.1 and 12.2.1, the latest
  // release, both still declare `glob ^13.0.6` and `minimatch ^10.2.5`, so no
  // released npm fixes either, and bundled files cannot be overridden. Removing
  // these needs an npm release that bumps those ranges.
  ['GHSA-3g3m-2fjr-6pw4', 'glob'], // AbortSignal cancellation bypass
  ['GHSA-g3hv-75wg-m249', 'minimatch'], // globparts amplification
]);
const DEV_ONLY_SEVERITIES = new Set(['info', 'low', 'moderate']);
const DEV_ONLY_ADVISORIES = new Map([
  // minimatch is patched only in 11.0.1 (published 2026-10-27), a major release.
  // No consumer has moved: eslint 10.14.0 declares ^10.2.6, @typescript-eslint
  // 8.72.1 declares ^10.2.2, and karma 6.4.4, karma-coverage 2.2.1 and
  // @eslint/eslintrc 3.3.7 all declare ^3.0.4, whose v3 call style minimatch 11
  // no longer exports. Forcing the major through an override would break the
  // test runner rather than fix anything, and 15 of the 17 packages the frontend
  // audit reports are flagged only through this one.
  //
  // Every flagged path is build and test tooling: eslint, typescript-eslint,
  // angular-eslint, karma and their dependencies. `npm audit --omit=dev` on the
  // frontend reports no findings at all, which is the condition enforced below.
  // The advisory is a denial of service through glob-pattern amplification, so
  // the exposure is a CI job processing its own repository's patterns.
  ['GHSA-g3hv-75wg-m249', 'minimatch'], // globparts amplification
]);
const UNPATCHED_ADVISORIES = new Map([
  // Empty. The braces entry (GHSA-vfj7-8cjw-p6xm) was removed on 2026-10-30:
  // GitHub withdrew that advisory on 2026-10-23, and neither audit reports the
  // package any more, so the exception covered nothing. Add an entry here only
  // for an advisory with no patched release in any version.
]);

if (new Date().toISOString().slice(0, 10) > ALLOWLIST_EXPIRES) {
  console.error(`npm audit allowlist expired on ${ALLOWLIST_EXPIRES}. Check whether a newer npm fixes the bundled dependencies (issue 927), then update or remove the allowlist in .github/scripts/audit-release-tooling.mjs.`);
  process.exit(1);
}

const runAudit = (args) => {
  let output;
  try {
    output = execFileSync('npm', ['audit', '--json', ...args], { encoding: 'utf8' });
  } catch (err) {
    // npm audit exits non-zero when it finds anything; the JSON is still on stdout.
    output = err.stdout;
  }
  const parsed = JSON.parse(output);
  if (parsed.error) {
    console.error(`npm audit ${args.join(' ')} failed:`, JSON.stringify(parsed.error));
    process.exit(1);
  }
  return parsed;
};

const report = runAudit([]);
// A package the production-only audit does not report reaches the tree through
// devDependencies alone. npm resolves that, so DEV_ONLY_ADVISORIES does not have
// to be told which packages are build-time.
//
// This one must not default to an empty object the way the full report may: a
// missing key here would mark every package dev-only and wave the whole dev tree
// through. An unrecognized shape is a failure, not an empty result.
const productionReport = runAudit(['--omit=dev']);
if (typeof productionReport.vulnerabilities !== 'object' || productionReport.vulnerabilities === null) {
  console.error('npm audit --omit=dev returned no vulnerabilities object; cannot establish which packages are dev-only.');
  process.exit(1);
}
const isDevOnly = (name) => !(name in productionReport.vulnerabilities);

const vulnerabilities = report.vulnerabilities ?? {};
const advisoriesOf = (vuln) => vuln.via.filter((via) => typeof via === 'object').map((via) => via.url.split('/').pop());
const viaNamesOf = (vuln) => vuln.via.filter((via) => typeof via === 'string');

// A package is blocked when one of its own advisories is not allowlisted, or when
// it is flagged through a blocked dependency. Blocking is propagated to a fixed
// point rather than by recursion, because semantic-release and its plugins flag
// each other in a cycle.
const ownAllowed = (name, vuln) => {
  // An entry that names neither an advisory nor a dependency carries no evidence either way; fail closed.
  if (vuln.via.length === 0) return false;
  const bundled = vuln.nodes.length > 0 && vuln.nodes.every((node) => node.startsWith(BUNDLED_NPM_PREFIX));
  const devOnly = vuln.nodes.length > 0 && DEV_ONLY_SEVERITIES.has(vuln.severity) && isDevOnly(name);
  return advisoriesOf(vuln).every((id) => UNPATCHED_ADVISORIES.get(id)?.pkg === name
    || (bundled && ALLOWED_ADVISORIES.get(id) === name)
    || (devOnly && DEV_ONLY_ADVISORIES.get(id) === name));
};
const blocked = new Set(Object.entries(vulnerabilities).filter(([name, vuln]) => !ownAllowed(name, vuln)).map(([name]) => name));
for (let changed = true; changed; ) {
  changed = false;
  for (const [name, vuln] of Object.entries(vulnerabilities)) {
    if (!blocked.has(name) && viaNamesOf(vuln).some((via) => blocked.has(via) || !(via in vulnerabilities))) {
      blocked.add(name);
      changed = true;
    }
  }
}
const isAllowed = (name) => !blocked.has(name);

const blocking = [];
for (const [name, vuln] of Object.entries(vulnerabilities)) {
  const advisories = advisoriesOf(vuln);
  const line = `${name} (${vuln.severity}) ${advisories.join(', ') || 'via ' + vuln.via.join(', ')} at ${vuln.nodes.join(', ')}`;
  if (isAllowed(name)) {
    console.log(`allowed: ${line}`);
  } else {
    blocking.push(line);
  }
}

if (blocking.length > 0) {
  console.error('npm audit findings not covered by the allowlist:');
  for (const line of blocking) console.error(`  ${line}`);
  process.exit(1);
}
console.log('npm audit: no findings outside the allowlist.');
