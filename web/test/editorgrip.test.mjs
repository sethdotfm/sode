// The corner grip's rules (gridmodel.js dragRect, editorrules.js gripTip) and what Test it live shows (liveCells).
import test from 'node:test';
import assert from 'node:assert/strict';
import { createGrid, select, extendPlan, dragRect, liveCells, countUnassigned, nextUnassigned, rectOf } from '../gridmodel.js';
import { gripTip } from '../editorrules.js';
import { namesFrom } from '../panelrules.js';

const S = (r, c, n, more = {}) => ({ r, c, kind: 'source', n, ...more });
const grid = (cells = [], rows = 4, cols = 8) => createGrid({ rows, cols, cells });
const pick = (g, r1, c1, r2 = r1, c2 = c1) => select(select(g, r1, c1), r2, c2, true);

test('the grip grows the side the pointer is furthest past, and nothing else', () => {
  const g = pick(grid([S(2, 2, 0), S(2, 3, 1)]), 2, 2, 2, 3);
  assert.deepEqual(dragRect(g, 2, 6), { r1: 2, c1: 2, r2: 2, c2: 6 });
  assert.deepEqual(dragRect(g, 4, 3), { r1: 2, c1: 2, r2: 4, c2: 3 });
  assert.deepEqual(dragRect(g, 2, 1), { r1: 2, c1: 1, r2: 2, c2: 3 });
  assert.deepEqual(dragRect(g, 1, 2), { r1: 1, c1: 2, r2: 2, c2: 3 });
  assert.deepEqual(dragRect(g, 4, 8), { r1: 2, c1: 2, r2: 2, c2: 8 }, 'a tie goes across');
  assert.deepEqual(dragRect(g, 3, 8), { r1: 2, c1: 2, r2: 2, c2: 8 });
  assert.equal(dragRect(g, 2, 3), null, 'inside the selection');
  assert.equal(dragRect(grid(), 1, 1), null, 'no selection');
});

test('a drag rectangle feeds extendPlan', () => {
  const g = pick(grid([S(1, 1, 4), S(1, 2, 5)]), 1, 1, 1, 2);
  const p = extendPlan(g, dragRect(g, 1, 5));
  assert.deepEqual([p.first, p.last, p.count], [6, 8, 3]);
});

test('the tip names the first and last and counts, and says so when nothing fits', () => {
  const names = namesFrom({
    sources: [
      { n: 6, name: 'CAM 6' },
      { n: 8, name: 'CAM 8' },
    ],
  });
  const g = pick(grid([S(1, 1, 4), S(1, 2, 5)]), 1, 1, 1, 2);
  const p = extendPlan(g, dragRect(g, 1, 5));
  assert.equal(gripTip(p, names.of), '3 squares, CAM 6 to CAM 8');
  const one = extendPlan(g, dragRect(g, 1, 3));
  assert.equal(gripTip(one, names.of), '1 square, CAM 6');
  assert.equal(gripTip({ slots: [] }, names.of), 'Nothing to add here');
  assert.equal(gripTip(null, names.of), '');
});

test('Test it live leaves unassigned cells out, and the toolbar count and Next still find them', () => {
  const g = grid([S(1, 1, 0), { r: 1, c: 2, kind: 'source' }, { r: 1, c: 3, kind: 'destination' }, { r: 2, c: 1, kind: 'take' }]);
  assert.deepEqual(
    liveCells(g).map((c) => [c.r, c.c]),
    [
      [1, 1],
      [2, 1],
    ],
  );
  assert.equal(countUnassigned(g), 2);
  assert.deepEqual(rectOf(nextUnassigned(g)), { r1: 1, c1: 2, r2: 1, c2: 2 });
});
