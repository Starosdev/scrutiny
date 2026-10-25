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
// Each entry names the one package it applies to, shares the same expiry, and
// records why the release tooling's use of that package is not exposed. A package that npm audit
// flags only because a dependency is vulnerable (via names, no advisory of its
// own) is allowed when every dependency it is flagged through is allowed.
import { execFileSync } from 'node:child_process';

const ALLOWLIST_EXPIRES = '2026-11-01';
const BUNDLED_NPM_PREFIX = 'node_modules/npm/node_modules/';
const ALLOWED_ADVISORIES = new Set([
  // npm 11.21.1 bundles ip-address 10.7.2 and undici 6.29.0. These copies
  // cannot be overridden. The configured release plugins do not execute npm.
  // See docs/DEPENDENCIES.md for the audit and upstream release evidence.
  'GHSA-rxp7-6hxx-x8q5', // ip-address
  'GHSA-4qv8-2p9j-2685', // undici EventSource parser
  'GHSA-xjj6-f33q-929c', // undici content encodings
]);
const UNPATCHED_ADVISORIES = new Map([
  // braces <= 3.0.3, the latest release: stack exhaustion from deeply nested brace
  // patterns. It is reached only through Node tooling that expands globs from
  // checked-in config: micromatch in semantic-release (.releaserc.json), and
  // chokidar, fast-glob and micromatch in the frontend build and test tooling
  // (Angular devkit, Tailwind, Karma). None of it ships in the browser bundle.
  ['GHSA-vfj7-8cjw-p6xm', { pkg: 'braces', reason: 'no patched release; globs come only from checked-in tooling config' }],
]);

if (new Date().toISOString().slice(0, 10) > ALLOWLIST_EXPIRES) {
  console.error(`npm audit allowlist expired on ${ALLOWLIST_EXPIRES}. Check whether a newer npm fixes the bundled dependencies (issue 927), then update or remove the allowlist in .github/scripts/audit-release-tooling.mjs.`);
  process.exit(1);
}

let output;
try {
  output = execFileSync('npm', ['audit', '--json'], { encoding: 'utf8' });
} catch (err) {
  // npm audit exits non-zero when it finds anything; the JSON is still on stdout.
  output = err.stdout;
}
const report = JSON.parse(output);
if (report.error) {
  console.error('npm audit failed:', JSON.stringify(report.error));
  process.exit(1);
}

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
  const bundled = vuln.nodes.every((node) => node.startsWith(BUNDLED_NPM_PREFIX));
  return advisoriesOf(vuln).every((id) => UNPATCHED_ADVISORIES.get(id)?.pkg === name || (bundled && ALLOWED_ADVISORIES.has(id)));
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
