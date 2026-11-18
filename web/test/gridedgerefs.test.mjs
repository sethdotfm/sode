// Grid panel edges as router references, read into number form and sent back (gridedgerefs.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { edgeIn, edgeOut, edgesIn, edgesOut } from '../gridedgerefs.js';
import { gridAxes } from '../gridpaneledges.js';
import { namesReply, gridPanel } from './gridpanelfixture.mjs';

const R = 'a1b2c3d4e5f60718';

// As GET /panels/{id} reads it.
const stored = gridPanel({
  sources: {
    members: 'all',
    hide: [{ router: R, n: 1, name: 'CAM 2' }],
    items: [{ router: R, n: 2, name: 'VT 1', pinned: true, art: 'icon:mic' }],
  },
  destinations: {
    members: [
      { router: R, n: 1, name: 'MON B' },
      { router: R, n: 0, name: 'MON A' },
    ],
  },
});

const numbers = gridPanel({
  sources: { members: 'all', hide: [1], items: { 2: { pinned: true, art: 'icon:mic' } } },
  destinations: { members: [1, 0] },
});

test('a stored panel reads as the number form', () => {
  assert.deepEqual(edgesIn(stored), numbers);
});

test('the panel page and the Live Editor show the same axes from the stored panel', () => {
  const page = gridAxes(edgesIn(stored), namesReply());
  assert.deepEqual(page, { srcs: [2, 0], dsts: [1, 0] });
  // The Live Editor hands the run the editor's edges, which are already numbers.
  assert.deepEqual(gridAxes(edgesIn(edgesIn(stored)), namesReply()), page);
});

test('a save sends references with only n, items as a list in router order', () => {
  assert.deepEqual(edgeOut({ members: [5, 3], items: { 7: { pinned: true }, 3: { art: 'icon:mic' } } }), {
    members: [{ n: 5 }, { n: 3 }],
    items: [
      { n: 3, art: 'icon:mic' },
      { n: 7, pinned: true },
    ],
  });
  assert.deepEqual(edgeOut({ members: 'all', hide: [4] }), { members: 'all', hide: [{ n: 4 }] });
});

test('an item with no setting left is not sent', () => {
  assert.deepEqual(edgeOut({ members: 'all', items: { 4: { pinned: undefined } } }), { members: 'all' });
});

test('read, save and read again is the same number form', () => {
  assert.deepEqual(edgesIn(edgesOut(numbers)), numbers);
  assert.deepEqual(edgeIn(undefined), undefined);
});

test('a Buttons panel is not changed', () => {
  const p = { name: 'B', pages: [] };
  assert.equal(edgesIn(p), p);
  assert.equal(edgesOut(p), p);
});
