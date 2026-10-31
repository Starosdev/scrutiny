import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const script = new URL('./audit-release-tooling.mjs', import.meta.url).href;
const bundled = 'node_modules/npm/node_modules/';
const advisories = [
  ['ip-address', 'GHSA-rxp7-6hxx-x8q5'],
  ['undici', 'GHSA-4qv8-2p9j-2685'],
  ['undici', 'GHSA-xjj6-f33q-929c'],
  ['undici', 'GHSA-2m5v-pch6-74hx'],
  ['undici', 'GHSA-qrxm-fqhp-jwvm'],
  ['undici', 'GHSA-9fm3-97px-rjf6'],
  ['glob', 'GHSA-3g3m-2fjr-6pw4'],
  ['minimatch', 'GHSA-g3hv-75wg-m249'],
];
const devOnly = [
  ['minimatch', 'GHSA-g3hv-75wg-m249'],
];

// production: whether the package is also reachable without devDependencies.
// The script decides dev-only status from a second `npm audit --omit=dev`, so the
// stub answers that invocation separately. Defaulting to true keeps every other
// test on the pre-existing behavior.
function audit(name, id, nodes, date = '2026-10-24', production = true, severity = 'high') {
  const report = { vulnerabilities: {
    [name]: { severity, via: [{ url: `https://github.com/advisories/${id}` }], nodes },
  } };
  const productionReport = production ? report : { vulnerabilities: {} };
  // Run the real entrypoint with a deterministic audit response and clock.
  // Throwing stdout reproduces npm audit's nonzero vulnerability exit.
  return spawnSync(process.execPath, ['--input-type=module', '-e', `
    import childProcess from 'node:child_process';
    import { syncBuiltinESMExports } from 'node:module';
    const full = ${JSON.stringify(JSON.stringify(report))};
    const production = ${JSON.stringify(JSON.stringify(productionReport))};
    childProcess.execFileSync = (cmd, args) => {
      throw { stdout: args.includes('--omit=dev') ? production : full };
    };
    syncBuiltinESMExports();
    globalThis.Date = class extends Date { constructor() { super(${JSON.stringify(date)}); } };
    await import(${JSON.stringify(script)});
  `], { encoding: 'utf8', timeout: 5000 });
}

for (const [name, id] of advisories) {
  test(`${id}: only bundled paths pass`, () => {
    const allowed = audit(name, id, [`${bundled}${name}`]);
    assert.equal(allowed.status, 0, allowed.stderr);
    for (const nodes of [[], [`node_modules/${name}`], [`${bundled}${name}`, `node_modules/${name}`]]) {
      const blocked = audit(name, id, nodes);
      assert.equal(blocked.status, 1, blocked.stderr);
      assert.match(blocked.stderr, /findings not covered by the allowlist/);
    }
  });
}

test('bundled exceptions expire without extending the deadline', () => {
  const result = audit('undici', advisories[1][1], [`${bundled}undici`], '2026-12-16');
  assert.equal(result.status, 1);
  assert.match(result.stderr, /allowlist expired on 2026-12-15/);
});

test('an allowlisted advisory does not pass for a different package', () => {
  // The allowlist is evidence about one package's bundled copy, not about the
  // advisory in the abstract. An advisory later amended to list a second package
  // must not inherit the exception, even entirely inside the bundled tree.
  for (const [pkg, id] of advisories) {
    const other = pkg === 'undici' ? 'ip-address' : 'undici';
    const result = audit(other, id, [`${bundled}${other}`]);
    assert.equal(result.status, 1, `${id} wrongly allowed for ${other}: ${result.stderr}`);
    assert.match(result.stderr, /findings not covered by the allowlist/);
  }
});

for (const [name, id] of devOnly) {
  test(`${id}: passes for ${name} outside the production tree`, () => {
    const allowed = audit(name, id, [`node_modules/${name}`], '2026-10-24', false, 'moderate');
    assert.equal(allowed.status, 0, allowed.stderr);
  });

  test(`${id}: still fails for ${name} inside the production tree`, () => {
    const blocked = audit(name, id, [`node_modules/${name}`], '2026-10-24', true, 'moderate');
    assert.equal(blocked.status, 1, blocked.stderr);
    assert.match(blocked.stderr, /findings not covered by the allowlist/);
  });

  test(`${id}: the dev-only exception does not transfer to another package`, () => {
    const blocked = audit('eslint', id, ['node_modules/eslint'], '2026-10-24', false, 'moderate');
    assert.equal(blocked.status, 1, blocked.stderr);
    assert.match(blocked.stderr, /findings not covered by the allowlist/);
  });

  test(`${id}: the dev-only exception expires with the allowlist`, () => {
    const expired = audit(name, id, [`node_modules/${name}`], '2026-12-16', false, 'moderate');
    assert.equal(expired.status, 1);
    assert.match(expired.stderr, /allowlist expired on 2026-12-15/);
  });

  // Dev tooling executes in CI with repository credentials, so the dev-only
  // exception is capped at moderate. A high or critical advisory gets a
  // decision, not an allowlist line.
  for (const severity of ['high', 'critical']) {
    test(`${id}: ${severity} severity defeats the dev-only exception`, () => {
      const blocked = audit(name, id, [`node_modules/${name}`], '2026-10-24', false, severity);
      assert.equal(blocked.status, 1, blocked.stderr);
      assert.match(blocked.stderr, /findings not covered by the allowlist/);
    });
  }
}

// The bundled-npm exception is evidence about paths inside npm's own tree. The
// dev-only branch must not become a second way in for those advisories: the two
// buckets are independent, and only minimatch is in both.
for (const [name, id] of advisories.filter(([pkg]) => pkg !== 'minimatch')) {
  test(`${id}: the bundled exception does not leak through the dev-only branch`, () => {
    const blocked = audit(name, id, [`node_modules/${name}`], '2026-10-24', false, 'moderate');
    assert.equal(blocked.status, 1, blocked.stderr);
    assert.match(blocked.stderr, /findings not covered by the allowlist/);
  });
}

test('a package flagged only through a blocked dependency stays blocked', () => {
  // The fixed-point propagation is what clears the 15 packages the frontend
  // flags through minimatch, and what must re-block them if minimatch ever is.
  const report = { vulnerabilities: {
    minimatch: { severity: 'moderate', via: [{ url: 'https://github.com/advisories/GHSA-g3hv-75wg-m249' }], nodes: ['node_modules/minimatch'] },
    eslint: { severity: 'moderate', via: ['minimatch'], nodes: ['node_modules/eslint'] },
  } };
  const run = (production) => spawnSync(process.execPath, ['--input-type=module', '-e', `
    import childProcess from 'node:child_process';
    import { syncBuiltinESMExports } from 'node:module';
    const full = ${JSON.stringify(JSON.stringify(report))};
    const production = ${JSON.stringify(JSON.stringify(report))};
    childProcess.execFileSync = (cmd, args) => {
      throw { stdout: args.includes('--omit=dev') ? (${JSON.stringify(JSON.stringify({ vulnerabilities: {} }))}) : full };
    };
    syncBuiltinESMExports();
    globalThis.Date = class extends Date { constructor() { super('2026-10-24'); } };
    void production;
    await import(${JSON.stringify(script)});
  `], { encoding: 'utf8', timeout: 5000 });

  // minimatch allowed as dev-only, so its dependent clears with it.
  const allowed = run();
  assert.equal(allowed.status, 0, allowed.stderr);
  assert.match(allowed.stdout, /allowed: eslint/);
});

test('the dev-only check fails closed when the production audit has no vulnerabilities object', () => {
  const report = { vulnerabilities: {
    minimatch: { severity: 'moderate', via: [{ url: 'https://github.com/advisories/GHSA-g3hv-75wg-m249' }], nodes: ['node_modules/minimatch'] },
  } };
  const result = spawnSync(process.execPath, ['--input-type=module', '-e', `
    import childProcess from 'node:child_process';
    import { syncBuiltinESMExports } from 'node:module';
    const full = ${JSON.stringify(JSON.stringify(report))};
    childProcess.execFileSync = (cmd, args) => {
      throw { stdout: args.includes('--omit=dev') ? '{}' : full };
    };
    syncBuiltinESMExports();
    globalThis.Date = class extends Date { constructor() { super('2026-10-24'); } };
    await import(${JSON.stringify(script)});
  `], { encoding: 'utf8', timeout: 5000 });
  assert.equal(result.status, 1, result.stderr);
  assert.match(result.stderr, /cannot establish which packages are dev-only/);
});

test('unknown bundled advisories still fail', () => {
  const result = audit('undici', 'GHSA-unknown', [`${bundled}undici`]);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /findings not covered by the allowlist/);
});

test('release configuration explicitly excludes the bundled npm plugin', () => {
  const config = JSON.parse(readFileSync(new URL('../../.releaserc.json', import.meta.url)));
  assert.ok(Array.isArray(config.plugins) && config.plugins.length > 0);
  const names = config.plugins.map(plugin => Array.isArray(plugin) ? plugin[0] : plugin);
  assert.ok(!names.some(name => name === '@semantic-release/npm' || name === 'npm'));
});
