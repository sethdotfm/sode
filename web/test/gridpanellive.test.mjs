// What a live update (a newer saved copy) keeps on an open Grid panel: arrange (gridpanelarrange.js) and restage (gridpanelstage.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { arrange } from '../gridpanelarrange.js';
import { createStage, pressCrosspoint, restage, routesOf } from '../gridpanelstage.js';
import { createFind, typed, switchMode } from '../gridpanelfind.js';
import { createPicks } from '../gridpanelpick.js';
import { CORNER } from '../gridpanelcursor.js';
import { namesReply, gridPanel } from './gridpanelfixture.mjs';

const nm = namesReply({
  sources: [
    [0, 'CAM 1'],
    [1, 'CAM 2'],
    [2, 'VT 1'],
    [3, 'CAM 3'],
  ],
  destinations: [
    [0, 'MON A'],
    [1, 'MON B'],
    [2, 'MON C'],
  ],
});
const start = { find: createFind(), picks: createPicks(), cur: CORNER, axes: { srcs: [], dsts: [] } };
const open = (panel, was = start) => ({ ...was, ...arrange(panel, nm, was) });

test('a first arrange shows every member and finds nothing', () => {
  const v = open(gridPanel());
  assert.deepEqual(v.axes, { srcs: [0, 1, 2, 3], dsts: [0, 1, 2] });
  assert.deepEqual(v.full, v.axes);
  assert.deepEqual(v.matches, { srcs: [], dsts: [] });
});

test('an update keeps the Find text, its mode and the current match', () => {
  let v = open(gridPanel());
  const f = typed(v.find, 'srcs', 'cam');
  v = { ...v, find: { ...f, srcs: { ...f.srcs, at: 3 } } };
  v = open(gridPanel({ sources: { members: [1, 2, 3] } }), v);
  assert.equal(v.find.srcs.text, 'cam');
  assert.equal(v.find.srcs.at, 3);
  assert.deepEqual(v.matches.srcs, [1, 3]);
  assert.deepEqual(v.axes.srcs, [1, 2, 3], 'Find mode shows all members');
});

test('an update keeps a Filter, which shows only matches from the new members', () => {
  let v = open(gridPanel());
  v = { ...v, find: switchMode(typed(v.find, 'srcs', 'cam')) };
  v = open(gridPanel({ sources: { members: [1, 2, 3] } }), v);
  assert.equal(v.find.mode, 'filter');
  assert.deepEqual(v.axes.srcs, [1, 3]);
  assert.deepEqual(v.full.srcs, [1, 2, 3]);
});

test('picks on names that left the panel go; picks still on it stay, also when a Filter hides them', () => {
  let v = open(gridPanel());
  v = { ...v, picks: { ...v.picks, srcs: [0, 2], dsts: [1, 2] }, find: switchMode(typed(v.find, 'srcs', 'cam')) };
  v = open(gridPanel({ sources: { members: [1, 2, 3] }, destinations: { members: [0, 1] } }), v);
  assert.deepEqual(v.picks.srcs, [2], 'VT 1 is hidden by the Filter but stays picked');
  assert.deepEqual(v.picks.dsts, [1]);
});

test('the cursor stays on the same source and destination when items before them leave', () => {
  let v = open(gridPanel());
  v = { ...v, cur: { i: 2, j: 3 } }; // MON C, CAM 3
  v = open(gridPanel({ sources: { members: [2, 3] }, destinations: { members: [2] } }), v);
  assert.deepEqual(v.cur, { i: 0, j: 1 });
});

test('the cursor on a source that left keeps its place, clamped to the new edge', () => {
  let v = open(gridPanel());
  v = { ...v, cur: { i: 0, j: 3 } };
  v = open(gridPanel({ sources: { members: [0, 1] } }), v);
  assert.deepEqual(v.cur, { i: 0, j: 1 });
});

test('an update drops the staged routes and an Undo stage, as a Buttons panel does', () => {
  let st = pressCrosspoint(createStage({ mode: 'take' }), { src: 1, dst: 0 }, [1]).st;
  st = { ...st, undoId: 'u1' };
  const next = restage(st, gridPanel());
  assert.deepEqual(routesOf(next), []);
  assert.equal(next.undoId, null);
  assert.equal(next.mode, 'take');
});

test('an update takes the mode from the new settings', () => {
  assert.equal(restage(createStage({ mode: 'take' }), gridPanel({ settings: { mode: 'instant' } })).mode, 'instant');
  assert.equal(restage(createStage({ mode: 'instant' }), gridPanel()).mode, 'take');
});

test('an update never cuts a take in flight short', () => {
  const st = { ...createStage(), busy: true };
  assert.equal(restage(st, gridPanel()).busy, true);
});

test('an update carries the take flow: takeOnly turns the confirm box off, and back', () => {
  assert.equal(restage(createStage({ mode: 'take' }), gridPanel({ settings: { mode: 'takeOnly' } })).ask, false);
  assert.equal(restage(createStage({ mode: 'takeOnly' }), gridPanel()).ask, true);
});
