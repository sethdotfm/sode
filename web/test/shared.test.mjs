// Logic both layouts share: toggle, keepSearch and the "Last:" chips.
// Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

test('toggle adds, then removes', async () => {
  const { toggle } = await import('./all.mjs');
  const s = new Set();
  toggle(s, 'a');
  assert.deepEqual([...s], ['a']);
  toggle(s, 'b');
  toggle(s, 'a');
  assert.deepEqual([...s], ['b']);
});

test('keepSearch holds only a non-blank search that is still in the box', async () => {
  const { keepSearch } = await import('./all.mjs');
  let text = 'cam';
  const k = keepSearch(() => text);
  assert.equal(k.is(), false);
  k.set(true, 'cam');
  assert.equal(k.is(), true);
  text = 'cam 2';
  assert.equal(k.is(), false);
  k.set(false, 'cam 2');
  assert.equal(k.is(), false);
  k.set(true, '  ');
  text = '  ';
  assert.equal(k.is(), false);
});

test('lastChip: hides what the box already has', async () => {
  const { S, lastChip } = await import('./all.mjs');
  S.cfg = { numberOffset: 1, levelNumbering: '1', defaultLevels: '1,2', filterSets: [] };
  S.levels = [{ ui: 1 }, { ui: 2 }, { ui: 3 }];
  const it = (n) => ({ n, num: n + 1, name: `X${n}` });
  const t = { src: it(1), dsts: [it(5), it(6)], levels: [1, 3] };
  const empty = { src: null, dsts: [], levels: [] };
  const c = lastChip(t, empty);
  assert.ok(c.src && c.dst);
  assert.equal(c.dst[1], ' +1');
  assert.equal(lastChip(t, { ...empty, src: it(1) }).src, false);
  assert.equal(lastChip(t, { ...empty, dsts: [it(6), it(5)] }).dst, false);
  assert.notEqual(lastChip(t, { ...empty, dsts: [it(5)] }).dst, false);
  assert.equal(lastChip(t, { ...empty, levels: [1, 3] }).lvl, false);
  assert.equal(lastChip(null, empty).src, false);
  assert.equal(lastChip({ ...t, levels: [] }, empty).lvl, false);
});

test('nextTake: last take one number higher, per side', async () => {
  const { S, nextTake } = await import('./all.mjs');
  const it = (n) => ({ n, num: n + 1, name: `X${n}` });
  S.srcBy = new Map([1, 2].map((n) => [n, it(n)]));
  S.dstBy = new Map([5, 6, 7].map((n) => [n, it(n)]));
  S.hist = [{ id: 1, src: 1, srcName: 'X1', dst: 5, dstName: 'X5', levels: [1], ok: true }];
  let t = nextTake();
  assert.equal(t.src.n, 2);
  assert.deepEqual(t.dsts.map((d) => d.n), [6]);
  S.hist = [{ id: 2, src: 2, srcName: 'X2', dst: 7, dstName: 'X7', levels: [1], ok: true }];
  t = nextTake();
  assert.equal(t.src, null, 'past the last source');
  assert.deepEqual(t.dsts, [], 'past the last destination');
  S.hist = [];
  assert.equal(nextTake(), null);
});

test('protectedLabel names the devices in the badge', async () => {
  const { protectedLabel } = await import('./all.mjs');
  assert.equal(protectedLabel([9]), 'Protected by 9');
  assert.equal(protectedLabel([9, 12]), 'Protected by 9, 12');
});
