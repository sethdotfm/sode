// Logic both layouts share: toggle, keepSearch and the "Last:" chips.
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

// The last patch is this browser's own: sendTakes sets it, History never does.
async function patched(src, dsts, levels = [1], srcName = `X${src}`) {
  const { S, sendTakes } = await import('./all.mjs');
  globalThis.WebSocket = { OPEN: 1 };
  S.ws = { readyState: 1, lastRx: Date.now(), send() {} };
  const it = (n) => ({ n, num: n + 1, name: `X${n}`, label: `X${n}` });
  const s0 = { ...it(src), name: srcName, label: srcName };
  sendTakes({ verb: 'patch', items: dsts.map((d) => ({ dst: it(d), groups: [{ src: s0, levels }] })) });
}

test('lastTake: no Last until this browser has made a take, then that take and not History', async () => {
  const { S, lastTake, lastChip } = await import('./all.mjs');
  S.cfg = { numberOffset: 1, levelNumbering: '1', defaultLevels: '1,2', filterSets: [] };
  S.levels = [{ ui: 1 }, { ui: 2 }];
  S.levelBy = new Map([
    [1, {}],
    [2, {}],
  ]);
  S.srcBy = new Map([3, 4].map((n) => [n, { n, num: n + 1, name: `X${n}` }]));
  S.dstBy = new Map([7, 8].map((n) => [n, { n, num: n + 1, name: `X${n}` }]));
  S.hist = [{ id: 1, src: 3, srcName: 'X3', dst: 7, dstName: 'X7', levels: [1], ok: true }];
  const empty = { src: null, dsts: [], levels: [] };
  assert.equal(lastTake(), null);
  assert.deepEqual(lastChip(lastTake(), empty), { src: false, dst: false, lvl: false });
  await patched(4, [8]);
  const t = lastTake();
  assert.equal(t.src.n, 4);
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [8],
  );
  assert.ok(lastChip(t, empty).src);
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
  await patched(1, [5]);
  let t = nextTake();
  assert.equal(t.src.n, 2);
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [6],
  );
  await patched(2, [7]);
  t = nextTake();
  assert.equal(t.src, null, 'past the last source');
  assert.deepEqual(t.dsts, [], 'past the last destination');
});

test('nextTake: fuzzy takes the next match of the name text, in router order', async () => {
  const store = new Map();
  globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, v), removeItem: (k) => store.delete(k) };
  const { S, nextTake, setPref } = await import('./all.mjs');
  const it = (n, name) => ({ n, num: String(n + 1), name, lname: name.toLowerCase(), label: `${n + 1} ${name}` });
  const srcs = [it(1, 'Cam 1'), it(2, 'Bars'), it(7, 'Cam 2'), it(9, 'Cam 3')];
  S.srcBy = new Map(srcs.map((s) => [s.n, s]));
  S.dstBy = new Map();
  await patched(1, [5], [1], 'Cam 1');
  setPref('nextMode', 'number');
  assert.equal(nextTake().src.name, 'Bars');
  setPref('nextMode', 'fuzzy');
  assert.equal(nextTake().src.name, 'Cam 2');
  setPref('nextMode', '');
});

test('nextTake: fuzzy steps through letters too (#149)', async () => {
  const store = new Map();
  globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, v), removeItem: (k) => store.delete(k) };
  const { S, nextTake, setPref, nameStem } = await import('./all.mjs');
  assert.equal(nameStem('Prmpt A'), 'Prmpt');
  assert.equal(nameStem('Cam 3'), 'Cam');
  assert.equal(nameStem('Cam3'), 'Cam');
  assert.equal(nameStem('Mon 2 B'), 'Mon');
  assert.equal(nameStem('Bars'), 'Bars');
  assert.equal(nameStem('A'), 'A');
  const it = (n, name) => ({ n, num: String(n + 1), name, lname: name.toLowerCase(), label: `${n + 1} ${name}` });
  const srcs = [it(1, 'Prmpt A'), it(2, 'Bars'), it(7, 'Prmpt B'), it(9, 'Prmpt C')];
  S.srcBy = new Map(srcs.map((s) => [s.n, s]));
  S.dstBy = new Map();
  await patched(1, [5], [1], 'Prmpt A');
  setPref('nextMode', 'fuzzy');
  assert.equal(nextTake().src.name, 'Prmpt B');
  setPref('nextMode', '');
});

test('protectedLabel names the devices in the badge', async () => {
  const { protectedLabel } = await import('./all.mjs');
  assert.equal(protectedLabel([9]), 'Protected by 9');
  assert.equal(protectedLabel([9, 12]), 'Protected by 9, 12');
});

test('nextTake: repeating keeps going while the boxes hold the last Next (#195)', async () => {
  const { S, nextTake, usedNext, resetNext } = await import('./all.mjs');
  resetNext();
  const it = (n) => ({ n, num: n + 1, name: `X${n}` });
  S.srcBy = new Map([1, 2, 3, 4].map((n) => [n, it(n)]));
  S.dstBy = new Map([5, 6, 7, 8, 9].map((n) => [n, it(n)]));
  await patched(1, [5, 6]);
  const empty = { src: null, dsts: [] };
  let t = nextTake(empty);
  assert.equal(t.src.n, 2);
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [6, 7],
  );
  usedNext('src', t);
  usedNext('dst', t);
  t = nextTake({ src: it(2), dsts: [it(6), it(7)] });
  assert.equal(t.src.n, 3);
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [7, 8],
  );
  t = nextTake({ src: it(4), dsts: [it(9)] });
  assert.equal(t.src.n, 2);
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [6, 7],
  );
  resetNext();
});

test('nextTake: fuzzy falls back to the next number when the name runs out (#196)', async () => {
  const store = new Map();
  globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, v), removeItem: (k) => store.delete(k) };
  const { S, nextTake, setPref, resetNext } = await import('./all.mjs');
  resetNext();
  const it = (n, name) => ({ n, num: String(n + 1), name, lname: name.toLowerCase(), label: `${n + 1} ${name}` });
  S.srcBy = new Map([it(1, 'Cam 1'), it(2, 'Cam 2'), it(3, 'Bars')].map((s) => [s.n, s]));
  S.dstBy = new Map();
  await patched(2, [5], [1], 'Cam 2');
  setPref('nextMode', 'fuzzy');
  assert.equal(nextTake().src.name, 'Bars');
  await patched(3, [5], [1], 'Bars');
  assert.equal(nextTake().src, null, 'past the end of the list');
  setPref('nextMode', '');
});
