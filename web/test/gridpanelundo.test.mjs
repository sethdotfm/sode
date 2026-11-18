// Undo on a Grid panel (gridpanelundo.js): u stages this instance's Undo in place of the staged routes.
import test from 'node:test';
import assert from 'node:assert/strict';
import { stageUndo, leaveUndo, afterUndo, undoText } from '../gridpanelundo.js';
import { createStage, pressCrosspoint, routesOf, dropStaged, cancelConfirm } from '../gridpanelstage.js';
import { envFrom } from '../panelrules.js';
import { tallyReply } from './gridpanelfixture.mjs';

const env = (prot = []) => envFrom(tallyReply(), { protected: prot.map((dst) => ({ dst, level: 1 })) });
// GET /panels/{id}/undo's `undo`.
const next = (over = {}) => ({
  id: 41,
  allowed: true,
  reason: '',
  routes: [
    { src: 0, dst: 1, levels: [1, 2] },
    { src: 2, dst: 0, levels: [1] },
  ],
  ...over,
});

test('u stages the whole undo as the server will send it, in place of the staged routes', () => {
  const st = pressCrosspoint(createStage(), { src: 1, dst: 1 }, [3]).st;
  const u = stageUndo(st, next(), env());
  assert.equal(u.undoId, 41);
  assert.equal(u.error, null);
  assert.deepEqual(routesOf(u), [
    { src: 0, dst: 1, levels: [1, 2] },
    { src: 2, dst: 0, levels: [1] },
  ]);
});

test('with nothing to undo, a refusal or a protected destination, the stage stays as it was', () => {
  const st = pressCrosspoint(createStage(), { src: 1, dst: 1 }, [3]).st;
  for (const [n, e, code] of [
    [null, env(), 'nothing-to-undo'],
    [next({ allowed: false, reason: "a route is outside this panel's Rights" }), env(), 'undo-refused'],
    [next(), env([0]), 'protected'],
  ]) {
    const u = stageUndo(st, n, e);
    assert.equal(u.error.code, code);
    assert.deepEqual(u.staged, st.staged);
    assert.equal(u.undoId, null);
  }
});

test('a crosspoint press drops a staged undo whole before it stages', () => {
  const u = stageUndo(createStage(), next(), env());
  const st = leaveUndo(u);
  assert.equal(st.undoId, null);
  assert.deepEqual(st.staged, []);
  const plain = pressCrosspoint(createStage(), { src: 1, dst: 1 }, [3]).st;
  assert.equal(leaveUndo(plain), plain);
});

test('Esc and a closed Instant mode confirm drop the undo', () => {
  assert.equal(dropStaged(stageUndo(createStage(), next(), env())).undoId, null);
  const instant = stageUndo(createStage({ mode: 'instant' }), next(), env());
  assert.equal(cancelConfirm(instant).undoId, null);
  // A Take mode confirm closed keeps it staged, as any staged route.
  assert.equal(cancelConfirm(stageUndo(createStage(), next(), env())).undoId, 41);
});

test('after the undo is sent or refused, nothing is staged and the id is gone', () => {
  const sent = { ...stageUndo(createStage(), next(), env()), busy: true };
  const ok = afterUndo(sent, null);
  assert.deepEqual([ok.staged, ok.undoId, ok.busy, ok.error], [[], null, false, null]);
  // It is possible that the id is stale, so u must ask again.
  const bad = afterUndo(sent, 'What Undo would undo has changed; press u again');
  assert.deepEqual([bad.staged, bad.undoId, bad.busy, bad.error.code], [[], null, false, 'take']);
});

test('the status for an undo that cannot stage', () => {
  assert.equal(undoText({ code: 'nothing-to-undo' }), 'Nothing to undo');
  assert.equal(undoText({ code: 'undo-refused', message: 'outside Rights' }), 'Undo is refused: outside Rights');
  assert.equal(undoText({ code: 'cap' }), '');
});
