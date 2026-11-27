import test from 'node:test';
import assert from 'node:assert/strict';
import { firstTicked, groupByOwner, parseShown, swapLabel, swapShow, swapValue } from '../swaprules.js';
import { diffRows, sectionOfKey } from '../savereviewrules.js';

const items = [
  { id: 'panel/p1', kind: 'panel', name: 'Desk', owner: 'Bo', broken: false },
  { id: 'preset/everyone/p1', kind: 'preset', name: 'Studio', owner: '', broken: true },
  { id: 'preset/u-1/a1', kind: 'preset', name: 'Wide', owner: 'Ana', broken: false },
  { id: 'filterSet/everyone/Cams', kind: 'filterSet', name: 'Cams', owner: '', broken: false },
];

test('groups by owner, Everyone first, then names in order', () => {
  const g = groupByOwner(items, false);
  assert.deepEqual(
    g.map((x) => [x.owner, x.items.map((i) => i.name)]),
    [
      ['Everyone', ['Studio', 'Cams']],
      ['Ana', ['Wide']],
      ['Bo', ['Desk']],
    ],
  );
});

test('Only Broken keeps the Broken items and drops empty owners', () => {
  assert.deepEqual(
    groupByOwner(items, true).map((x) => [x.owner, x.items.map((i) => i.name)]),
    [['Everyone', ['Studio']]],
  );
});

test('the value sends the ticked items that Only Broken shows', () => {
  const ticked = new Set(['panel/p1', 'preset/everyone/p1', 'preset/u-1/a1']);
  const v = swapValue({ router: 'r1', side: 'destination', from: 22, to: 39 }, items, ticked, true);
  assert.deepEqual(v, { router: 'r1', side: 'destination', from: 22, to: 39, items: ['preset/everyone/p1'] });
  assert.equal(swapValue({ router: 'r1', side: 'source', from: 1, to: 2 }, items, new Set(), false), null);
});

test('numbers are typed as shown and sent as the router holds them', () => {
  assert.equal(parseShown('23', 1), 22);
  assert.equal(parseShown(' 0 ', 0), 0);
  assert.equal(parseShown('0', 1), null);
  assert.equal(parseShown('x', 1), null);
  assert.equal(parseShown('', 1), null);
});

test('the review row names the Swap and counts items per owner', () => {
  const v = { router: 'r1', side: 'destination', from: 22, to: 39, items: ['preset/everyone/p1', 'preset/u-1/a1', 'panel/p1'] };
  assert.equal(swapLabel(v, 1), 'Swap destination 23 for 40');
  assert.equal(swapLabel(v, 0, 'B Stage 2'), 'Swap destination 22 for 39 on B Stage 2');
  assert.equal(swapShow(v, items), '3 items: Everyone 1, Ana 1, Bo 1');
  assert.equal(swapShow(undefined, items), 'No Swap');
  const rows = diffRows({}, { swap: v });
  assert.equal(rows.length, 1);
  assert.equal(sectionOfKey('swap'), 'routers');
});

test('a found list ticks every item but tie lines', () => {
  const items = [
    { id: 'preset/everyone/p1', kind: 'preset' },
    { id: 'tieLine/t3', kind: 'tieLine' },
    { id: 'panel/x', kind: 'panel' },
  ];
  assert.deepEqual(firstTicked(items), ['preset/everyone/p1', 'panel/x']);
});
