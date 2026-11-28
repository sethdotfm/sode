import test from 'node:test';
import assert from 'node:assert/strict';

import './domstub.mjs';
const { replacedOn } = await import('../gridpanelconfirm.js');

test('a source already routed replaces nothing', () => {
  const now = [{ src: 7, levels: [1, 2] }];
  assert.deepEqual(replacedOn(now, 7, new Set([1, 2])), []);
});

test('another source on the same levels is replaced, one on other levels is not', () => {
  const now = [
    { src: 7, levels: [1] },
    { src: 8, levels: [2] },
    { src: 9, levels: [3] },
  ];
  assert.deepEqual(replacedOn(now, 7, new Set([1, 2])), [{ src: 8, levels: [2] }]);
  assert.deepEqual(replacedOn(undefined, 7, new Set([1])), []);
});
