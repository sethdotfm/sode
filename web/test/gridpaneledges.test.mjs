// Which sources and destinations a Grid panel shows, in display order (gridpaneledges.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { isGridPanel, gridAxes } from '../gridpaneledges.js';
import { namesReply, gridPanel } from './gridpanelfixture.mjs';

test('only a panel of type grid is a Grid panel; no type is a Buttons panel', () => {
  assert.equal(isGridPanel(gridPanel()), true);
  assert.equal(isGridPanel({ type: 'buttons' }), false);
  assert.equal(isGridPanel({ name: 'old file' }), false);
});

test('an "all" edge shows every router item in router order', () => {
  const names = namesReply({
    sources: [
      [2, 'VT 1'],
      [0, 'CAM 1'],
      [1, 'CAM 2'],
    ],
    destinations: [
      [1, 'MON B'],
      [0, 'MON A'],
    ],
  });
  assert.deepEqual(gridAxes(gridPanel(), names), { srcs: [0, 1, 2], dsts: [0, 1] });
});

test('a missing edge reads as "all"', () => {
  const p = gridPanel();
  delete p.sources;
  assert.deepEqual(gridAxes(p, namesReply()).srcs, [0, 1, 2]);
});

test('an "all" edge follows the router: an item the router adds shows, one it drops goes', () => {
  const p = gridPanel();
  assert.deepEqual(
    gridAxes(
      p,
      namesReply({
        sources: [
          [0, 'CAM 1'],
          [5, 'NEW'],
        ],
      }),
    ).srcs,
    [0, 5],
  );
});

test('no names yet shows no items', () => {
  assert.deepEqual(gridAxes(gridPanel(), {}), { srcs: [], dsts: [] });
});

test('"all" is resolved against the viewer\'s Rights: items outside them are left out', () => {
  const p = gridPanel({ rights: { sources: [0, 2], dsts: [1] } });
  assert.deepEqual(gridAxes(p, namesReply()), { srcs: [0, 2], dsts: [1] });
});

test('an unset Rights list allows every item, an empty one none', () => {
  assert.deepEqual(gridAxes(gridPanel({ rights: { dsts: [] } }), namesReply()), { srcs: [0, 1, 2], dsts: [] });
});

test('an explicit list leaves out members the router no longer reports and members outside the Rights', () => {
  const p = gridPanel({ sources: { members: [2, 9, 0, 1] }, rights: { sources: [0, 2, 9] } });
  assert.deepEqual(gridAxes(p, namesReply()).srcs, [2, 0]);
});

const six = () =>
  namesReply({
    sources: [
      [0, 'CAM 1'],
      [1, 'CAM 2'],
      [2, 'CAM 3'],
      [3, 'VT 1'],
      [4, 'VT 2'],
      [5, 'GFX'],
    ],
  });
const srcsOf = (sources) => gridAxes(gridPanel({ sources }), six()).srcs;

test('an "all" edge leaves out its hidden items and still follows the router', () => {
  assert.deepEqual(srcsOf({ members: 'all', hide: [1, 4] }), [0, 2, 3, 5]);
  const added = namesReply({
    sources: [
      [0, 'CAM 1'],
      [1, 'CAM 2'],
      [7, 'NEW'],
    ],
  });
  assert.deepEqual(gridAxes(gridPanel({ sources: { members: 'all', hide: [1] } }), added).srcs, [0, 7]);
});

test('a list edge shows its members in list order', () => {
  assert.deepEqual(srcsOf({ members: [4, 0, 2] }), [4, 0, 2]);
  assert.deepEqual(srcsOf({ members: [] }), []);
});

test('pinned members come first, in their own order, and show only one time', () => {
  const items = { 3: { pinned: true }, 1: { pinned: true }, 5: { art: 'camera' } };
  assert.deepEqual(srcsOf({ members: 'all', items }), [1, 3, 0, 2, 4, 5]);
  assert.deepEqual(srcsOf({ members: [5, 3, 0, 1], items }), [3, 1, 5, 0]);
});

test('a pin on an "all" edge keeps it following the router', () => {
  const sources = { members: 'all', items: { 5: { pinned: true } } };
  const added = namesReply({
    sources: [
      [0, 'CAM 1'],
      [5, 'GFX'],
      [9, 'NEW'],
    ],
  });
  assert.deepEqual(gridAxes(gridPanel({ sources }), added).srcs, [5, 0, 9]);
});

test('a pin on a hidden item or on a non-member shows nothing', () => {
  const items = { 4: { pinned: true }, 2: { pinned: true } };
  assert.deepEqual(srcsOf({ members: 'all', hide: [4], items }), [2, 0, 1, 3, 5]);
  assert.deepEqual(srcsOf({ members: [0, 1], items }), [0, 1]);
});

test('destinations take the same display order as sources', () => {
  const p = gridPanel({ destinations: { members: [1, 0], items: { 0: { pinned: true } } } });
  assert.deepEqual(gridAxes(p, namesReply()).dsts, [0, 1]);
});
