import test from 'node:test';
import assert from 'node:assert/strict';
import { newState, hasConfirm, tapDst, tapSrc, afterConfirm, clearAll, prune, confirmStatus } from '../gridstate.js';

test('destinations toggle, several at once; the state is not mutated', () => {
  const a = newState();
  const b = tapDst(tapDst(a, 3), 5);
  assert.deepEqual([...b.lit], [3, 5]);
  assert.deepEqual([...tapDst(b, 3).lit], [5]);
  assert.equal(a.lit.size, 0);
});

test('a source is selected one at a time and tapping it again unselects', () => {
  let st = tapSrc(newState(), 7);
  assert.equal(st.src, 7);
  st = tapSrc(st, 9);
  assert.equal(st.src, 9);
  assert.equal(tapSrc(st, 9).src, null);
});

test('Confirm needs a source and a destination and says what is missing', () => {
  let st = newState();
  assert.deepEqual(confirmStatus(st), { ready: false, text: 'Pick a source and a destination' });
  st = tapSrc(st, 1);
  assert.equal(confirmStatus(st).text, 'Light a destination');
  st = tapDst(st, 2);
  assert.equal(confirmStatus(st).ready, true);
  assert.equal(confirmStatus(tapDst(newState(), 2)).text, 'Pick a source');
});

test('after Confirm the source goes and the destinations stay lit', () => {
  const st = afterConfirm(tapDst(tapSrc(newState(), 1), 2));
  assert.equal(st.src, null);
  assert.deepEqual([...st.lit], [2]);
  assert.equal(clearAll(st).lit.size, 0);
});

test('prune drops a source or destination the router lost', () => {
  const st = prune(tapDst(tapDst(tapSrc(newState(), 1), 2), 3), (n) => n !== 1, (n) => n !== 3);
  assert.equal(st.src, null);
  assert.deepEqual([...st.lit], [2]);
});

test('hasConfirm', () => {
  assert.equal(hasConfirm({ cells: [{ kind: 'src' }, { kind: 'confirm' }] }), true);
  assert.equal(hasConfirm({ cells: [{ kind: 'src' }] }), false);
  assert.equal(hasConfirm(null), false);
});
