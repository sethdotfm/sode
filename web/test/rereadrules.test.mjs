import { test } from 'node:test';
import assert from 'node:assert/strict';
import { namesChange, changeText } from '../rereadrules.js';

const L = (...names) => names.map((name, i) => ({ n: i, name }));

test('namesChange counts new names, new numbers and removed numbers per side', () => {
  const before = { srcs: L('Cam 1', 'Cam 2'), dsts: L('Mon 1', 'Mon 2', 'Mon 3') };
  assert.deepEqual(namesChange(before, before), { srcs: 0, dsts: 0 });
  assert.deepEqual(namesChange(before, { srcs: L('Cam 1', 'Cam X', 'Cam 3'), dsts: L('Mon 1') }), { srcs: 2, dsts: 2 });
});

test('changeText says what changed and the totals', () => {
  const total = { srcs: 200, dsts: 1 };
  assert.equal(changeText({ srcs: 0, dsts: 0 }, total), 'No names changed. The router has 200 sources and 1 destination.');
  assert.equal(changeText({ srcs: 1, dsts: 0 }, total), '1 source changed. The router has 200 sources and 1 destination.');
  assert.equal(changeText({ srcs: 3, dsts: 2 }, total), '3 sources and 2 destinations changed. The router has 200 sources and 1 destination.');
});
