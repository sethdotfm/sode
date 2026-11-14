// The Undo cell's rules (panelmodel.js) and its status words (panelrules.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { createState, undoReady, startUndo, finishUndo, finishTake, startInstant, failTake, resultOf, errorOf, undoneOf, ERROR_MS } from '../panelmodel.js';
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

test('the cell is ready only with a take to undo inside the Rights and nothing in flight', () => {
  const st = createState();
  assert.equal(undoReady(st, next()), true);
  assert.equal(undoReady(st, null), false, 'nothing left to undo');
  assert.equal(undoReady(st, undefined), false, 'not asked yet, or the key lacks the take scope');
  assert.equal(undoReady(st, next(false)), false, 'outside the Rights');
  assert.equal(undoReady({ ...st, busy: true }, next()), false);
  assert.equal(startUndo(st, null), null);
  assert.equal(startUndo(st, next()).state.busy, true);
});

test('a finished undo shows results like a take, keeps the staged plan and names what was undone', () => {
  const st = { ...createState(), staged: [{ dst: 2, src: 9, levels: [1] }] };
  const done = finishUndo(startUndo(st, next()).state, { ok: true, results: [{ src: 1, dst: 2, result: 'connected' }] }, T0);
  assert.equal(done.busy, false);
  assert.equal(resultOf(done, 2, T0), 'connected');
  assert.equal(done.staged.length, 1);
  assert.equal(errorOf(done, T0), null);
  assert.equal(undoneText(undoneOf(done, T0), names), 'Undone: MON 2 back to CAM 1');
  assert.equal(undoneOf(done, T0 + ERROR_MS + 1), null);
});

test('a done undo does not hold back the next instant-mode press', () => {
  const st = { ...createState({ mode: 'instant' }), staged: [{ dst: 4, src: 1, levels: [1] }] };
  const done = finishUndo(startUndo(st, next()).state, { ok: true, results: [{ src: 1, dst: 2, result: 'connected' }] }, T0);
  const env = { protectedDsts: new Set() };
  assert.ok(startInstant(done, env));
  assert.equal(startInstant(done, env).state.undone, null);
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

test('a failed undo reads as a failed take does', () => {
  const reply = { ok: false, results: [{ src: 1, dst: 2, result: 'failed', error: 'no answer' }] };
  const done = finishUndo(createState(), reply, T0);
  assert.equal(resultOf(done, 2, T0), 'failed');
  assert.equal(errorOf(done, T0), null);
  assert.equal(undoneText(undoneOf(done, T0), names), 'Undo failed on MON 2');
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
