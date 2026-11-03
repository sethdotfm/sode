// Redo: plans built from takes that worked. Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const setup = async () => {
  const m = await import('./all.mjs');
  const { S } = m;
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}], [2, {}], [3, {}]]);
  S.srcBy = new Map([[7, { n: 7, num: 8, name: 'CAM 8', label: 'CAM 8' }]]);
  S.dstBy = new Map([[5, { n: 5, num: 6, name: 'MON 6', label: 'MON 6' }]]);
  S.tally = new Map();
  return m;
};
const take = (o = {}) => ({ id: 1, src: 7, srcName: 'CAM 8', dst: 5, dstName: 'MON 6', levels: [1, 2, 3], prev: [[1, 0], [2, 0], [3, 0]], ok: true, ...o });

test('canRedo: only confirmed Sode takes', async () => {
  const { canRedo } = await setup();
  assert.equal(canRedo(take()), true);
  assert.equal(canRedo(take({ ok: false })), false);
  assert.equal(canRedo(take({ ext: true })), false);
  assert.equal(canRedo(take({ ext: true, found: true })), false);
  assert.equal(canRedo(take({ ext: true, follow: 3 })), false);
  assert.equal(canRedo(take({ undo: 2 })), false);
  assert.equal(canRedo(null), false);
});

test('redoPlan resends src to dst and links nothing', async () => {
  const { redoPlan, folderRedoPlan, planVerb, planSummary } = await setup();
  const p = redoPlan(take());
  assert.equal(p.verb, 'redo');
  assert.equal(planVerb(p), 'redo');
  const it = p.items[0];
  assert.equal(it.dst.n, 5);
  assert.deepEqual(it.groups.map((g) => [g.src.n, g.levels]), [[7, [1, 2, 3]]]);
  assert.equal(it.retry, null);
  assert.equal(it.undo, null);
  assert.equal(redoPlan(take({ ext: true })).items.length, 0);
  // A folder redoes only its redoable takes.
  const f = { entries: [take({ id: 1 }), take({ id: 2, dst: 6, ext: true }), take({ id: 3, dst: 9 })] };
  assert.deepEqual(folderRedoPlan(f).items.map((i) => i.redo.id), [1, 3]);
  assert.ok(planSummary(p).length > 0);
});

test('redoNoop: destination already has the source everywhere', async () => {
  const { S, redoPlan, redoNoop } = await setup();
  const item = () => redoPlan(take()).items[0];
  assert.equal(redoNoop(item()), false); // no tally known
  S.tally = new Map([[5, new Map([[1, 7], [2, 7], [3, 7]])]]);
  assert.equal(redoNoop(item()), true);
  S.tally = new Map([[5, new Map([[1, 7], [2, 7], [3, 4]])]]);
  assert.equal(redoNoop(item()), false);
  assert.equal(redoNoop({ ...item(), redo: null }), false);
});

test('canRedo: needs a level that still exists', async () => {
  const { canRedo, redoPlan, folderRedoPlan } = await setup();
  assert.equal(canRedo(take({ levels: [1, 9] })), true); // some levels gone: the rest go
  assert.deepEqual(redoPlan(take({ levels: [1, 9] })).items[0].groups[0].levels, [1]);
  assert.equal(canRedo(take({ levels: [9, 10] })), false); // all gone: nothing to send
  assert.equal(redoPlan(take({ levels: [9] })).items.length, 0);
  assert.equal(folderRedoPlan({ entries: [take({ levels: [9] })] }).items.length, 0);
});
