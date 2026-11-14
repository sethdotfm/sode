// The Undo cell's rules (panelmodel.js) and its status words (panelrules.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { createState, undoReady, startUndo, finishUndo, failTake, resultOf, errorOf, ERROR_MS } from '../panelmodel.js';
import { errorText, namesFrom, KINDS } from '../panelrules.js';
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
  assert.equal(errorText(errorOf(done, T0), names), 'Undone: MON 2 back to CAM 1');
  assert.equal(errorOf(done, T0 + ERROR_MS + 1), null);
});

test('an undo of several destinations names each', () => {
  const reply = {
    ok: true,
    results: [
      { src: 1, dst: 2, result: 'connected' },
      { src: 1, dst: 4, result: 'already' },
    ],
  };
  assert.equal(errorText(errorOf(finishUndo(createState(), reply, T0), T0), names), 'Undone: MON 2, MON 4');
});

test('a failed undo reads as a failed take does', () => {
  const reply = { ok: false, results: [{ src: 1, dst: 2, result: 'failed', error: 'no answer' }] };
  const done = finishUndo(createState(), reply, T0);
  assert.equal(resultOf(done, 2, T0), 'failed');
  assert.equal(errorText(errorOf(done, T0), names), 'Undo failed on MON 2');
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
