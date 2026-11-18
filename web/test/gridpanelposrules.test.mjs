import { test } from 'node:test';
import assert from 'node:assert/strict';
import { thumbOf, rangeText } from '../gridpanelposrules.js';

test('a thumb is never too short to see and stays on its track', () => {
  const s = { offset: 0, max: 140000, n: 2500, first: 1, last: 5 };
  assert.deepEqual(thumbOf(s, 300), { size: 32, at: 0 });
  assert.equal(thumbOf({ ...s, offset: s.max }, 300).at, 268);
  assert.equal(thumbOf({ ...s, offset: s.max * 2 }, 300).at, 268);
});

test('a short list gets a thumb in proportion, and none to move with nothing to scroll', () => {
  assert.equal(thumbOf({ offset: 0, max: 300 }, 300).size, 150);
  assert.deepEqual(thumbOf({ offset: 0, max: 0 }, 300), { size: 300, at: 0 });
});

test('the range words', () => {
  assert.equal(rangeText({ first: 12, last: 16, n: 2500 }), '12–16/2500');
  assert.equal(rangeText({ first: 0, last: 0, n: 0 }), '');
});
