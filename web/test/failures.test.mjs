// Failed takes: what the router reported, undo and changedSince.
// Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';

// core.js reads these at load.
const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

test('failed takes: reasons, undo and changedSince', async () => {
  const { S, undoPlan, failureGroups, changedSince } = await import('../core.js');
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}], [2, {}], [3, {}], [4, {}]]);
  S.srcBy = new Map([[0, { n: 0, num: 1, name: 'CAM 1', label: 'CAM 1' }], [8, { n: 8, num: 9, name: 'CAM 9', label: 'CAM 9' }]]);
  // Take of src 7 on levels 1-4, from src 0. Level 1 was left on 0, level 2
  // is on 8 (someone else's), level 3 gave no answer, level 4 worked.
  const e = {
    id: 4, dst: 5, src: 7, srcName: 'X', dstName: 'Y', time: new Date().toISOString(), ok: false, levels: [1, 2, 3, 4],
    prev: [[1, 0], [2, 0], [3, 0], [4, 0]], failed: [1, 2, 3], now: [[1, 0], [2, 8]], err: 'no confirmation from router: ...',
  };
  assert.deepEqual(failureGroups(e).map((g) => [g.levels, g.text]), [[[1], 'still on CAM 1'], [[2], 'on CAM 9'], [[3], 'no answer']]);
  assert.equal(failureGroups({ ...e, ok: true }).length, 0);
  assert.equal(failureGroups({ ...e, failed: undefined }).length, 0); // an older entry
  // Undo restores the levels that may have changed: 3 (unknown) and 4.
  const item = undoPlan(e).items[0];
  assert.deepEqual(item.groups.map((g) => g.levels), [[3, 4]]);
  assert.deepEqual(item.untouched, [1, 2]);
  // Level 4 moved on since; level 3 can't tell (it never confirmed).
  S.tally = new Map([[5, new Map([[3, 0], [4, 9]])]]);
  assert.deepEqual(changedSince(item).map((r) => r.levels), [[4]]);
  // An old failed entry without the data checks nothing, as before.
  assert.deepEqual(changedSince(undoPlan({ ...e, failed: undefined, now: undefined }).items[0]), []);
});
