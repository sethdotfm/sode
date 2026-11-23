import test from 'node:test';
import assert from 'node:assert/strict';
import { tallyReplyOf, protectedReplyOf, namesReplyOf, aimOf, stagedRows, routesWord, withBoxRoute, takeCount, storedItems } from '../gridpoprules.js';
import { envFrom, namesFrom } from '../panelrules.js';
import { gridAxes } from '../gridpaneledges.js';
import { viaOn } from '../viarules.js';

const levels = [
  { ui: 1, wire: 0, label: 'V', swatch: 'video' },
  { ui: 2, wire: 1, label: 'A1', swatch: '01', color: '#123456' },
];
const tally = new Map([
  [
    0,
    new Map([
      [1, 4],
      [2, 5],
    ]),
  ],
  [3, new Map([[1, 7]])],
]);

test('the tally answer has the GET /tally shape, so the Grid body reads it as a panel page does', () => {
  const t = tallyReplyOf({ levels, levelNumbering: '1', defaultLevels: '1-2', tally });
  assert.deepEqual(t.levels, [
    { level: 1, label: 'V', swatch: 'video' },
    { level: 2, label: 'A1', swatch: '01', color: '#123456' },
  ]);
  assert.equal(t.levelNumbering, '1');
  assert.equal(t.defaultLevels, '1-2');
  const env = envFrom(t, protectedReplyOf(new Map([[3, new Map([[1, { dev: 9, kind: 1 }]])]])));
  assert.equal(env.tally.get(0).get(2), 5);
  assert.equal(env.tally.get(3).get(1), 7);
  assert.deepEqual([...env.protectedDsts], [3]);
});

test('the names answer keeps the router numbers, names and number offset', () => {
  const nm = namesReplyOf({
    srcs: [{ n: 0, name: 'CAM 1', num: '1', label: '1 CAM 1' }],
    dsts: [
      { n: 0, name: 'MON A' },
      { n: 1, name: 'MON B' },
    ],
    numberOffset: 1,
  });
  assert.deepEqual(nm, {
    numberOffset: 1,
    sources: [{ n: 0, name: 'CAM 1' }],
    destinations: [
      { n: 0, name: 'MON A' },
      { n: 1, name: 'MON B' },
    ],
  });
  assert.equal(namesFrom(nm).destination(1), 'MON B');
});

test('the popup shows only the items in the Rights, in router order', () => {
  const nm = namesReplyOf({
    srcs: [0, 1, 2].map((n) => ({ n, name: `S${n}` })),
    dsts: [0, 1].map((n) => ({ n, name: `D${n}` })),
    numberOffset: 1,
  });
  const panel = { type: 'grid', sources: { members: 'all' }, destinations: { members: 'all' }, rights: { sources: [2, 0], dsts: [{ n: 1 }] } };
  assert.deepEqual(gridAxes(panel, nm), { srcs: [0, 2], dsts: [1] });
});

test('a valid pair in the boxes aims at it on every open, with the main page levels', () => {
  assert.deepEqual(aimOf({ src: 3, dst: 5, first: true }), { src: 3, dst: 5, levels: true });
  assert.deepEqual(aimOf({ src: 3, dst: 5, first: false }), { src: 3, dst: 5, levels: true });
});

test('with no valid pair a reopen comes back as left, and one valid box is scrolled into view', () => {
  assert.equal(aimOf({ src: null, dst: null, first: false }), null);
  assert.deepEqual(aimOf({ src: 3, dst: null, first: false }), { src: 3, dst: null, levels: false });
  assert.deepEqual(aimOf({ src: null, dst: 5, first: false }), { src: null, dst: 5, levels: false });
});

test('the first open with no valid pair starts from the one box or the top left, with the main page levels', () => {
  assert.deepEqual(aimOf({ src: null, dst: null, first: true }), { src: null, dst: null, levels: true });
  assert.deepEqual(aimOf({ src: 3, dst: null, first: true }), { src: 3, dst: null, levels: true });
});

test('the staged list has one row for each source and level set, in the order first staged', () => {
  const rows = stagedRows([
    { src: 1, dst: 10, levels: [1, 2] },
    { src: 2, dst: 11, levels: [1, 2] },
    { src: 1, dst: 12, levels: [1, 2] },
    { src: 1, dst: 13, levels: [1] },
  ]);
  assert.deepEqual(rows, [
    { src: 1, levels: [1, 2], dsts: [10, 12] },
    { src: 2, levels: [1, 2], dsts: [11] },
    { src: 1, levels: [1], dsts: [13] },
  ]);
});

test('the staged list names its routes', () => {
  assert.equal(routesWord(1), '1 route');
  assert.equal(routesWord(12), '12 routes');
});

test('One Take adds the boxes route to the staged routes, and the boxes win on a destination and level in both', () => {
  const staged = [
    { src: 1, dst: 5, levels: [1, 2] },
    { src: 2, dst: 6, levels: [1, 2] },
    { src: 3, dst: 7, levels: [1] },
  ];
  assert.deepEqual(withBoxRoute(staged, { src: 9, dsts: [6, 8], levels: [1] }), [
    { src: 1, dst: 5, levels: [1, 2] },
    { src: 2, dst: 6, levels: [2] },
    { src: 3, dst: 7, levels: [1] },
    { src: 9, dst: 6, levels: [1] },
    { src: 9, dst: 8, levels: [1] },
  ]);
  // A staged route that the boxes cover on every level goes; the same route again stays one route.
  assert.deepEqual(withBoxRoute(staged, { src: 3, dsts: [7], levels: [1] }), [
    { src: 1, dst: 5, levels: [1, 2] },
    { src: 2, dst: 6, levels: [1, 2] },
    { src: 3, dst: 7, levels: [1] },
  ]);
  assert.deepEqual(withBoxRoute(staged, { src: 1, dsts: [5], levels: [2, 3] }), [
    { src: 1, dst: 5, levels: [1, 2, 3] },
    { src: 2, dst: 6, levels: [1, 2] },
    { src: 3, dst: 7, levels: [1] },
  ]);
  assert.deepEqual(staged[0].levels, [1, 2], 'the staged routes stay as they are');
});

test('the main Take counts the staged routes and the boxes route as one take sends them', () => {
  const staged = [
    { src: 1, dst: 5, levels: [1, 2] },
    { src: 2, dst: 6, levels: [1] },
  ];
  assert.equal(takeCount([], { src: 9, dsts: [6, 8], levels: [1] }), 0);
  assert.equal(takeCount(staged, null), 2);
  assert.equal(takeCount(staged, { src: 9, dsts: [7, 8], levels: [1] }), 4);
  // The boxes take all the levels of a staged destination: that staged route goes.
  assert.equal(takeCount(staged, { src: 9, dsts: [6], levels: [1] }), 2);
  // The same route as a staged one stays one route.
  assert.equal(takeCount(staged, { src: 2, dsts: [6], levels: [2] }), 2);
});

test('a crosspoint add stores no levels while the Levels box is empty, as the boxes Add does; a name keeps its levels', () => {
  const xp = { kind: 'xp', items: [{ src: 3, dsts: [5], levels: [1, 2] }] };
  assert.deepEqual(storedItems(xp, []), [{ src: 3, dsts: [5], levels: [] }]);
  assert.deepEqual(storedItems(xp, [2]), xp.items);
  const name = { kind: 'dsts', items: [{ src: 3, dsts: [5], levels: [1, 2] }] };
  assert.deepEqual(storedItems(name, []), name.items);
});

test('a route that a tie line feeds carries its Via row, while the row is for the same source', () => {
  const row = { dst: 0, level: 1, src: 4, via: [{ id: 't1', name: 'TIE 3' }], origin: { router: 'a', n: 1 } };
  const stale = { dst: 3, level: 1, src: 6, via: [{ id: 't1', name: 'TIE 3' }], origin: { router: 'a', n: 1 } };
  const via = new Map([
    [0, new Map([[1, row]])],
    [3, new Map([[1, stale]])],
  ]);
  const t = tallyReplyOf({ levels, levelNumbering: '1', defaultLevels: '1-2', tally, via });
  const env = envFrom(t, protectedReplyOf(new Map()));
  assert.equal(env.via.get(0).get(1), row);
  assert.equal(env.via.has(3), false);
  assert.equal(viaOn(env, 4, 0), row);
  assert.equal(viaOn(env, 5, 0), null);
  assert.equal(viaOn(env, 7, 3), null);
});
