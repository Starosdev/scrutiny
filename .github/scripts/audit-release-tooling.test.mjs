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
];

function audit(name, id, nodes, date = '2026-10-24') {
  const report = { vulnerabilities: {
    [name]: { severity: 'high', via: [{ url: `https://github.com/advisories/${id}` }], nodes },
  } };
  // Run the real entrypoint with a deterministic audit response and clock.
  // Throwing stdout reproduces npm audit's nonzero vulnerability exit.
  return spawnSync(process.execPath, ['--input-type=module', '-e', `
    import childProcess from 'node:child_process';
    import { syncBuiltinESMExports } from 'node:module';
    childProcess.execFileSync = () => { throw { stdout: ${JSON.stringify(JSON.stringify(report))} }; };
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
  const result = audit('undici', advisories[1][1], [`${bundled}undici`], '2026-11-02');
  assert.equal(result.status, 1);
  assert.match(result.stderr, /allowlist expired on 2026-11-01/);
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
