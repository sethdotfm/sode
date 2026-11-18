// A Grid panel on the phone layout (gridpanelphone.js): the press point of the numbered targets with no hover.
// Also the Take button, which every layout shows.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createPicks, pickName } from '../gridpanelpick.js';
import { restPoint, takeButton, tapMods, pickNote } from '../gridpanelphone.js';

const axes = { srcs: [30, 10, 20, 40, 50], dsts: [7, 3, 5, 9, 1, 2] };
const picked = (srcs, dsts) => ({ ...createPicks(), srcs, dsts });

test('with no picks there is no rest point', () => {
  assert.equal(restPoint(createPicks(), axes, { i: 2, j: 1 }), null);
});

test('the rest point is the first picked source and destination in display order', () => {
  assert.deepEqual(restPoint(picked([40, 10], [9, 3]), axes, { i: -1, j: -1 }), { src: 10, dst: 3 });
});

test('with only sources picked the targets start at the cursor row, else the first destination', () => {
  assert.deepEqual(restPoint(picked([20, 40], []), axes, { i: 3, j: -1 }), { src: 20, dst: 9 });
  assert.deepEqual(restPoint(picked([20, 40], []), axes, { i: -1, j: 2 }), { src: 20, dst: 7 });
});

test('with only destinations picked the source is the cursor column, else the first source', () => {
  assert.deepEqual(restPoint(picked([], [5, 1]), axes, { i: -1, j: 3 }), { src: 40, dst: 5 });
  assert.deepEqual(restPoint(picked([], [5, 1]), axes, { i: 0, j: -1 }), { src: 30, dst: 5 });
});

test('picks hidden by a Filter are not a rest point', () => {
  assert.deepEqual(restPoint(picked([99, 50], []), axes, { i: -1, j: -1 }), { src: 50, dst: 7 });
  assert.equal(restPoint(picked([99], []), axes, { i: -1, j: -1 }), null);
});

test('the Take button shows in Take mode on every layout and works only with staged routes', () => {
  const at = (o) => takeButton({ mode: 'take', staged: 2, busy: false, ...o });
  assert.deepEqual(at({}), { shown: true, disabled: false, armed: true });
  assert.deepEqual(at({ staged: 0 }), { shown: true, disabled: true, armed: false });
  assert.deepEqual(at({ busy: true }), { shown: true, disabled: true, armed: false });
  assert.deepEqual(at({ mode: 'instant' }), { shown: false, disabled: true, armed: false });
});

test('a tap on the phone layout adds or removes one name, so a finger can pick a set', () => {
  const tap = (p, n) => pickName(p, 'srcs', n, tapMods({}, true), axes.srcs);
  let p = tap(createPicks(), 10);
  p = tap(p, 40);
  assert.deepEqual(p.srcs, [10, 40]);
  p = tap(p, 10);
  assert.deepEqual(p.srcs, [40]);
});

test('a click on the desktop layout keeps its modifiers', () => {
  assert.deepEqual(tapMods({ shiftKey: true, metaKey: false, ctrlKey: false }, false), { range: true, toggle: false });
  assert.deepEqual(tapMods({ shiftKey: false, metaKey: false, ctrlKey: true }, false), { range: false, toggle: true });
  assert.deepEqual(tapMods({ shiftKey: true }, true), { range: true, toggle: true });
});

test('the chip beside Clear counts picks first, then staged routes, and has no help words', () => {
  const at = (o) => pickNote({ srcs: 0, dsts: 0, staged: 0, ...o });
  assert.equal(at({ dsts: 1 }), '1 destination picked');
  assert.equal(at({ dsts: 2, srcs: 1, staged: 3 }), '2 destinations, 1 source picked');
  assert.equal(at({ srcs: 3 }), '3 sources picked');
  assert.equal(at({ staged: 2 }), '2 staged');
  assert.equal(at({}), '');
});
