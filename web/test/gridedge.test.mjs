// A paste past the edge of the grid (gridedge.js): Clip and Wrap. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createGrid, select, FILL_ORDERS } from '../gridmodel.js';
import { pastePlan, applyPaste } from '../gridclip.js';
import { clipPaste, wrapPaste, edgeChoices, edgeNote } from '../gridedge.js';

const grid = (cells, size = { rows: 4, cols: 8 }) => createGrid({ ...size, cells });
const at = (g, r, c) => select(g, r, c);
const block = (cells, rows, cols) => ({ sode: 'sode-panel-cells', version: 1, panel: 'p-1', rows, cols, cells });
const spots = (placed) => placed.map((c) => `${c.r},${c.c} ${c.rs}x${c.cs} ${c.kind}`);
const one = (r, c, kind = 'take') => ({ r, c, rs: 1, cs: 1, kind });

test('Clip drops cells wholly outside and shrinks a merged cell partly outside', () => {
  const g = at(grid([]), 3, 7);
  const b = block([one(1, 1), { r: 1, c: 2, rs: 2, cs: 3, kind: 'clear' }, one(2, 1, 'source'), one(3, 1, 'preset'), one(1, 4)], 3, 4);
  const plan = pastePlan(g, b, { panel: 'p-1' });
  const out = clipPaste(g, plan);
  assert.deepEqual(spots(out.placed), ['3,7 1x1 take', '3,8 2x1 clear', '4,7 1x1 source']);
  assert.deepEqual([out.dropped, out.shrunk], [2, 1]);
  assert.equal(out.dropped + out.shrunk, plan.outside);
});

test('Clip keeps every field of a cell it shrinks', () => {
  const g = at(grid([]), 4, 8);
  const b = block([{ r: 1, c: 1, rs: 2, cs: 2, kind: 'source', n: 3, caption: 'Cam', levels: [1] }], 2, 2);
  const out = clipPaste(g, pastePlan(g, b, { panel: 'p-1' }));
  assert.deepEqual(out.placed, [{ r: 4, c: 8, rs: 1, cs: 1, kind: 'source', n: 3, caption: 'Cam', levels: [1] }]);
});

// The six fill orders over a 3 by 3 grid, from square 2,2, with a 1 by 4 block of single cells.
const WALKS = {
  across: ['2,2', '2,3', '3,1', '3,2'],
  'across-rev': ['2,2', '2,1', '3,3', '3,2'],
  'snake-across': ['2,2', '2,1', '3,1', '3,2'],
  down: ['2,2', '3,2', '1,3', '2,3'],
  'down-rev': ['2,2', '1,2', '3,3', '2,3'],
  'snake-down': ['2,2', '1,2', '1,3', '2,3'],
};

test('Wrap walks every fill order from the paste square, in the block reading order', () => {
  assert.deepEqual(Object.keys(WALKS).sort(), FILL_ORDERS.map((o) => o.id).sort());
  const g = at(grid([], { rows: 3, cols: 3 }), 2, 2);
  const b = block(['take', 'clear', 'source', 'preset'].map((k, i) => one(1, i + 1, k)), 1, 4);
  const plan = pastePlan(g, b, { panel: 'p-1' });
  for (const [order, want] of Object.entries(WALKS)) {
    const out = wrapPaste(g, plan, order);
    assert.deepEqual(
      out.placed.map((c) => `${c.r},${c.c}`),
      want,
      order,
    );
    assert.deepEqual(
      out.placed.map((c) => c.kind),
      ['take', 'clear', 'source', 'preset'],
      order,
    );
    assert.equal(out.dropped, 0, order);
  }
});

test('Wrap skips blank squares of the block and counts the cells it moves', () => {
  const g = at(grid([]), 1, 7);
  const b = block([one(1, 1), one(1, 3, 'clear'), one(2, 2, 'source')], 2, 3);
  const out = wrapPaste(g, pastePlan(g, b, { panel: 'p-1' }), 'across');
  assert.deepEqual(spots(out.placed), ['1,7 1x1 take', '1,8 1x1 clear', '2,1 1x1 source']);
  assert.deepEqual([out.moved, out.shrunk, out.dropped], [2, 0, 0]);
});

test('Wrap places a merged cell whole where it fits and steps over the squares it covers', () => {
  const g = at(grid([]), 1, 7);
  const b = block([one(1, 1), { r: 1, c: 2, rs: 2, cs: 2, kind: 'clear' }, one(1, 4, 'source')], 2, 4);
  const out = wrapPaste(g, pastePlan(g, b, { panel: 'p-1' }), 'across');
  // 1,8 cannot hold a 2 by 2 cell, so it shrinks; then 2,1 and 2,2.
  assert.deepEqual(spots(out.placed), ['1,7 1x1 take', '1,8 1x1 clear', '2,1 1x1 source']);
  assert.equal(out.shrunk, 1);

  const h = at(grid([]), 1, 6);
  const fits = wrapPaste(h, pastePlan(h, b, { panel: 'p-1' }), 'across');
  assert.deepEqual(spots(fits.placed), ['1,6 1x1 take', '1,7 2x2 clear', '2,1 1x1 source']);
  assert.equal(fits.shrunk, 0);
  const next = applyPaste(h, fits.placed);
  assert.equal(next.cells.length, 3);
});

test('Wrap shrinks a merged cell that would cover a square already placed', () => {
  // Right to left: the 1 by 2 cell lands on 1,7 after 1,8, and would run back over 1,8.
  const g = at(grid([]), 1, 8);
  const b = block([one(1, 1), { r: 1, c: 2, rs: 1, cs: 2, kind: 'clear' }], 1, 3);
  const out = wrapPaste(g, pastePlan(g, b, { panel: 'p-1' }), 'across-rev');
  assert.deepEqual(spots(out.placed), ['1,8 1x1 take', '1,7 1x1 clear']);
  assert.equal(out.shrunk, 1);
});

test('Wrap drops what finds no square before the last square of the order', () => {
  const g = at(grid([], { rows: 2, cols: 2 }), 2, 2);
  const b = block([one(1, 1), one(1, 2, 'clear'), one(1, 3, 'source')], 1, 3);
  const out = wrapPaste(g, pastePlan(g, b, { panel: 'p-1' }), 'across');
  assert.deepEqual(spots(out.placed), ['2,2 1x1 take']);
  assert.equal(out.dropped, 2);
});

test('the choices name the fill order and the counts, and the note says what happened', () => {
  const b = block([one(1, 1), one(1, 2, 'clear'), { r: 2, c: 1, rs: 1, cs: 2, kind: 'source' }], 2, 2);
  const g = at(grid([]), 4, 7);
  const plan = pastePlan(g, b, { panel: 'p-1' });
  const c = edgeChoices(g, plan, 'down');
  assert.equal(c.orderName, 'Down, then across');
  assert.equal(c.clip.text, 'Clip drops 1 cell that falls outside the grid.');
  assert.equal(c.wrap.text, 'Wrap moves 2 cells along Down, then across, and shrinks 1 merged cell.');
  assert.equal(edgeNote(plan, c.clip), 'Pasted 2 cells. Clip dropped 1 cell.');

  const h = at(grid([]), 1, 8);
  const p2 = pastePlan(h, b, { panel: 'p-1' });
  const c2 = edgeChoices(h, p2, 'across');
  assert.equal(c2.clip.text, 'Clip drops 1 cell that falls outside the grid and shrinks 1 merged cell.');
  assert.equal(c2.wrap.text, 'Wrap moves 2 cells along Across, then down.');
  assert.equal(edgeNote(p2, c2.wrap), 'Pasted 3 cells. Wrap moved 2 cells along Across, then down.');

  const k = at(grid([], { rows: 2, cols: 2 }), 2, 2);
  const p3 = pastePlan(k, block([one(1, 1), one(1, 2, 'clear'), one(1, 3, 'source')], 1, 3), { panel: 'p-1' });
  const c3 = edgeChoices(k, p3, 'across');
  assert.equal(c3.wrap.text, 'Wrap moves 0 cells along Across, then down, and drops 2 that find no square.');
  assert.equal(edgeNote(p3, c3.wrap), 'Pasted 1 cell. Wrap moved 0 cells along Across, then down, and dropped 2 that found no square.');
});

test('the note keeps what a paste that fits would say about images', () => {
  const g = at(grid([]), 1, 8);
  const b = { ...block([{ ...one(1, 1), art: 'a-0123abcd' }, one(1, 2, 'clear')], 1, 2), panel: 'p-2' };
  const plan = pastePlan(g, b, { panel: 'p-1' });
  const c = edgeChoices(g, plan, 'across');
  assert.match(edgeNote(plan, c.clip), /^Pasted 1 cell\. 1 cell lost its uploaded image.* Clip dropped 1 cell\.$/);
});

test('an unknown order wraps across, the default Find starts on', () => {
  const g = at(grid([]), 1, 8);
  const b = block([one(1, 1), one(1, 2, 'clear')], 1, 2);
  const c = edgeChoices(g, pastePlan(g, b, { panel: 'p-1' }), 'sideways');
  assert.equal(c.orderName, 'Across, then down');
  assert.deepEqual(spots(c.wrap.placed), ['1,8 1x1 take', '2,1 1x1 clear']);
});
