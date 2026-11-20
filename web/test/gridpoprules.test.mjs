import test from 'node:test';
import assert from 'node:assert/strict';
import { tallyReplyOf, protectedReplyOf, namesReplyOf, aimOf, stagedRows, routesWord } from '../gridpoprules.js';
import { envFrom, namesFrom } from '../panelrules.js';
import { gridAxes } from '../gridpaneledges.js';

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

test('the list Take names its routes', () => {
  assert.equal(routesWord(1), '1 route');
  assert.equal(routesWord(12), '12 routes');
});
