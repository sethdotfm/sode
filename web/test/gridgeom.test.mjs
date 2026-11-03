import test from 'node:test';
import assert from 'node:assert/strict';
import { rectBetween, expandRect, join, split, assign, remove, overlaps, resize, savable, squares, lostByJoin, lostByResize, cellAt, confirmFor, slugify, validLink, freeLink } from '../gridgeom.js';

const c = (x, y, w = 1, h = 1, kind = 'src', ref = '1', label = '') => ({ x, y, w, h, kind, ref, label });

test('rectBetween takes any corner order', () => {
  assert.deepEqual(rectBetween({ x: 3, y: 2 }, { x: 1, y: 4 }), { x: 1, y: 2, w: 3, h: 3 });
  assert.deepEqual(rectBetween({ x: 0, y: 0 }, { x: 0, y: 0 }), { x: 0, y: 0, w: 1, h: 1 });
});

test('expandRect takes a clipped joined cell whole', () => {
  const cells = [c(1, 1, 2, 2)];
  assert.deepEqual(expandRect(cells, { x: 0, y: 0, w: 2, h: 2 }), { x: 0, y: 0, w: 3, h: 3 });
  assert.deepEqual(expandRect(cells, { x: 3, y: 3, w: 1, h: 1 }), { x: 3, y: 3, w: 1, h: 1 });
});

test('join merges cells into one, keeping the first assigned', () => {
  const cells = [c(0, 0, 1, 1, 'dst', '5', 'Mon'), c(1, 0, 1, 1, 'src', '9'), c(5, 5)];
  const r = join(cells, { x: 0, y: 0, w: 2, h: 1 });
  assert.equal(r.cells.length, 2);
  assert.deepEqual(cellAt(r.cells, 1, 0), { x: 0, y: 0, w: 2, h: 1, kind: 'dst', ref: '5', label: 'Mon' });
  assert.deepEqual(overlaps(r.cells), []);
});

test('join of empty squares makes an unassigned span, dropped on save', () => {
  const r = join([], { x: 0, y: 0, w: 2, h: 2 });
  assert.deepEqual(r.cells, [{ x: 0, y: 0, w: 2, h: 2, kind: '', ref: '', label: '' }]);
  assert.deepEqual(savable(r.cells), []);
});

test('join of one square changes nothing', () => {
  const cells = [c(0, 0)];
  assert.equal(join(cells, { x: 0, y: 0, w: 1, h: 1 }).cells, cells);
});

test('join that clips a span takes all of it', () => {
  const cells = [c(1, 0, 2, 1, 'src', '3')];
  const r = join(cells, { x: 0, y: 0, w: 2, h: 1 });
  assert.deepEqual(r.cells, [{ x: 0, y: 0, w: 3, h: 1, kind: 'src', ref: '3', label: '' }]);
});

test('split returns single squares, the first keeping the assignment', () => {
  const out = split([c(0, 0, 2, 2, 'dst', '4'), c(3, 3, 2, 1)], { x: 1, y: 1, w: 1, h: 1 });
  assert.deepEqual(out[0], c(0, 0, 1, 1, 'dst', '4'));
  assert.equal(out[1].w, 2); // untouched
  assert.equal(squares(out, 5, 5).length, 25 - 1);
});

test('assign edits a cell or makes one', () => {
  let cells = assign([], 2, 1, { kind: 'dst', ref: '7' });
  assert.deepEqual(cells, [{ x: 2, y: 1, w: 1, h: 1, kind: 'dst', ref: '7', label: '' }]);
  cells = assign(join(cells, { x: 2, y: 1, w: 2, h: 1 }).cells, 3, 1, { label: 'X' });
  assert.equal(cells.length, 1);
  assert.equal(cells[0].label, 'X');
});

test('remove clears every cell touching the rect', () => {
  assert.equal(remove([c(0, 0, 2, 1), c(4, 4)], { x: 1, y: 0, w: 1, h: 1 }).length, 1);
});

test('overlaps finds cells sharing a square', () => {
  assert.deepEqual(overlaps([c(0, 0, 2, 2), c(1, 1), c(2, 2)]), [[0, 1]]);
});

test('resize drops and shortens cells at the new edge', () => {
  const out = resize([c(0, 0, 3, 1), c(4, 0), c(0, 3)], 2, 3);
  assert.deepEqual(out, [c(0, 0, 2, 1)]);
});

test('squares covers every square once', () => {
  const sq = squares([c(0, 0, 2, 1)], 3, 2);
  assert.equal(sq.length, 1 + 4);
  assert.equal(sq.reduce((n, s) => n + s.w * s.h, 0), 6);
});

test('confirmFor: panel overrides the global default', () => {
  assert.equal(confirmFor({ confirm: 'default' }, true), true);
  assert.equal(confirmFor({ confirm: 'default' }, false), false);
  assert.equal(confirmFor({ confirm: 'on' }, false), true);
  assert.equal(confirmFor({ confirm: 'off' }, true), false);
});

test('links', () => {
  assert.equal(slugify('Studio A / Monitors!'), 'studio-a-monitors');
  assert.ok(validLink('a-1'));
  for (const bad of ['', 'A', 'a--b', '-a', 'a-', 'a b', 'ws', 'a'.repeat(41)]) assert.ok(!validLink(bad), bad);
  assert.equal(freeLink('Studio A', ['studio-a']), 'studio-a-2');
  assert.equal(freeLink('!!!', []), 'grid');
  assert.equal(freeLink('ws', []), 'ws-2');
});

test('lost assignments are counted before join and resize', () => {
  const cells = [c(0, 0, 1, 1, 'dst', '5'), c(1, 0, 1, 1, 'src', '9'), c(2, 0, 1, 1, 'src', '8'), c(5, 5)];
  assert.equal(lostByJoin(cells, { x: 0, y: 0, w: 3, h: 1 }), 2);
  assert.equal(lostByJoin(cells, { x: 0, y: 0, w: 1, h: 1 }), 0);
  assert.equal(lostByJoin([], { x: 0, y: 0, w: 2, h: 2 }), 0);
  assert.equal(lostByResize(cells, 2, 2), 2); // (2,0) and (5,5)
  assert.equal(lostByResize([c(1, 0, 3, 1)], 2, 2), 0); // shortened, kept
});
