// Fill cascade (gridedge.js cascadeWrap): Wrap's cells past a Page's last square go on to later Pages. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createGrid, select } from '../gridmodel.js';
import { pastePlan } from '../gridclip.js';
import { edgeChoices, edgeNote, cascadeWrap } from '../gridedge.js';

const one = (r, c, kind = 'take') => ({ r, c, rs: 1, cs: 1, kind });
const block = (n) => ({ sode: 'sode-panel-cells', version: 1, panel: 'p-1', rows: 1, cols: n, cells: Array.from({ length: n }, (_, i) => one(1, i + 1)) });
const size = { rows: 2, cols: 2 };
const page = (id, cells = [], enabled = true) => ({ id, enabled, cells });
const spots = (cells) => cells.map((c) => `${c.r},${c.c}`);

// A 2 by 2 grid, pasting 7 cells at the last square of Page 1 leaves 6 for later Pages.
function setup(n = 7, pages) {
  const g = select(createGrid({ ...size, cells: [] }), 2, 2);
  const plan = pastePlan(g, block(n), { panel: 'p-1' });
  return { g, plan, pages };
}

test('the rest goes on to the next enabled blank Page, from its first square', () => {
  const { g, plan } = setup(5);
  const pages = [page('a'), page('b')];
  const c = edgeChoices(g, plan, 'across', { pages, from: 0 });
  assert.deepEqual(spots(c.wrap.placed), ['2,2']);
  assert.equal(c.wrap.onto.length, 1);
  assert.equal(c.wrap.onto[0].index, 1);
  assert.deepEqual(spots(c.wrap.onto[0].cells), ['1,1', '1,2', '2,1', '2,2']);
  assert.equal(c.wrap.dropped, 0);
});

test('disabled and used Pages are skipped; the cascade stops at the last Page and never returns to Page 1', () => {
  const pages = [page('a'), page('b', [], false), page('c', [one(1, 1)]), page('d')];
  const out = cascadeWrap(pages, 0, Array.from({ length: 6 }, (_, i) => one(1, i + 1)), 'across', size);
  assert.deepEqual(
    out.onto.map((o) => o.index),
    [3],
  );
  assert.equal(out.left, 2);
  // From the last Page nothing goes anywhere, Page 1 included.
  const last = cascadeWrap([page('a'), page('b')], 1, [one(1, 1)], 'across', size);
  assert.deepEqual([last.onto.length, last.left], [0, 1]);
});

test('the dialog and the status line count the Pages carried on to and the cells cut off', () => {
  const { g, plan } = setup(7);
  const pages = [page('a'), page('b')];
  const c = edgeChoices(g, plan, 'across', { pages, from: 0 });
  assert.equal(c.wrap.dropped, 2);
  assert.match(c.wrap.text, /carries 4 cells on to Page 2/);
  assert.match(c.wrap.text, /cuts off 2 that do not fit/);
  assert.match(edgeNote(plan, c.wrap), /^Pasted 5 cells\..*carried 4 cells on to Page 2\. 2 cells were left over and cut off\./);
});

test('without later Pages Wrap reads as before', () => {
  const { g, plan } = setup(3);
  const c = edgeChoices(g, plan, 'across');
  assert.equal(c.wrap.dropped, 2);
  assert.doesNotMatch(c.wrap.text, /carries/);
});
