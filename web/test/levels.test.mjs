// Tests for the levels language (levels boxes, default levels). Not embedded.
// docker run --rm -v "$PWD/web":/w node:22-alpine node --test '/w/test/*.test.mjs'
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseLevelsAs } from '../levels.js';

const p = (text, mode = '1', max = 17) => parseLevelsAs(text, mode, max);
const range = (a, b) => Array.from({ length: b - a + 1 }, (_, i) => a + i);

test('lists, ranges and leaving out', () => {
  assert.deepEqual(p('1-3,5'), [1, 2, 3, 5]);
  assert.deepEqual(p('1+3+5-8'), [1, 3, 5, 6, 7, 8]);
  assert.deepEqual(p('3-1'), [1, 2, 3]);
  assert.deepEqual(p('!1'), range(2, 17));
  assert.deepEqual(p('1-8!3'), [1, 2, 4, 5, 6, 7, 8]);
  assert.deepEqual(p('1-8 !3'), [1, 2, 4, 5, 6, 7, 8]);
});

test('words', () => {
  assert.deepEqual(p('1 thru 8 not 3'), [1, 2, 4, 5, 6, 7, 8]);
  assert.deepEqual(p('1 t 4 n 2'), [1, 3, 4]);
  assert.deepEqual(p('1 and 2'), [1, 2]);
  // "and" adds; "not" leaves out only the level after it.
  assert.deepEqual(p('1 through 8 not 3 and 5'), [1, 2, 4, 5, 6, 7, 8]);
  assert.deepEqual(p('1 through 4 and 9'), [1, 2, 3, 4, 9]);
  assert.deepEqual(p('1 through 4 not 2 and not 3'), [1, 4]);
});

test('number pad: . is a comma, .. a range', () => {
  assert.deepEqual(p('1..8.10'), [1, 2, 3, 4, 5, 6, 7, 8, 10]);
  assert.deepEqual(p('1.3.5'), [1, 3, 5]);
  assert.deepEqual(p('1...3'), [1, 2, 3]);
  assert.deepEqual(p('2..4 not 3'), [2, 4]);
});

test('numbering modes', () => {
  assert.deepEqual(p('0..3', '0'), [1, 2, 3, 4]);
  assert.deepEqual(p('0', '0'), [1]);
  assert.deepEqual(p('16', '0'), [17]);
  assert.deepEqual(p('v..a3', 'va'), [1, 2, 3, 4]);
  assert.deepEqual(p('v and a2', 'va'), [1, 3]);
  assert.deepEqual(p('0', 'va'), [1]);
  assert.deepEqual(p('not v', 'va'), range(2, 17));
});

test('invalid', () => {
  assert.equal(p('18'), null);
  assert.equal(p('0'), null);
  assert.equal(p('17', '0'), null);
  assert.equal(p('1..'), null);
  assert.equal(p('abc'), null);
  assert.equal(p('1-2-3'), null);
  assert.equal(p('!1-17'), null); // everything left out
  assert.equal(p(''), null);
});
