// Which rows of a long list are drawn.
import test from 'node:test';
import assert from 'node:assert/strict';
import { windowAround, growEdge, WINDOW, STEP } from '../listwindow.js';

test('a short list is drawn whole', () => {
  assert.deepEqual(windowAround(40, 10), { lo: 0, hi: 40 });
  assert.deepEqual(windowAround(WINDOW, 0), { lo: 0, hi: WINDOW });
});

test('a long list is windowed around the row, clamped at both ends', () => {
  assert.deepEqual(windowAround(2690, 1000), { lo: 925, hi: 1075 });
  assert.deepEqual(windowAround(2690, 3), { lo: 0, hi: WINDOW });
  assert.deepEqual(windowAround(2690, 2689), { lo: 2690 - WINDOW, hi: 2690 });
});

test('growEdge adds a step at the end or start, none at the list ends', () => {
  assert.deepEqual(growEdge({ lo: 100, hi: 250 }, 2690, true), { lo: 100, hi: 250 + STEP });
  assert.deepEqual(growEdge({ lo: 100, hi: 250 }, 2690, false), { lo: 100 - STEP, hi: 250 });
  const end = { lo: 50, hi: 200 };
  assert.equal(growEdge(end, 200, true), end);
  const start = { lo: 0, hi: 150 };
  assert.equal(growEdge(start, 2690, false), start);
});
