// The grid model with the Preset, Clear and Current Destination cell kinds.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createGrid, select, setKind, unassigned, cellAt } from '../gridmodel.js';

const grid = (cells) => createGrid({ rows: 4, cols: 8, cells });

test('a Preset cell is unassigned until it names a preset, and Clear and Current need nothing', () => {
  const g = grid([
    { r: 1, c: 1, kind: 'preset' },
    { r: 1, c: 2, kind: 'preset', scope: 'me', name: 'Show open' },
    { r: 1, c: 3, kind: 'clear' },
    { r: 1, c: 4, kind: 'current' },
  ]);
  assert.equal(g.cells.length, 4);
  assert.deepEqual(
    unassigned(g).map((c) => c.c),
    [1],
  );
});

test('Type from Preset to another kind drops the preset it named and keeps the caption', () => {
  const g = setKind(select(grid([{ r: 1, c: 1, kind: 'preset', scope: 'me', name: 'Show open', caption: 'Open' }]), 1, 1), 'source');
  const c = cellAt(g, 1, 1);
  assert.deepEqual([c.scope, c.name, c.caption], [undefined, undefined, 'Open']);
});
