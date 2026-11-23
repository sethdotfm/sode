import test from 'node:test';
import assert from 'node:assert/strict';
import { edgeIn, edgeOut, edgesIn, edgesOut } from '../gridedgerefs.js';
import { gridAxes } from '../gridpaneledges.js';
import { namesReply, gridPanel } from './gridpanelfixture.mjs';
import { makePlaces, SPAN } from '../routerids.js';

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

// On one router an item number is the router number.
const read = numbers;

test('a stored panel reads as the number form, on item numbers', () => {
  assert.deepEqual(edgesIn(stored), read);
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
  assert.deepEqual(edgesIn(edgesOut(edgesIn(stored))), read);
  assert.deepEqual(edgeIn(undefined), undefined);
});

const B = 'b0b0b0b0b0b0b0b0';
const places = makePlaces([{ uid: R }, { uid: B, prefix: 'B' }]);

test('a reference on another router reads as an item number past the first router, and goes back on its router', () => {
  const e = edgeIn(
    {
      members: [
        { router: R, n: 1, name: 'MON B' },
        { router: B, n: 4, name: 'B MON' },
      ],
      items: [{ router: B, n: 4, pinned: true }],
    },
    places,
  );
  assert.deepEqual(e.members, [1, SPAN + 4]);
  assert.deepEqual(e.items, { [SPAN + 4]: { pinned: true } });
  assert.deepEqual(edgeOut({ ...e, members: [...e.members, 7] }, places), {
    members: [
      { router: R, n: 1 },
      { router: B, n: 4 },
      { router: R, n: 7 },
    ],
    items: [{ router: B, n: 4, pinned: true }],
  });
});

test('one number on two routers is two items', () => {
  const e = edgeIn(
    {
      members: [
        { router: R, n: 5, name: 'CAM 5' },
        { router: B, n: 5, name: 'B CAM' },
      ],
      items: [{ router: B, n: 5, art: 'icon:mic' }],
    },
    places,
  );
  assert.deepEqual(e.members, [5, SPAN + 5]);
  assert.deepEqual(edgeOut(e, places), {
    members: [
      { router: R, n: 5 },
      { router: B, n: 5 },
    ],
    items: [{ router: B, n: 5, art: 'icon:mic' }],
  });
  // An item number read again is the same item number, as the Live Editor hands the run the editor's edges.
  assert.deepEqual(edgeIn(e, places), e);
});

test('a router that GET /routers does not list gets its own place', () => {
  const e = edgeIn(
    {
      members: [
        { router: 'gone', n: 2 },
        { router: R, n: 2 },
      ],
    },
    places,
  );
  assert.deepEqual(e.members, [2 * SPAN + 2, 2]);
  assert.deepEqual(edgeOut(e, places).members, [
    { router: 'gone', n: 2 },
    { router: R, n: 2 },
  ]);
});

test('a Buttons panel is not changed', () => {
  const p = { name: 'B', pages: [] };
  assert.equal(edgesIn(p), p);
  assert.equal(edgesOut(p), p);
});
