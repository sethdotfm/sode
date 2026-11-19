import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createState,
  undoReady,
  pressUndo,
  undoState,
  startTake,
  finishUndo,
  finishTake,
  startInstant,
  failTake,
  armed,
  applyTally,
  clear,
  pressSource,
  pressPreset,
  pressDestination,
  destinationState,
  resultOf,
  errorOf,
  undoneOf,
  ERROR_MS,
} from '../panelmodel.js';
import { errorText, undoneText, undoMoved, envFrom, namesFrom, KINDS } from '../panelrules.js';
import { CATEGORIES, CATEGORY_LABEL, CATEGORY_ICON } from '../findrules.js';
import { kindLabel } from '../editorrules.js';

const T0 = 1000;
const names = namesFrom({
  sources: [{ n: 1, name: 'CAM 1' }],
  destinations: [
    { n: 2, name: 'MON 2' },
    { n: 4, name: 'MON 4' },
  ],
});
const next = (allowed = true) => ({ entries: [], routes: [{ src: 1, dst: 2, levels: [1] }], allowed, reason: allowed ? '' : 'MON 2 is outside your rights' });

const env = (protectedDsts = []) =>
  envFrom({ levels: [{ level: 1 }, { level: 2 }], routes: [{ dst: 2, level: 1, src: 9 }] }, { protected: protectedDsts.map((dst) => ({ dst })) });
const two = {
  id: 7,
  entries: [],
  allowed: true,
  reason: '',
  routes: [
    { src: 1, dst: 2, levels: [1] },
    { src: 3, dst: 4, levels: [1, 2] },
  ],
};
const staged = (st) => st.staged.map((x) => [x.dst, x.src, x.levels]);

test('the cell is ready only with a take to undo inside the Rights and nothing in flight', () => {
  const st = createState();
  assert.equal(undoReady(st, next()), true);
  assert.equal(undoReady(st, null), false, 'nothing left to undo');
  assert.equal(undoReady(st, undefined), false, 'not asked yet, or the key lacks the take scope');
  assert.equal(undoReady(st, next(false)), false, 'outside the Rights');
  assert.equal(undoReady({ ...st, busy: true }, next()), false);
});

test('an Undo press stages the routes GET answered, whole, in place of the plan, and views the first', () => {
  const st = { ...createState({ multi: false, levels: [2] }), staged: [{ dst: 9, src: 5, levels: [1] }] };
  const s = pressUndo(st, two, env(), T0);
  assert.deepEqual(staged(s), [
    [2, 1, [1]],
    [4, 3, [1, 2]],
  ]);
  assert.equal(s.undoId, 7);
  assert.deepEqual(s.view, { dst: 2, levels: null });
  assert.equal(destinationState(s, 4), 'staged');
  assert.equal(undoState(s), 'pending');
  assert.equal(undoState(createState()), 'idle');
  assert.equal(armed(s, env()), true);
  const t = startTake(s, env());
  assert.equal(t.undoId, 7);
  assert.equal(t.state.busy, true);
});

test('with Fixed destination the view stays the fixed one and no undo route is dropped', () => {
  const st = createState({ fixedDests: [4] });
  const s = pressUndo(st, two, env(), T0);
  assert.equal(s.view.dst, 4);
  assert.equal(s.staged.length, 2);
});

test('nothing to undo, or an undo outside the Rights, says so and leaves the plan', () => {
  const st = { ...createState(), staged: [{ dst: 9, src: 5, levels: [1] }] };
  const none = pressUndo(st, null, env(), T0);
  assert.deepEqual(none.staged, st.staged);
  assert.equal(none.undoId, null);
  assert.equal(errorText(errorOf(none, T0), names), 'Nothing to undo');
  const out = pressUndo(st, next(false), env(), T0);
  assert.deepEqual(out.staged, st.staged);
  assert.equal(errorText(errorOf(out, T0), names), 'MON 2 is outside your rights');
  const prot = pressUndo(st, two, env([4]), T0);
  assert.deepEqual(prot.staged, st.staged);
  assert.equal(errorText(errorOf(prot, T0), names), 'MON 4 is protected');
});

test('a second Undo press stages the same take again rather than one further back', () => {
  const s = pressUndo(pressUndo(createState(), two, env(), T0), two, env(), T0);
  assert.equal(s.undoId, 7);
  assert.equal(s.staged.length, 2);
});

test('a Source or Preset press, an unstage or Clear drops the staged undo whole', () => {
  const e = { ...env(), presets: new Map([['everyone:P', [{ src: 1, dsts: [6], levels: [] }]]]) };
  const s = pressUndo(createState(), two, e, T0);
  const src = pressSource(s, { kind: 'source', n: 5 }, e, T0);
  assert.equal(src.undoId, null);
  assert.deepEqual(staged(src), [[2, 5, [1, 2]]]);
  const pre = pressPreset(s, { kind: 'preset', scope: 'everyone', name: 'P' }, e, T0);
  assert.equal(pre.undoId, null);
  assert.deepEqual(staged(pre), [[6, 1, [1, 2]]]);
  const viewed = pressDestination(s, { kind: 'destination', n: 4 });
  assert.equal(viewed.undoId, 7, 'viewing a destination changes no route');
  const off = pressDestination(viewed, { kind: 'destination', n: 4 });
  assert.equal(off.undoId, null);
  assert.deepEqual(off.staged, []);
  const cleared = clear(s);
  assert.equal(cleared.undoId, null);
  assert.deepEqual(cleared.staged, []);
});

test('a refused Source press leaves the staged undo', () => {
  const s = pressUndo(createState(), two, env([2]), T0);
  assert.equal(s.error.code, 'protected');
  const ok = pressUndo(createState(), two, env(), T0);
  assert.equal(pressSource(ok, { kind: 'source', n: 5 }, env([2]), T0).undoId, 7);
});

test('the tally leaves a staged undo whole, since the server sends its own plan for the take', () => {
  const s = pressUndo(createState(), two, env(), T0);
  const moved = envFrom({ levels: [{ level: 1 }, { level: 2 }], routes: [{ dst: 2, level: 1, src: 1 }] }, {});
  assert.equal(applyTally(s, moved), s);
});

test('a finished undo shows results like a take, drops the undo plan and names what was undone', () => {
  const s = pressUndo(createState(), two, env(), T0);
  const reply = {
    ok: false,
    results: [
      { src: 1, dst: 2, result: 'connected' },
      { src: 3, dst: 4, result: 'failed', error: 'no result' },
    ],
  };
  const done = finishUndo(startTake(s, env()).state, reply, T0);
  assert.equal(done.busy, false);
  assert.equal(resultOf(done, 2, T0), 'connected');
  assert.equal(resultOf(done, 4, T0), 'failed');
  assert.deepEqual(done.staged, [], 'a failed route is not kept: pressing Undo again stages what is left');
  assert.equal(done.undoId, null);
  assert.equal(errorOf(done, T0), null);
  assert.equal(undoneText(undoneOf(done, T0), names), 'Undo failed on MON 4');
  const ok = finishUndo(
    startTake(pressUndo(createState(), next(), env(), T0), env()).state,
    { ok: true, results: [{ src: 1, dst: 2, result: 'connected' }] },
    T0,
  );
  assert.equal(undoneText(undoneOf(ok, T0), names), 'Undone: MON 2 back to CAM 1');
  assert.equal(undoneOf(ok, T0 + ERROR_MS + 1), null);
});

test('a refused or unanswered undo drops the undo plan; a refused take keeps its plan', () => {
  const s = startTake(pressUndo(createState(), two, env(), T0), env()).state;
  const conflict = failTake(s, 'conflict', 'What Undo would undo has changed; press it again', T0);
  assert.deepEqual(conflict.staged, []);
  assert.equal(conflict.undoId, null);
  assert.equal(errorText(errorOf(conflict, T0), names), 'What Undo would undo has changed; press it again');
  const plain = { ...createState(), staged: [{ dst: 2, src: 1, levels: [1] }] };
  assert.equal(failTake(plain, 'network', 'x', T0).staged.length, 1);
});

test('in Instant mode the Undo press is its own Confirm', () => {
  const s = pressUndo(createState({ mode: 'instant' }), two, env(), T0);
  assert.equal(armed(s, env()), false);
  const go = startInstant(s, env());
  assert.equal(go.undoId, 7);
  assert.equal(go.routes.length, 2);
  const done = finishUndo(go.state, { ok: true, results: [{ src: 1, dst: 2, result: 'connected' }] }, T0);
  assert.ok(startInstant(pressSource({ ...done, view: { dst: 2, levels: null } }, { kind: 'source', n: 5 }, env(), T0), env()));
});

// A Late Result and a router refusal (a Protected destination) both come back as failed.
test('an undo marks each destination as a take does', () => {
  const reply = {
    ok: false,
    results: [
      { src: 1, dst: 2, result: 'failed', error: 'no result' },
      { src: 1, dst: 4, result: 'failed', error: 'destination is protected on levels 1' },
    ],
  };
  const st = createState();
  assert.deepEqual(finishUndo(st, reply, T0).results, finishTake(st, reply, T0).results);
  assert.equal(resultOf(finishUndo(st, reply, T0), 4, T0), 'failed');
});

test('an undo of several destinations names each', () => {
  const reply = {
    ok: true,
    results: [
      { src: 1, dst: 2, result: 'connected' },
      { src: 1, dst: 4, result: 'already' },
    ],
  };
  assert.equal(undoneText(undoneOf(finishUndo(createState(), reply, T0), T0), names), 'Undone: MON 2, MON 4');
});

test('a refusal from the server reads as its message', () => {
  const refused = failTake(createState(), 'nothing_to_do', 'this panel instance has no take left to undo', T0);
  assert.equal(errorText(errorOf(refused, T0), names), 'this panel instance has no take left to undo');
});

test('Undo is a cell kind with a label and an icon everywhere a kind is listed', () => {
  assert.ok(KINDS.includes('undo'));
  assert.ok(CATEGORIES.includes('undo'));
  assert.equal(CATEGORY_LABEL.undo, 'Undo');
  assert.equal(CATEGORY_ICON.undo, 'undo-2');
  assert.equal(kindLabel('undo'), 'Undo');
  assert.equal(names.of({ kind: 'undo' }), 'Undo');
});

test('the target is asked again only when a destination it would restore moves', () => {
  const tally = (routes) => envFrom({ routes }, {});
  const a = tally([
    { dst: 2, level: 1, src: 3 },
    { dst: 2, level: 2, src: 3 },
    { dst: 9, level: 1, src: 1 },
  ]);
  const reordered = tally([
    { dst: 9, level: 1, src: 5 },
    { dst: 2, level: 2, src: 3 },
    { dst: 2, level: 1, src: 3 },
  ]);
  const moved = tally([
    { dst: 2, level: 1, src: 1 },
    { dst: 2, level: 2, src: 3 },
  ]);
  assert.equal(undoMoved(a, reordered, next()), false, 'another destination moved, and the order is no move');
  assert.equal(undoMoved(a, moved, next()), true);
  assert.equal(undoMoved(a, moved, null), false);
  assert.equal(undoMoved(a, moved, undefined), false);
});
