import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';

// Execute the real policy with a fixed audit report and clock. No npm process
// or network request is needed to test the exception boundaries.
const source = readFileSync(new URL('./audit-release-tooling.mjs', import.meta.url), 'utf8')
  .replace("import { execFileSync } from 'node:child_process';", '');

function auditStatus(pkg, advisory, nodes, date = '2026-10-24') {
  const report = { vulnerabilities: {
    [pkg]: { severity: 'high', nodes, via: [{ url: `https://github.com/advisories/${advisory}` }] },
  } };
  const exit = {};
  let status = 0;
  try {
    runInNewContext(source, {
      execFileSync: () => JSON.stringify(report),
      Date: class extends Date { constructor() { super(date); } },
      console: { log() {}, error() {} },
      process: { exit(code) { status = code; throw exit; } },
    });
  } catch (error) {
    if (error !== exit) throw error;
  }
  return status;
}

for (const [pkg, advisory] of [
  ['ip-address', 'GHSA-rxp7-6hxx-x8q5'],
  ['undici', 'GHSA-4qv8-2p9j-2685'],
  ['undici', 'GHSA-xjj6-f33q-929c'],
]) {
  const bundled = `node_modules/npm/node_modules/${pkg}`;
  test(`${advisory}: bundled copy allowed before expiry`, () => {
    assert.equal(auditStatus(pkg, advisory, [bundled]), 0);
  });
  test(`${advisory}: non-bundled copy rejected`, () => {
    assert.equal(auditStatus(pkg, advisory, [`node_modules/${pkg}`]), 1);
  });
  test(`${advisory}: mixed bundled and non-bundled copies rejected`, () => {
    assert.equal(auditStatus(pkg, advisory, [bundled, `node_modules/${pkg}`]), 1);
  });
  test(`${advisory}: exception expires`, () => {
    assert.equal(auditStatus(pkg, advisory, [bundled], '2026-11-02'), 1);
  });
}

test('unknown bundled advisory remains blocked', () => {
  assert.equal(auditStatus('undici', 'GHSA-unknown', ['node_modules/npm/node_modules/undici']), 1);
});
