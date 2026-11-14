// Last kept in localStorage: survives a reload, and a single-source redo sets it.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const store = new Map();
globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };
globalThis.WebSocket = { OPEN: 1 };
const winListeners = {};
globalThis.addEventListener = (type, fn) => (winListeners[type] ||= []).push(fn);

const KEY = 'sode-last-take';
const it = (n) => ({ n, num: n + 1, name: `X${n}`, label: `X${n}` });

async function setup() {
  const m = await import('./all.mjs');
  const { S } = m;
  S.cfg = { numberOffset: 1, levelNumbering: '1', defaultLevels: '1,2', filterSets: [] };
  S.levels = [{ ui: 1 }, { ui: 2 }, { ui: 3 }];
  S.levelBy = new Map([1, 2, 3].map((u) => [u, {}]));
  S.srcBy = new Map([1, 2, 3, 4].map((n) => [n, it(n)]));
  S.dstBy = new Map([5, 6, 7, 8].map((n) => [n, it(n)]));
  S.ws = { readyState: 1, lastRx: Date.now(), send() {} };
  return m;
}

// Runs before any take in this process, so only the stored value can answer: a reload.
test('lastTake: a stored Last loads after a reload, with current names', async () => {
  const { S, lastTake } = await setup();
  store.set(
    KEY,
    JSON.stringify({
      src: [2, 'Old'],
      dsts: [
        [5, 'Old5'],
        [6, 'Old6'],
      ],
      levels: [1, 3, 9],
    }),
  );
  S.srcBy.get(2).name = 'Cam 2';
  const t = lastTake();
  assert.equal(t.src.name, 'Cam 2');
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [5, 6],
  );
  assert.deepEqual(t.levels, [1, 3], 'a level the router no longer has is dropped');
  S.srcBy.get(2).name = 'X2';
});

test('lastTake: corrupt or unmatched stored values are no Last', async () => {
  const { lastTake } = await setup();
  for (const v of [
    '{not json',
    '"text"',
    '[]',
    JSON.stringify({ src: 'a', dsts: [[5, 'X5']], levels: [1] }),
    JSON.stringify({ src: [2, 'X2'], dsts: [], levels: [1] }),
    JSON.stringify({ src: [2, 'X2'], dsts: [[5]], levels: [1] }),
    JSON.stringify({ src: [2, 'X2'], dsts: [[5, 'X5']], levels: 'x' }),
    JSON.stringify({ src: [99, 'Gone'], dsts: [[5, 'X5']], levels: [1] }),
    JSON.stringify({
      src: [2, 'X2'],
      dsts: [
        [5, 'X5'],
        [99, 'Gone'],
      ],
      levels: [1],
    }),
  ]) {
    store.set(KEY, v);
    assert.equal(lastTake(), null, v);
  }
  store.delete(KEY);
  assert.equal(lastTake(), null);
});

test('lastTake: memory answers only when storage is blocked or the write failed', async () => {
  const { sendTakes, lastTake, patchPlan } = await setup();
  sendTakes(patchPlan(it(4), [it(6)], [1]));
  store.set(KEY, '{not json');
  assert.equal(lastTake(), null, 'corrupt');
  store.delete(KEY);
  assert.equal(lastTake(), null, 'cleared after a good write');
  const set = localStorage.setItem;
  localStorage.setItem = () => {
    throw new Error('full');
  };
  try {
    store.set(KEY, JSON.stringify({ src: [1, 'X1'], dsts: [[5, 'X5']], levels: [1] }));
    sendTakes(patchPlan(it(4), [it(6)], [1]));
  } finally {
    localStorage.setItem = set;
  }
  assert.equal(store.has(KEY), false, 'a failed write removes the older value');
  assert.equal(lastTake().src.n, 4, 'write failed');
  const get = localStorage.getItem;
  localStorage.getItem = () => {
    throw new Error('blocked');
  };
  try {
    assert.equal(lastTake().src.n, 4, 'blocked');
  } finally {
    localStorage.getItem = get;
  }
});

test("another tab's take redraws the Last chips", async () => {
  const { on } = await setup();
  let n = 0;
  on('last', () => n++);
  for (const fn of winListeners.storage) fn({ key: 'sode-theme' });
  assert.equal(n, 0);
  for (const fn of winListeners.storage) fn({ key: KEY });
  assert.equal(n, 1);
});

test('sendTakes: a patch is stored and reads back as Last', async () => {
  const { sendTakes, lastTake, patchPlan } = await setup();
  sendTakes(patchPlan(it(3), [it(7), it(8)], [2]));
  assert.deepEqual(JSON.parse(store.get(KEY)), {
    src: [3, 'X3'],
    dsts: [
      [7, 'X7'],
      [8, 'X8'],
    ],
    levels: [2],
  });
  const t = lastTake();
  assert.equal(t.src.n, 3);
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [7, 8],
  );
});

const route = (dst, src, levels) => ({ dst: it(dst), groups: [{ src: it(src), levels }], skipped: [], untouched: [], undo: null, retry: null });

test('lastFromPlan: a redo with one source and one set of levels sets Last; others leave it', async () => {
  const { lastFromPlan } = await setup();
  assert.deepEqual(lastFromPlan({ verb: 'redo', items: [route(5, 1, [1, 2]), route(6, 1, [2, 1])] }), {
    src: [1, 'X1'],
    dsts: [
      [5, 'X5'],
      [6, 'X6'],
    ],
    levels: [1, 2],
  });
  assert.equal(lastFromPlan({ verb: 'redo', items: [route(5, 1, [1]), route(6, 2, [1])] }), null, 'several sources');
  assert.equal(lastFromPlan({ verb: 'redo', items: [route(5, 1, [1]), route(6, 1, [2])] }), null, 'different levels');
  assert.equal(lastFromPlan({ verb: 'redo', items: [] }), null);
  assert.equal(lastFromPlan({ verb: 'retry', items: [route(5, 1, [1])] }), null);
  assert.equal(lastFromPlan({ verb: 'preset', items: [route(5, 1, [1])] }), null);
  assert.equal(lastFromPlan({ verb: 'undo', items: [route(5, 1, [1])] }), null);
});

test('sendTakes: a single-source redo updates Last; a Group redo, a Retry and a Preset do not', async () => {
  const { sendTakes, lastTake, patchPlan } = await setup();
  sendTakes(patchPlan(it(1), [it(5)], [1]));
  sendTakes({ verb: 'redo', items: [route(6, 2, [1]), route(7, 3, [1])] });
  sendTakes({ verb: 'retry', items: [route(6, 2, [1])] });
  sendTakes({ verb: 'preset', preset: 'P', items: [route(6, 2, [1])] });
  assert.equal(lastTake().src.n, 1);
  sendTakes({ verb: 'redo', items: [route(8, 4, [3])] });
  const t = lastTake();
  assert.equal(t.src.n, 4);
  assert.deepEqual(
    t.dsts.map((d) => d.n),
    [8],
  );
  assert.deepEqual(t.levels, [3]);
});
