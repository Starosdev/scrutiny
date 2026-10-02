#!/usr/bin/env node
// Runs `npm audit` for the root release tooling and fails on any finding except
// the allowlisted advisories below.
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
import { execFileSync } from 'node:child_process';

const ALLOWLIST_EXPIRES = '2026-11-01';
const BUNDLED_NPM_PREFIX = 'node_modules/npm/node_modules/';
const ALLOWED_ADVISORIES = new Set([
  // brace-expansion
  'GHSA-q2hr-2g5m-vwhr',
  'GHSA-qhr7-859c-m2p7',
  'GHSA-6j4f-fj2g-mc7p',
  // ip-address
  'GHSA-rpw4-54j3-4h4q',
  'GHSA-2vr4-cq9g-pvrc',
  'GHSA-j6r3-76f7-8jcv',
  'GHSA-h3mg-xc3c-68pw',
  // undici
  'GHSA-3wwx-pv8p-q78v',
  'GHSA-r53p-7pc4-xj5r',
  'GHSA-rfgv-xxqx-mfg5',
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

const blocking = [];
for (const [name, vuln] of Object.entries(report.vulnerabilities ?? {})) {
  const bundled = vuln.nodes.every((node) => node.startsWith(BUNDLED_NPM_PREFIX));
  const advisories = vuln.via.filter((via) => typeof via === 'object').map((via) => via.url.split('/').pop());
  const allowed = bundled && advisories.every((id) => ALLOWED_ADVISORIES.has(id));
  const line = `${name} (${vuln.severity}) ${advisories.join(', ') || 'via ' + vuln.via.join(', ')} at ${vuln.nodes.join(', ')}`;
  if (allowed) {
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
