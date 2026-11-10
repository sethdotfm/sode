// The editor's Artwork tool (arttoolrules.js): Icon A on every selected cell, Icon B one glyph per cell in the fill order, Clear. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { artToolPlan, applyArtTool } from '../arttoolrules.js';
import { createGrid, select, cellAt } from '../gridmodel.js';

// A 2x3 block of Source cells, all selected.
const block = (cells) => {
  const g = createGrid({
    rows: 2,
    cols: 3,
    cells: cells || [1, 2].flatMap((r) => [1, 2, 3].map((c) => ({ r, c, rs: 1, cs: 1, kind: 'source', n: r * 10 + c }))),
  });
  return select(select(g, 1, 1), 2, 3, true);
};
const glyphs = (g, order) => order.map(([r, c]) => cellAt(g, r, c)?.artB ?? '-').join(' ');
const READING = [
  [1, 1],
  [1, 2],
  [1, 3],
  [2, 1],
  [2, 2],
  [2, 3],
];

test('Icon A goes on every selected cell and leaves Icon B as it was', () => {
  const g0 = block();
  const g = { ...g0, cells: g0.cells.map((c) => (c.r === 1 && c.c === 1 ? { ...c, artB: '3' } : c)) };
  const plan = artToolPlan(g, { a: 'icon:camera' });
  assert.equal(plan.slots.length, 6);
  assert.equal(plan.empty, 0);
  assert.equal(plan.left, 0);
  const out = applyArtTool(g, plan);
  assert.ok(out.cells.every((c) => c.art === 'icon:camera'));
  assert.equal(cellAt(out, 1, 1).artB, '3');
});

test('Icon A can be an uploaded image id', () => {
  const out = applyArtTool(block(), artToolPlan(block(), { a: 'a-0123abcd' }));
  assert.ok(out.cells.every((c) => c.art === 'a-0123abcd'));
});

test('Icon B gives each cell the next glyph in every fill order', () => {
  const cases = {
    across: '1 2 3 4 5 6',
    'across-rev': '3 2 1 6 5 4',
    'snake-across': '1 2 3 6 5 4',
    down: '1 3 5 2 4 6',
    'down-rev': '2 4 6 1 3 5',
    'snake-down': '1 4 5 2 3 6',
  };
  for (const [order, want] of Object.entries(cases)) {
    const g = block();
    const out = applyArtTool(g, artToolPlan(g, { b: '1' }, order));
    assert.equal(glyphs(out, READING), want, order);
  }
});

test('Icon B leaves Icon A as it was', () => {
  const g0 = block();
  const g = { ...g0, cells: g0.cells.map((c) => ({ ...c, art: 'icon:mic' })) };
  const out = applyArtTool(g, artToolPlan(g, { b: 'A' }));
  assert.ok(out.cells.every((c) => c.art === 'icon:mic'));
  assert.equal(glyphs(out, READING), 'A B C D E F');
});

test('Icon B stops at 16 and never runs on into A; the cells past the end are counted', () => {
  const g = block();
  const plan = artToolPlan(g, { b: '14' });
  assert.equal(plan.slots.length, 3);
  assert.equal(plan.left, 3);
  assert.equal(glyphs(applyArtTool(g, plan), READING), '14 15 16 - - -');
});

test('Icon B stops at Z and does not wrap', () => {
  const g = block();
  const plan = artToolPlan(g, { b: 'Y' });
  assert.equal(plan.left, 4);
  assert.equal(glyphs(applyArtTool(g, plan), READING), 'Y Z - - - -');
});

test('a cell past the end keeps the Icon B it had', () => {
  const g0 = block();
  const g = { ...g0, cells: g0.cells.map((c) => (c.r === 2 && c.c === 3 ? { ...c, artB: 'Q' } : c)) };
  assert.equal(cellAt(applyArtTool(g, artToolPlan(g, { b: 'Z' })), 2, 3).artB, 'Q');
});

test('empty squares are skipped, counted and take no glyph; an unassigned cell still gets artwork', () => {
  const g = block([
    { r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n: 1 },
    { r: 1, c: 3, rs: 1, cs: 1, kind: 'source' },
    { r: 2, c: 2, rs: 1, cs: 1, kind: 'take' },
  ]);
  const plan = artToolPlan(g, { b: '0' });
  assert.equal(plan.empty, 3);
  assert.equal(plan.left, 0);
  assert.equal(glyphs(applyArtTool(g, plan), READING), '0 - 1 - 2 -');
  assert.equal(applyArtTool(g, plan).cells.length, 3, 'no cell is made for an empty square');
});

test('a merged cell is one cell, and its covered squares are not empty', () => {
  const g = block([
    { r: 1, c: 1, rs: 2, cs: 2, kind: 'take' },
    { r: 1, c: 3, rs: 1, cs: 1, kind: 'clear' },
    { r: 2, c: 3, rs: 1, cs: 1, kind: 'current' },
  ]);
  const plan = artToolPlan(g, { b: '5' });
  assert.equal(plan.empty, 0);
  assert.deepEqual(
    plan.slots.map((s) => [s.r, s.c, s.patch.artB]),
    [
      [1, 1, '5'],
      [1, 3, '6'],
      [2, 3, '7'],
    ],
  );
});

test('Clear drops the cell own artwork and Hide text so a Default artwork shows through', () => {
  const g0 = block();
  const g = { ...g0, cells: g0.cells.map((c) => ({ ...c, art: 'icon:mic', artB: '2', hideText: true, caption: 'Mic' })) };
  const out = applyArtTool(g, artToolPlan(g, { clear: true }));
  for (const c of out.cells) {
    assert.ok(!('art' in c) && !('artB' in c) && !('hideText' in c));
    assert.equal(c.caption, 'Mic');
  }
});

test('no selection or no choice is no plan, and an unknown glyph is no plan', () => {
  assert.equal(artToolPlan(createGrid(), { a: 'icon:mic' }), null);
  assert.equal(artToolPlan(block(), {}), null);
  assert.equal(artToolPlan(block(), { b: 'zz' }), null);
  const g = block();
  assert.equal(applyArtTool(g, null), g);
});
