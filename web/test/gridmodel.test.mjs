// The editor's grid model (gridmodel.js): resize, select, type, merge, unmerge, fill orders, numbering, unassigned.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  ROWS_MAX,
  COLS_MAX,
  createGrid,
  cellAt,
  resize,
  cellsLost,
  select,
  deselect,
  rectOf,
  selCells,
  selSquares,
  single,
  setKind,
  setCell,
  clearSelection,
  canMerge,
  merge,
  canUnmerge,
  unmerge,
  FILL_ORDERS,
  squareOrder,
  fillPlan,
  applyFill,
  extendPlan,
  applyExtend,
  isUnassigned,
  saved,
} from '../gridmodel.js';
import { cellsIn } from '../panelrules.js';

const S = (r, c, n, more = {}) => ({ r, c, kind: 'source', n, ...more });
const D = (r, c, n) => ({ r, c, kind: 'destination', n });
const grid = (cells = [], rows = 4, cols = 8) => createGrid({ rows, cols, cells });
const pick = (g, r1, c1, r2 = r1, c2 = c1) => select(select(g, r1, c1), r2, c2, true);
const at = (g, r, c) => cellAt(g, r, c);

test('a new grid is 4x8 and empty, and a stored panel loads with whole spans', () => {
  const g = createGrid();
  assert.deepEqual([g.rows, g.cols, g.cells.length], [4, 8, 0]);
  const h = createGrid({ rows: 2, cols: 3, cells: [S(1, 1, 0), { r: 2, c: 2, rs: 1, cs: 2, kind: 'take' }] });
  assert.deepEqual([h.cells[0].rs, h.cells[0].cs], [1, 1]);
  assert.equal(h.cells[1].cs, 2);
});

test('loading drops cells outside the grid or overlapping an earlier one, as the panel page does', () => {
  const g = createGrid({ rows: 2, cols: 2, cells: [S(1, 1, 0), S(1, 1, 1), S(3, 1, 2), { r: 1, c: 2, rs: 1, cs: 5, kind: 'take' }] });
  assert.equal(g.cells.length, 1);
});

test('resize clamps to 1..24, drops cells that start outside and clips spans that reach outside', () => {
  let g = grid([S(1, 1, 0), S(4, 8, 1), { r: 1, c: 5, cs: 4, kind: 'take' }]);
  assert.equal(cellsLost(g, 2, 6), 1);
  g = resize(g, 2, 6);
  assert.deepEqual([g.rows, g.cols], [2, 6]);
  assert.equal(g.cells.length, 2);
  assert.equal(at(g, 1, 5).cs, 2);
  assert.deepEqual([resize(g, 99, 99).rows, resize(g, 99, 99).cols], [ROWS_MAX, COLS_MAX]);
  assert.deepEqual([resize(g, 0, -3).rows, resize(g, 0, -3).cols], [1, 1]);
  assert.deepEqual([resize(g, 'x', 3).rows, resize(g, 'x', 3).cols], [2, 3]);
});

test('growing the grid adds empty squares and changes no cell', () => {
  const g = grid([S(1, 1, 0)]);
  const h = resize(g, 6, 10);
  assert.deepEqual(h.cells, g.cells);
});

test('a resize that cuts the selection clears it', () => {
  let g = pick(grid(), 3, 7, 4, 8);
  assert.ok(resize(g, 2, 2).sel === null);
  g = pick(grid(), 1, 1, 2, 2);
  assert.ok(resize(g, 3, 3).sel);
});

test('click selects one square and selecting never changes the cells or the size', () => {
  const g = grid([S(1, 1, 0)]);
  const h = pick(g, 1, 1, 3, 4);
  assert.deepEqual(h.cells, g.cells);
  assert.deepEqual([h.rows, h.cols], [4, 8]);
  assert.deepEqual(rectOf(h), { r1: 1, c1: 1, r2: 3, c2: 4 });
  assert.equal(selSquares(h), 12);
});

test('dragging in any direction gives the same rectangle', () => {
  assert.deepEqual(rectOf(pick(grid(), 3, 4, 1, 2)), { r1: 1, c1: 2, r2: 3, c2: 4 });
});

test('a selection grows to take in every merged cell it touches', () => {
  const g = grid([{ r: 1, c: 1, rs: 2, cs: 2, kind: 'take' }]);
  assert.deepEqual(rectOf(pick(g, 2, 2, 2, 3)), { r1: 1, c1: 1, r2: 2, c2: 3 });
  assert.equal(single(pick(g, 2, 2)).kind, 'take');
});

test('select off the grid is ignored, deselect clears, and single needs exactly one cell', () => {
  const g = grid([S(1, 1, 0), S(1, 2, 1)]);
  assert.equal(select(g, 9, 9).sel, null);
  assert.equal(deselect(pick(g, 1, 1)).sel, null);
  assert.equal(single(pick(g, 1, 1)).n, 0);
  assert.equal(single(pick(g, 1, 1, 1, 2)), null);
  assert.equal(single(pick(grid(), 1, 1)), null);
});

test('selCells lists the cells in the selection and not the ones beside it', () => {
  const g = pick(grid([S(1, 1, 0), S(1, 2, 1), S(1, 3, 2)]), 1, 1, 1, 2);
  assert.deepEqual(
    selCells(g).map((c) => c.n),
    [0, 1],
  );
});

test('Type gives every selected square the kind: empty squares get a cell, cells change kind', () => {
  let g = setKind(pick(grid([S(1, 1, 7)]), 1, 1, 1, 3), 'destination');
  assert.equal(g.cells.length, 3);
  assert.ok(g.cells.every((c) => c.kind === 'destination'));
  assert.equal(at(g, 1, 1).n, undefined);
});

test('Type to the kind a cell already has keeps its number', () => {
  const g = setKind(pick(grid([S(1, 1, 7)]), 1, 1), 'source');
  assert.equal(at(g, 1, 1).n, 7);
});

test('Type to another kind keeps caption, levels and span but drops the number', () => {
  const g = setKind(pick(grid([S(1, 1, 7, { caption: 'Cam', levels: [1] })]), 1, 1), 'destination');
  const c = at(g, 1, 1);
  assert.deepEqual([c.kind, c.n, c.caption, c.levels], ['destination', undefined, 'Cam', [1]]);
});

test('Type ignores a kind the panel page does not know, and a Take cell needs no number', () => {
  const g = pick(grid(), 1, 1);
  assert.equal(setKind(g, 'nonsense'), g);
  assert.equal(setKind(g, 'take').cells.filter(isUnassigned).length, 0);
});

test('Type Empty and Delete remove the cells in the selection', () => {
  const g = pick(grid([S(1, 1, 0), S(1, 2, 1), S(2, 1, 2)]), 1, 1, 1, 2);
  assert.deepEqual(
    setKind(g, '').cells.map((c) => c.n),
    [2],
  );
  assert.deepEqual(
    clearSelection(g).cells.map((c) => c.n),
    [2],
  );
});

test('Type on a selection with a merged cell changes the whole merged cell once', () => {
  const g = grid([{ r: 1, c: 1, rs: 1, cs: 2, kind: 'take' }]);
  const h = setKind(pick(g, 1, 1), 'source');
  assert.equal(h.cells.length, 1);
  assert.equal(h.cells[0].cs, 2);
});

test('merge needs two squares, keeps the first cell settings and leaves one cell', () => {
  const g = pick(grid([S(1, 1, 3, { caption: 'A' }), S(1, 2, 4)]), 1, 1, 2, 2);
  assert.ok(canMerge(g));
  const m = merge(g);
  assert.equal(m.cells.length, 1);
  assert.deepEqual([m.cells[0].r, m.cells[0].c, m.cells[0].rs, m.cells[0].cs, m.cells[0].n, m.cells[0].caption], [1, 1, 2, 2, 3, 'A']);
  assert.deepEqual(rectOf(m), { r1: 1, c1: 1, r2: 2, c2: 2 });
});

test('merging empty squares makes a Take cell, the large button the panel is for', () => {
  const m = merge(pick(grid(), 1, 1, 2, 2));
  assert.equal(m.cells[0].kind, 'take');
});

test('merge of one square does nothing, and the merged cell reads back on the panel page', () => {
  const one = pick(grid([S(1, 1, 0)]), 1, 1);
  assert.equal(canMerge(one), false);
  assert.equal(merge(one), one);
  const m = merge(pick(grid([S(2, 2, 5)]), 2, 2, 3, 4));
  assert.equal(cellsIn(saved(m)).length, 1);
});

test('merge picks the first cell in reading order, not the order they were stored', () => {
  const g = pick(grid([S(1, 2, 9), S(1, 1, 8)]), 1, 1, 1, 2);
  assert.equal(merge(g).cells[0].n, 8);
});

test('unmerge splits to 1x1 at the top left, the others are empty, and the selection stays', () => {
  const g = pick(grid([{ r: 1, c: 1, rs: 2, cs: 2, kind: 'source', n: 4 }]), 1, 1);
  assert.ok(canUnmerge(g));
  const u = unmerge(g);
  assert.equal(u.cells.length, 1);
  assert.deepEqual([u.cells[0].rs, u.cells[0].cs, u.cells[0].n], [1, 1, 4]);
  assert.equal(at(u, 2, 2), undefined);
  assert.equal(canUnmerge(u), false);
  assert.equal(unmerge(u), u);
  assert.ok(u.sel);
});

test('setCell changes one cell by identity of position and drops a field set to undefined', () => {
  const g = grid([S(1, 1, 0, { caption: 'x' })]);
  const h = setCell(g, { r: 1, c: 1 }, { caption: undefined, n: 5 });
  assert.deepEqual([h.cells[0].n, 'caption' in h.cells[0]], [5, false]);
  assert.equal(g.cells[0].n, 0);
});

test('the six fill orders walk a 2x3 block as their icons say', () => {
  const q = { r1: 1, c1: 1, r2: 2, c2: 3 };
  const w = (o) =>
    squareOrder(q, o)
      .map(([r, c]) => `${r}${c}`)
      .join(' ');
  assert.deepEqual(
    FILL_ORDERS.map((o) => o.id),
    ['across', 'across-rev', 'snake-across', 'down', 'down-rev', 'snake-down'],
  );
  assert.equal(w('across'), '11 12 13 21 22 23');
  assert.equal(w('across-rev'), '13 12 11 23 22 21');
  assert.equal(w('snake-across'), '11 12 13 23 22 21');
  assert.equal(w('down'), '11 21 12 22 13 23');
  assert.equal(w('down-rev'), '21 11 22 12 23 13');
  assert.equal(w('snake-down'), '11 21 22 12 13 23');
});

test('a fill from one item onward places router numbers in order and leaves extra squares empty', () => {
  const g = pick(grid(), 1, 1, 1, 4);
  const p = fillPlan(g, { kind: 'source', items: [10, 11, 12], order: 'across' });
  assert.deepEqual(
    p.slots.map((s) => [s.r, s.c, s.n]),
    [
      [1, 1, 10],
      [1, 2, 11],
      [1, 3, 12],
    ],
  );
  assert.equal(p.dropped, 0);
  const h = applyFill(g, p);
  assert.equal(h.cells.length, 3);
  assert.equal(at(h, 1, 4), undefined);
  assert.ok(h.cells.every((c) => c.kind === 'source'));
});

test('a fill with more items than squares reports how many did not fit', () => {
  const p = fillPlan(pick(grid(), 1, 1, 1, 2), { kind: 'source', items: [0, 1, 2, 3], order: 'across' });
  assert.equal(p.slots.length, 2);
  assert.equal(p.dropped, 2);
});

test('a fill replaces the kind and number of existing cells but keeps their caption and span', () => {
  const g = pick(grid([D(1, 1, 3), { r: 1, c: 2, rs: 1, cs: 2, kind: 'take' }]), 1, 1, 1, 3);
  const h = applyFill(g, fillPlan(g, { kind: 'source', items: [0, 1], order: 'across' }));
  assert.deepEqual([at(h, 1, 1).kind, at(h, 1, 1).n], ['source', 0]);
  assert.deepEqual([at(h, 1, 2).kind, at(h, 1, 2).n, at(h, 1, 2).cs], ['source', 1, 2]);
});

test('a fill can skip squares that already hold a cell', () => {
  const g = pick(grid([S(1, 2, 99)]), 1, 1, 1, 3);
  const p = fillPlan(g, { kind: 'source', items: [0, 1], order: 'across', keep: true });
  assert.deepEqual(
    p.slots.map((s) => [s.c, s.n]),
    [
      [1, 0],
      [3, 1],
    ],
  );
});

test('a fill follows the chosen order, so snake down fills the columns back and forth', () => {
  const g = pick(grid(), 1, 1, 2, 2);
  const h = applyFill(g, fillPlan(g, { kind: 'source', items: [0, 1, 2, 3], order: 'snake-down' }));
  assert.deepEqual([at(h, 1, 1).n, at(h, 2, 1).n, at(h, 2, 2).n, at(h, 1, 2).n], [0, 1, 2, 3]);
});

test('a fill with no selection is empty', () => {
  assert.equal(fillPlan(grid(), { kind: 'source', items: [0], order: 'across' }), null);
});

test('dragging the grip right continues each row by its own step and kind', () => {
  const g = pick(grid([S(1, 1, 4), S(1, 2, 5), S(2, 1, 10), S(2, 2, 12)]), 1, 1, 2, 2);
  const p = extendPlan(g, { r1: 1, c1: 1, r2: 2, c2: 4 });
  assert.deepEqual(
    p.slots.map((s) => [s.r, s.c, s.n]),
    [
      [1, 3, 6],
      [1, 4, 7],
      [2, 3, 14],
      [2, 4, 16],
    ],
  );
  assert.deepEqual([p.first, p.last, p.count], [6, 16, 4]);
  const h = applyExtend(g, p);
  assert.deepEqual(rectOf(h), { r1: 1, c1: 1, r2: 2, c2: 4 });
  assert.equal(at(h, 2, 4).n, 16);
});

test('dragging the grip down or left continues columns downward or backward', () => {
  const g = pick(grid([S(1, 3, 4), S(1, 4, 5)]), 1, 3, 1, 4);
  const down = extendPlan(g, { r1: 1, c1: 3, r2: 3, c2: 4 });
  assert.deepEqual(
    down.slots.map((s) => [s.r, s.c, s.n]),
    [
      [2, 3, 5],
      [3, 3, 6],
      [2, 4, 6],
      [3, 4, 7],
    ],
  );
  const left = extendPlan(g, { r1: 1, c1: 1, r2: 1, c2: 4 });
  assert.deepEqual(
    left.slots.map((s) => [s.c, s.n]),
    [
      [2, 3],
      [1, 2],
    ],
  );
});

test('the grip stops before router number 0 is passed', () => {
  const g = pick(grid([S(1, 3, 1), S(1, 4, 2)]), 1, 3, 1, 4);
  const p = extendPlan(g, { r1: 1, c1: 1, r2: 1, c2: 4 });
  assert.deepEqual(
    p.slots.map((s) => [s.c, s.n]),
    [[2, 0]],
  );
  assert.deepEqual([p.first, p.last, p.count], [0, 0, 1]);
});

test('the grip skips a line whose edge cell has no number, and clamps to the grid', () => {
  const g = pick(grid([S(1, 1, 0), { r: 2, c: 1, kind: 'source' }]), 1, 1, 2, 1);
  const p = extendPlan(g, { r1: 1, c1: 1, r2: 2, c2: 3 });
  assert.deepEqual(
    p.slots.map((s) => [s.r, s.c]),
    [
      [1, 2],
      [1, 3],
    ],
  );
  const one = pick(grid([S(1, 1, 0)]), 1, 1);
  assert.equal(extendPlan(one, { r1: 1, c1: 1, r2: 1, c2: 99 }).slots.length, 7);
});

test('the grip needs a target that grows the selection on exactly one side', () => {
  const g = pick(grid([S(1, 1, 0)]), 1, 1);
  assert.equal(extendPlan(g, { r1: 1, c1: 1, r2: 1, c2: 1 }), null);
  assert.equal(extendPlan(g, { r1: 1, c1: 1, r2: 2, c2: 2 }), null);
  assert.equal(extendPlan(grid(), { r1: 1, c1: 1, r2: 1, c2: 2 }), null);
});

test('a cell with a kind but no number is unassigned, in reading order, and a Take is not', () => {
  const g = grid([S(2, 1, 0), { r: 1, c: 5, kind: 'destination' }, { r: 1, c: 2, kind: 'source' }, { r: 3, c: 3, kind: 'take' }]);
  assert.deepEqual(
    g.cells.filter(isUnassigned).map((c) => [c.r, c.c]).sort((a, b) => a[0] - b[0] || a[1] - b[1]),
    [
      [1, 2],
      [1, 5],
    ],
  );
});

test('saved panel cells are in reading order with spans, and unassigned cells stay (the page leaves them out)', () => {
  const g = grid([S(2, 1, 1), { r: 1, c: 3, kind: 'source' }, S(1, 1, 0)]);
  const out = saved(g);
  assert.deepEqual(
    out.cells.map((c) => [c.r, c.c]),
    [
      [1, 1],
      [1, 3],
      [2, 1],
    ],
  );
  assert.deepEqual([out.rows, out.cols], [4, 8]);
  assert.equal(cellsIn(out).length, 2);
  assert.ok(!('sel' in out));
});

test('the model never changes the grid it was given', () => {
  const g = pick(grid([S(1, 1, 0), S(1, 2, 1)]), 1, 1, 2, 2);
  const frozen = JSON.stringify(g);
  merge(g);
  setKind(g, 'take');
  resize(g, 1, 1);
  clearSelection(g);
  applyFill(g, fillPlan(g, { kind: 'source', items: [5, 6], order: 'down' }));
  assert.equal(JSON.stringify(g), frozen);
});

test('Destination cells set aside while Fixed destination is on come back where their squares are still free', async () => {
  const { setAside, putBack } = await import('../gridmodel.js');
  const g = createGrid({ rows: 2, cols: 4, cells: [D(1, 1, 3), S(1, 2, 1), { ...D(2, 1, 4), cs: 2 }, D(2, 4, 5)] });
  const { grid, held } = setAside(g, 'destination');
  assert.deepEqual(
    grid.cells.map((c) => c.kind),
    ['source'],
  );
  assert.equal(held.length, 3);
  assert.deepEqual(saved(putBack(grid, held)), saved(g));
  // A square filled meanwhile wins, and a cell that no longer fits is dropped, as on load.
  const taken = setKind(select(grid, 2, 2), 'take');
  assert.deepEqual(
    saved(putBack(resize(taken, 2, 3), held)).cells.map((c) => `${c.r},${c.c} ${c.kind}`),
    ['1,1 destination', '1,2 source', '2,2 take'],
  );
  assert.equal(putBack(grid, []), grid);
  // Two held cells on one square: the first wins, so the stored panel never has overlapping cells.
  assert.deepEqual(
    saved(
      putBack(
        grid,
        [3, 9].map((n) => ({ ...D(1, 1, n), rs: 1, cs: 1 })),
      ),
    ).cells.map((c) => `${c.r},${c.c} ${c.n}`),
    ['1,1 3', '1,2 1'],
  );
});

test('the held Destination cells that still fit are the ones putBack keeps, so the editor can count the rest', async () => {
  const { setAside, heldThatFit } = await import('../gridmodel.js');
  const g = createGrid({ rows: 2, cols: 4, cells: [D(1, 1, 3), S(1, 2, 1), { ...D(2, 1, 4), cs: 2 }, D(2, 4, 5)] });
  const { grid, held } = setAside(g, 'destination');
  assert.equal(heldThatFit(grid, held).length, 3);
  // A square filled meanwhile and a span that reaches past a shrunk edge each lose one.
  const n = (cells) => cells.map((c) => c.n);
  assert.deepEqual(n(heldThatFit(setKind(select(grid, 2, 2), 'take'), held)), [3, 5]);
  assert.deepEqual(n(heldThatFit(resize(grid, 2, 1), held)), [3]);
  // Two held cells on one square: the first is kept.
  assert.deepEqual(
    n(
      heldThatFit(
        grid,
        [3, 9].map((x) => ({ ...D(1, 1, x), rs: 1, cs: 1 })),
      ),
    ),
    [3],
  );
});
