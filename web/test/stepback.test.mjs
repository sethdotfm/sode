// Ctrl+Z and Ctrl+Shift+Z on Home: the step rules (stepback.js).
import test from 'node:test';
import assert from 'node:assert/strict';

const tab = new Map();
globalThis.sessionStorage = { getItem: (k) => tab.get(k) ?? null, setItem: (k, v) => tab.set(k, String(v)), removeItem: (k) => tab.delete(k) };
const { backStep, forwardStep, nextRedo, noteOwn, ownTakes, noteSent, redoSteps } = await import('../stepback.js');

const take = (id, o = {}) => ({ id, src: id, dst: id, levels: [1, 2], ok: true, ...o });
const undo = (id, of, o = {}) => take(id, { undo: of, ...o });
const ids = (step) => step && step.entries.map((x) => x.id);

test('each Ctrl+Z goes one take further back and passes over undos and undone takes', () => {
  // newest first, as S.hist
  let hist = [take(2), take(1)];
  const own = [1, 2];
  assert.deepEqual(ids(backStep(hist, own)), [2]);
  hist = [undo(3, 2), ...hist];
  assert.deepEqual(ids(backStep(hist, [...own, 3])), [1], 'the undo itself is no step, and 2 is undone');
  hist = [undo(4, 1), ...hist];
  assert.equal(backStep(hist, [...own, 3, 4]), null);
});

test('only this browser’s takes count', () => {
  const hist = [take(3), take(2, { ext: true }), take(1)];
  assert.deepEqual(ids(backStep(hist, [1])), [1], 'take 3 came from another browser or the API');
  assert.equal(backStep(hist, []), null);
});

test('a take whose undo is still on its way is passed over; a failed undo leaves it a step', () => {
  const hist = [take(2), take(1)];
  assert.deepEqual(ids(backStep(hist, [1, 2], [[2]])), [1]);
  const failed = [undo(3, 2, { ok: false, failed: [1, 2] }), ...hist];
  assert.deepEqual(ids(backStep(failed, [1, 2], [[2]])), [2]);
  const partly = [undo(3, 2, { ok: false, failed: [2] }), ...hist];
  assert.deepEqual(ids(backStep(partly, [1, 2], [[2]])), [2], 'Partly undone is still a step, as for the panel Undo cell');
});

test('a Protect set is no step, a Free Protect is one', () => {
  const hist = [take(3, { protect: 'set', src: -1 }), take(2, { protect: 'clear', src: -1 }), take(1)];
  assert.deepEqual(ids(backStep(hist, [1, 2, 3])), [2]);
  const back = [take(4, { protect: 'set', src: -1, undo: 2 }), ...hist];
  assert.deepEqual(ids(backStep(back, [1, 2, 3, 4])), [1], 'the Protect set again undid the Free Protect');
});

test('a Group is one step, without its entries that are already undone', () => {
  const g = { group: 'g1', groupN: 3 };
  const hist = [undo(5, 3), take(4, g), take(3, g), take(2, g), take(1)];
  assert.deepEqual(ids(backStep(hist, [1, 2, 3, 4, 5])), [4, 2]);
});

test('Ctrl+Shift+Z goes forward through what Ctrl+Z undid, newest first', () => {
  const hist = [undo(4, 1), undo(3, 2), take(2), take(1)];
  const redo = [[2], [1]];
  const first = forwardStep(hist, redo);
  assert.deepEqual(first.ids, [1]);
  assert.deepEqual(first.keep, redo);
  const after = nextRedo(first.keep, { verb: 'redo', step: 'forward' });
  assert.deepEqual(forwardStep(hist, after).ids, [2]);
  assert.equal(forwardStep(hist, nextRedo(after, { verb: 'redo', step: 'forward' })).entries, null);
});

test('Ctrl+Shift+Z drops a step whose undo failed on every level, or whose entries are not loaded', () => {
  const hist = [undo(4, 2, { ok: false, failed: [1, 2] }), undo(3, 1), take(2), take(1)];
  const got = forwardStep(hist, [[1], [99], [2]]);
  assert.deepEqual(got.ids, [1]);
  assert.deepEqual(got.keep, [[1]]);
});

test('a new take clears the redo list; an undo, a redo, a retry or a Protect does not', () => {
  const redo = [[2], [1]];
  assert.deepEqual(nextRedo(redo, { verb: 'patch' }), []);
  assert.deepEqual(nextRedo(redo, { verb: 'preset' }), []);
  assert.deepEqual(nextRedo(redo, { verb: 'grid' }), []);
  for (const verb of ['undo', 'redo', 'retry']) assert.deepEqual(nextRedo(redo, { verb }), redo, verb);
  assert.deepEqual(nextRedo(redo, { verb: 'protect', protect: {} }), redo);
  assert.deepEqual(nextRedo(redo, { verb: 'undo', step: 'back', stepIds: [5, 4] }), [[2], [1], [5, 4]]);
  assert.deepEqual(nextRedo(redo, { verb: 'unprotect', protect: {}, step: 'back', stepIds: [5] }), redo, 'a Protect has no Redo');
});

test('take 1 to 1, then 2 to 2: two Ctrl+Z, then two Ctrl+Shift+Z', () => {
  let hist = [take(2), take(1)];
  let own = [1, 2];
  const press = (step, plan, made) => {
    noteSentTo(plan);
    hist = [made, ...hist];
    own = [...own, made.id];
    return step;
  };
  let redo = [];
  const noteSentTo = (plan) => {
    redo = nextRedo(redo, plan);
  };
  const back = () => {
    const s = backStep(hist, own, redo);
    return press(s, { verb: 'undo', step: 'back', stepIds: ids(s) }, undo(hist[0].id + 1, s.e.id));
  };
  const forward = () => {
    const s = forwardStep(hist, redo);
    redo = s.keep;
    return press(s, { verb: 'redo', step: 'forward' }, take(hist[0].id + 1, { src: s.entries[0].src, dst: s.entries[0].dst }));
  };
  assert.deepEqual(ids(back()), [2]);
  assert.deepEqual(ids(back()), [1]);
  assert.equal(backStep(hist, own, redo), null);
  assert.deepEqual(forward().entries[0].dst, 1);
  assert.deepEqual(forward().entries[0].dst, 2);
  assert.equal(forwardStep(hist, redo).entries, null);
  // The redo of 2 to 2 is now the newest step back.
  assert.equal(backStep(hist, own, redo).e.dst, 2);
});

test('the lists live in sessionStorage, so a reload keeps them', () => {
  tab.clear();
  noteOwn([7, undefined, 8]);
  assert.deepEqual(ownTakes(), [7, 8]);
  assert.deepEqual(JSON.parse(tab.get('sode-own-takes')), [7, 8]);
  noteSent({ verb: 'undo', step: 'back', stepIds: [8] });
  assert.deepEqual(redoSteps(), [[8]]);
  noteSent({ verb: 'patch' });
  assert.deepEqual(redoSteps(), []);
  tab.clear();
  assert.deepEqual(ownTakes(), [], 'a new tab starts empty');
});
