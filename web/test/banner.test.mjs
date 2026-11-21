import test from 'node:test';
import assert from 'node:assert/strict';

import './domstub.mjs';

test('dismissMs: only a success closes itself, and only with a time set', async () => {
  const { dismissMs } = await import('./all.mjs');
  assert.equal(dismissMs('ok', 8), 8000);
  assert.equal(dismissMs('ok', 0), 0);
  assert.equal(dismissMs('error', 8), 0);
  assert.equal(dismissMs('warn', 8), 0);
  assert.equal(dismissMs('info', 8), 0);
});

test('every result gets its own id, so a replaced banner is a different one', async () => {
  const { S, showResult, setResult } = await import('./all.mjs');
  showResult('ok', 'a', 'x');
  const a = S.result.id;
  showResult('ok', 'b', 'y');
  assert.notEqual(S.result.id, a);
  const b = S.result.id;
  setResult({ kind: 'ok', title: 'c', text: 'z', track: null });
  assert.ok(S.result.id > b);
});
