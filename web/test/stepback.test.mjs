// Ctrl+Z and Ctrl+Shift+Z on Home: the step rules (stepback.js).
import test from 'node:test';
import assert from 'node:assert/strict';

const tab = new Map();
globalThis.sessionStorage = { getItem: (k) => tab.get(k) ?? null, setItem: (k, v) => tab.set(k, String(v)), removeItem: (k) => tab.delete(k) };
const { backStep, forwardStep, nextRedo, noteOwn, ownTakes, noteSent, redoSteps, forgetSteps, entryKey } = await import('../stepback.js');

const take = (id, o = {}) => ({ id, time: `T${id}`, src: id, dst: id, levels: [1, 2], ok: true, ...o });
const undo = (id, of, o = {}) => take(id, { undo: of, ...o });
const ids = (step) => step && step.entries.map((x) => x.id);
// Keys of this tab's entries, as noteOwn writes them.
const k = (...n) => n.map((id) => entryKey(take(id)));

test('each Ctrl+Z goes one take further back and passes over undos and undone takes', () => {
  // newest first, as S.hist
  let hist = [take(2), take(1)];
  const own = k(1, 2);
  assert.deepEqual(ids(backStep(hist, own)), [2]);
  hist = [undo(3, 2), ...hist];
  assert.deepEqual(ids(backStep(hist, [...own, ...k(3)])), [1], 'the undo itself is no step, and 2 is undone');
  hist = [undo(4, 1), ...hist];
  assert.equal(backStep(hist, [...own, ...k(3, 4)]), null);
});

test('only this browser’s takes count', () => {
  const hist = [take(3), take(2, { ext: true }), take(1)];
  assert.deepEqual(ids(backStep(hist, k(1))), [1], 'take 3 came from another browser or the API');
  assert.equal(backStep(hist, []), null);
});

test('an id that History gave again after a Clear or a restart is not this tab’s take', () => {
  // Tab A sent #1 at T1. History restarted, and #1 is now the take of tab B, at another time.
  const hist = [take(1, { time: 'T9' })];
  assert.equal(backStep(hist, k(1)), null);
  assert.equal(forwardStep(hist, [k(1)]).entries, null);
});

test('a History Clear forgets both lists', () => {
  tab.clear();
  noteOwn([1], [take(1)]);
  noteSent({ verb: 'undo', step: 'back', stepIds: k(1) });
  forgetSteps();
  assert.deepEqual(ownTakes(), []);
  assert.deepEqual(redoSteps(), []);
});

test('a take whose undo is still on its way is passed over; a failed undo leaves it a step', () => {
  const hist = [take(2), take(1)];
  assert.deepEqual(ids(backStep(hist, k(1, 2), [k(2)])), [1]);
  const failed = [undo(3, 2, { ok: false, failed: [1, 2] }), ...hist];
  assert.deepEqual(ids(backStep(failed, k(1, 2), [k(2)])), [2]);
  const partly = [undo(3, 2, { ok: false, failed: [2] }), ...hist];
  assert.deepEqual(ids(backStep(partly, k(1, 2), [k(2)])), [2], 'Partly undone is still a step, as for the panel Undo cell');
});

test('a Retry of a failed undo is no step', () => {
  const hist = [take(4, { retry: 3 }), undo(3, 2, { ok: false, failed: [1, 2] }), take(2), take(1)];
  assert.deepEqual(ids(backStep(hist, k(1, 2, 3, 4))), [1], 'the Retry put 2 back, so the next step is 1');
});

test('an undo of a partly failed take that put back the levels it confirmed ends that step', () => {
  // Level 2 failed and the router answered for it, so the undo left it out (untouchedLevels in plans.js).
  const hist = [undo(3, 2, { levels: [1] }), take(2, { ok: false, failed: [2], now: [[2, 4]] }), take(1)];
  assert.deepEqual(ids(backStep(hist, k(1, 2, 3))), [1]);
});

test('a take with nothing to undo is passed over, as the panel Undo cell does', () => {
  const hist = [take(2), take(1)];
  const empty = (entries) => entries.some((x) => x.id === 2);
  assert.deepEqual(ids(backStep(hist, k(1, 2), [], empty)), [1]);
  assert.equal(
    backStep(hist, k(1, 2), [], () => true),
    null,
  );
});

test('a Protect set is no step, a Free Protect is one', () => {
  const hist = [take(3, { protect: 'set', src: -1 }), take(2, { protect: 'clear', src: -1 }), take(1)];
  assert.deepEqual(ids(backStep(hist, k(1, 2, 3))), [2]);
  const back = [take(4, { protect: 'set', src: -1, undo: 2 }), ...hist];
  assert.deepEqual(ids(backStep(back, k(1, 2, 3, 4))), [1], 'the Protect set again undid the Free Protect');
});

test('a Group is one step, without its entries that are already undone', () => {
  const g = { group: 'g1', groupN: 3 };
  const hist = [undo(5, 3), take(4, g), take(3, g), take(2, g), take(1)];
  assert.deepEqual(ids(backStep(hist, k(1, 2, 3, 4, 5))), [4, 2]);
});

test('Ctrl+Shift+Z goes forward through what Ctrl+Z undid, newest first', () => {
  const hist = [undo(4, 1), undo(3, 2), take(2), take(1)];
  const redo = [k(2), k(1)];
  const first = forwardStep(hist, redo);
  assert.deepEqual(first.ids, k(1));
  assert.deepEqual(first.keep, redo);
  const after = nextRedo(first.keep, { verb: 'redo', step: 'forward' });
  assert.deepEqual(forwardStep(hist, after).ids, k(2));
  assert.equal(forwardStep(hist, nextRedo(after, { verb: 'redo', step: 'forward' })).entries, null);
});

test('Ctrl+Shift+Z drops a step whose undo failed on every level, or whose entries are not loaded', () => {
  const hist = [undo(4, 2, { ok: false, failed: [1, 2] }), undo(3, 1), take(2), take(1)];
  const got = forwardStep(hist, [k(1), k(99), k(2)]);
  assert.deepEqual(got.ids, k(1));
  assert.deepEqual(got.keep, [k(1)]);
});

test('a new take clears the redo list; an undo, a redo, a retry or a Protect does not', () => {
  const redo = [k(2), k(1)];
  assert.deepEqual(nextRedo(redo, { verb: 'patch' }), []);
  assert.deepEqual(nextRedo(redo, { verb: 'preset' }), []);
  assert.deepEqual(nextRedo(redo, { verb: 'grid' }), []);
  for (const verb of ['undo', 'redo', 'retry']) assert.deepEqual(nextRedo(redo, { verb }), redo, verb);
  assert.deepEqual(nextRedo(redo, { verb: 'protect', protect: {} }), redo);
  assert.deepEqual(nextRedo(redo, { verb: 'undo', step: 'back', stepIds: k(5, 4) }), [k(2), k(1), k(5, 4)]);
  assert.deepEqual(nextRedo(redo, { verb: 'unprotect', protect: {}, step: 'back', stepIds: k(5) }), redo, 'a Protect has no Redo');
});

test('take 1 to 1, then 2 to 2: two Ctrl+Z, then two Ctrl+Shift+Z', () => {
  let hist = [take(2), take(1)];
  let own = k(1, 2);
  const press = (step, plan, made) => {
    noteSentTo(plan);
    hist = [made, ...hist];
    own = [...own, entryKey(made)];
    return step;
  };
  let redo = [];
  const noteSentTo = (plan) => {
    redo = nextRedo(redo, plan);
  };
  const back = () => {
    const s = backStep(hist, own, redo);
    return press(s, { verb: 'undo', step: 'back', stepIds: s.entries.map(entryKey) }, undo(hist[0].id + 1, s.e.id));
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
  noteOwn([7, undefined, 8, 9], [take(8), take(7)]);
  assert.deepEqual(ownTakes(), k(7, 8), 'an entry that is not loaded is not kept');
  assert.deepEqual(JSON.parse(tab.get('sode-own-takes')), k(7, 8));
  noteSent({ verb: 'undo', step: 'back', stepIds: k(8) });
  assert.deepEqual(redoSteps(), [k(8)]);
  noteSent({ verb: 'patch' });
  assert.deepEqual(redoSteps(), []);
  tab.clear();
  assert.deepEqual(ownTakes(), [], 'a new tab starts empty');
});
