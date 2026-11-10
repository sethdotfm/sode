// The editor's undo stack for grid edits (gridundo.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { UNDO_MAX, emptyUndo, record, undo, redo, sameCells } from '../gridundo.js';

const grid = (n) => ({ rows: 4, cols: 8, cells: n ? [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n }] : [], sel: null });

test('an empty stack has nothing to undo or redo', () => {
  assert.equal(undo(emptyUndo(), grid(0)), null);
  assert.equal(redo(emptyUndo(), grid(0)), null);
});

test('undo gives back the grid before the edit, and redo the grid after it', () => {
  const h = record(emptyUndo(), { what: 'paste', before: grid(0) });
  const u = undo(h, grid(1));
  assert.deepEqual(u.grid, grid(0));
  assert.equal(u.what, 'paste');
  const r = redo(u.undo, grid(0));
  assert.deepEqual(r.grid, grid(1));
  assert.equal(r.what, 'paste');
  assert.equal(undo(u.undo, grid(0)), null);
  assert.equal(redo(r.undo, grid(1)), null);
});

test('steps undo newest first', () => {
  let h = emptyUndo();
  h = record(h, { what: 'paste', before: grid(0) });
  h = record(h, { what: 'cut', before: grid(1) });
  const a = undo(h, grid(2));
  assert.equal(a.what, 'cut');
  assert.deepEqual(a.grid, grid(1));
  const b = undo(a.undo, a.grid);
  assert.equal(b.what, 'paste');
  assert.deepEqual(b.grid, grid(0));
  const c = redo(b.undo, b.grid);
  assert.equal(c.what, 'paste');
  assert.deepEqual(c.grid, grid(1));
});

test('a new edit after an undo clears redo', () => {
  let h = record(emptyUndo(), { what: 'paste', before: grid(0) });
  h = undo(h, grid(1)).undo;
  h = record(h, { what: 'Delete', before: grid(0) });
  assert.equal(redo(h, grid(3)), null);
  assert.equal(undo(h, grid(3)).what, 'Delete');
});

test('the stack holds 100 steps; the 101st edit loses the oldest', () => {
  assert.equal(UNDO_MAX, 100);
  let h = emptyUndo();
  for (let i = 0; i < 101; i++) h = record(h, { what: `edit ${i}`, before: grid(i) });
  let cur = grid(101);
  const seen = [];
  for (let s = undo(h, cur); s; s = undo(s.undo, cur)) {
    seen.push(s.what);
    cur = s.grid;
  }
  assert.equal(seen.length, 100);
  assert.equal(seen[0], 'edit 100');
  assert.equal(seen.at(-1), 'edit 1');
  assert.deepEqual(cur, grid(1));
});

test('edits that share a join key make one step that undoes to before the first', () => {
  let h = record(emptyUndo(), { what: 'cell edit', before: grid(0), join: 'caption 1 1' });
  h = record(h, { what: 'cell edit', before: grid(1), join: 'caption 1 1' });
  h = record(h, { what: 'cell edit', before: grid(2), join: 'caption 1 1' });
  const u = undo(h, grid(3));
  assert.deepEqual(u.grid, grid(0));
  assert.equal(undo(u.undo, grid(0)), null);
});

test('a join key does not reach past another edit or an undo', () => {
  let h = record(emptyUndo(), { what: 'cell edit', before: grid(0), join: 'caption 1 1' });
  h = record(h, { what: 'paste', before: grid(1) });
  h = record(h, { what: 'cell edit', before: grid(2), join: 'caption 1 1' });
  assert.deepEqual(undo(h, grid(3)).grid, grid(2));
  let k = record(emptyUndo(), { what: 'cell edit', before: grid(0), join: 'x' });
  k = undo(k, grid(1)).undo;
  k = redo(k, grid(0)).undo;
  k = record(k, { what: 'cell edit', before: grid(1), join: 'x' });
  assert.deepEqual(undo(k, grid(2)).grid, grid(1));
});

test('sameCells looks at size and cells, not the selection, order or array identity', () => {
  const a = grid(1);
  assert.ok(sameCells(a, { ...a, sel: { anchor: { r: 1, c: 1 }, head: { r: 1, c: 1 } } }));
  assert.ok(sameCells(a, { ...a, cells: a.cells.map((c) => ({ ...c })) }));
  assert.ok(!sameCells(a, grid(2)));
  assert.ok(!sameCells(a, { ...a, rows: 5 }));
  const two = [
    { r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n: 1 },
    { r: 1, c: 2, rs: 1, cs: 1, kind: 'source', n: 2 },
  ];
  assert.ok(sameCells({ ...a, cells: two }, { ...a, cells: [two[1], two[0]] }));
});
