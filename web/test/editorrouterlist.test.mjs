// The router list on the Grid panel editor's list tabs (editorgridrules.js): Find, selection and Add. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { routerRows, findRows, selectRows, addItems } from '../editorgridrules.js';
import { namesReply } from './gridpanelfixture.mjs';

const router = namesReply({
  sources: [
    [3, 'VT 2'],
    [0, 'CAM 1'],
    [1, 'CAM 2'],
    [2, 'VT 1'],
  ],
}).sources;

const ns = (rows) => rows.map((r) => r.n);

test('Add from the router lists what the panel does not have, in router order, limited by Rights', () => {
  assert.deepEqual(ns(routerRows(router, { members: [2, 9] }, { sources: [0, 2, 3] }, 'sources')), [0, 3]);
  assert.deepEqual(routerRows(router, { members: [] }, null, 'sources')[2], { n: 2, name: 'VT 1' });
});

test('on an "all" edge only the hidden items are left to add', () => {
  assert.deepEqual(ns(routerRows(router, { members: 'all', hide: [1] }, null, 'sources')), [1]);
});

test('Find matches names, and numbers as shown', () => {
  const rows = routerRows(router, { members: [] }, null, 'sources');
  assert.deepEqual(ns(findRows(rows, 'cam', 1)), [0, 1]);
  assert.deepEqual(ns(findRows(rows, '', 1)), [0, 1, 2, 3]);
  // Shown numbers are router numbers plus the offset, so 2-3 is router 1 and 2.
  assert.deepEqual(ns(findRows(rows, '2-3', 1)), [1, 2]);
  assert.deepEqual(ns(findRows(rows, '2-3', 0)), [2, 3]);
  assert.deepEqual(ns(findRows(rows, '4', 1)), [3]);
  assert.deepEqual(ns(findRows(rows, '/[', 1)), []);
});

test('a range of 2500 uses the numbers as shown', () => {
  const big = Array.from({ length: 2500 }, (_, n) => ({ n, name: `SRC ${n + 1}` }));
  const rows = findRows(routerRows(big, { members: [] }, null, 'sources'), '101-140', 1);
  assert.equal(rows.length, 40);
  assert.equal(rows[0].n, 100);
  assert.equal(rows.at(-1).n, 139);
});

test('a click selects one, Cmd toggles, and the selection holds router numbers', () => {
  const shown = routerRows(router, { members: [] }, null, 'sources');
  let p = selectRows({ selected: new Set([3]), anchor: null }, shown, 1, {});
  assert.deepEqual([...p.selected], [1]);
  assert.equal(p.anchor, 1);
  p = selectRows(p, shown, 3, { toggle: true });
  assert.deepEqual([...p.selected].sort(), [1, 3]);
  p = selectRows(p, shown, 1, { toggle: true });
  assert.deepEqual([...p.selected], [3]);
});

test('Shift-click adds only the rows Find shows between the anchor and the row', () => {
  const rows = routerRows(router, { members: [] }, null, 'sources');
  let p = selectRows({ selected: new Set(), anchor: null }, rows, 0, {});
  const shown = findRows(rows, '1,3,4', 1); // router 0, 2 and 3
  p = selectRows(p, shown, 3, { shift: true });
  assert.deepEqual([...p.selected].sort(), [0, 2, 3]);
  // A Shift-click keeps the selection, so items selected under another Find stay.
  p = selectRows({ selected: new Set([1]), anchor: 2 }, shown, 3, { shift: true });
  assert.deepEqual([...p.selected].sort(), [1, 2, 3]);
});

test('Shift-click with no anchor in view selects just the row', () => {
  const shown = findRows(routerRows(router, { members: [] }, null, 'sources'), 'vt', 1);
  const p = selectRows({ selected: new Set([0]), anchor: 1 }, shown, 3, { shift: true });
  assert.deepEqual([...p.selected].sort(), [0, 3]);
  assert.equal(p.anchor, 3);
});

test('Add appends the selection in router order and skips items the panel has', () => {
  const e = addItems({ members: [2, 9], items: { 9: { pinned: true } } }, router, new Set([3, 2, 0]));
  assert.deepEqual(e.members, [2, 9, 0, 3]);
  assert.deepEqual(e.items, { 9: { pinned: true } });
  const same = { members: [0] };
  assert.equal(addItems(same, router, new Set([0])), same);
});

test('Add on an "all" edge first turns it into a list', () => {
  const e = addItems({ members: 'all', hide: [1, 2], items: { 0: { art: 'x' } } }, router, new Set([2, 1, 3]));
  assert.deepEqual(e.members, [0, 3, 1, 2]);
  assert.equal(e.hide, undefined);
  assert.deepEqual(e.items, { 0: { art: 'x' } });
  const all = { members: 'all' };
  assert.equal(addItems(all, router, new Set([1])), all);
});
