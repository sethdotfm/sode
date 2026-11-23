// The Undone mark of History rows (undone.js, and how histRows and entryNote use it).
import test from 'node:test';
import assert from 'node:assert/strict';
import { undoIndex, markFor, groupMark, takeMark, takeLevels } from '../undone.js';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const store = new Map();
globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };

const T0 = Date.UTC(2026, 0, 1);
const take = (id, o = {}) => ({
  id,
  time: new Date(T0 - (100 - id) * 3600000).toISOString(),
  src: 7,
  srcName: 'CAM 8',
  dst: 5,
  dstName: 'MON 6',
  levels: [1, 2],
  prev: [],
  ok: true,
  ...o,
});
const mark = (list, id) => {
  const e = list.find((x) => x.id === id);
  return markFor([id], e.levels, undoIndex(list));
};

test('an undo that confirmed every level marks the take Undone', () => {
  const list = [take(2, { undo: 1 }), take(1)];
  const m = mark(list, 1);
  assert.equal(m.state, 'undone');
  assert.equal(m.text, 'Undone');
  assert.deepEqual(m.by, [2]);
  assert.equal(mark(list, 2), null);
});

test('an undo that failed on some levels gives Partly undone with the levels that went back', () => {
  const list = [take(2, { undo: 1, ok: false, failed: [2] }), take(1)];
  const m = mark(list, 1);
  assert.equal(m.state, 'partly');
  assert.equal(m.text, 'Partly undone');
  assert.deepEqual(m.levels, [1]);
});

test('an undo that failed on every level gives no mark', () => {
  assert.equal(mark([take(2, { undo: 1, ok: false, failed: [1, 2] }), take(1)], 1), null);
});

test('a Retry that confirms moves a partly undone take to Undone', () => {
  const list = [take(3, { retry: 2, levels: [2] }), take(2, { undo: 1, ok: false, failed: [2] }), take(1)];
  const m = mark(list, 1);
  assert.equal(m.state, 'undone');
  assert.deepEqual(m.by, [2]);
  // A Retry of a Retry reaches the undo row too.
  const deep = [take(4, { retry: 3, levels: [2] }), take(3, { retry: 2, levels: [2], ok: false, failed: [2] }), list[1], list[2]];
  assert.equal(mark(deep, 1).state, 'undone');
});

test('a Retry that fails too leaves the mark as it was', () => {
  const list = [take(3, { retry: 2, levels: [2], ok: false, failed: [2] }), take(2, { undo: 1, ok: false, failed: [2] }), take(1)];
  assert.equal(mark(list, 1).state, 'partly');
});

test('a Redo does not change the mark of the original', () => {
  const list = [take(3), take(2, { undo: 1 }), take(1)];
  assert.equal(mark(list, 1).state, 'undone');
  assert.equal(mark(list, 3), null);
});

test('a stacked row is marked when the undo names any entry of its chain', () => {
  const list = [take(4, { undo: 2 }), take(3), take(2), take(1)];
  const m = markFor([3, 2], [1, 2], undoIndex(list));
  assert.equal(m.state, 'undone');
  assert.deepEqual(m.by, [4]);
  assert.equal(markFor([3], [1, 2], undoIndex(list)), null);
});

test('a Group header counts whole routes', () => {
  const es = [1, 2, 3, 4, 5].map((id) => take(id, { dst: id, levels: [1] }));
  const marked = (n, state = 'undone') => es.map((e, i) => (i < n ? { ...e, undone: { state } } : e));
  assert.equal(groupMark(marked(5)).text, 'Undone');
  assert.equal(groupMark(marked(3)).text, '3 of 5 undone');
  assert.equal(groupMark(marked(0)), null);
  assert.equal(groupMark(marked(1, 'partly')).text, 'Partly undone');
});

const setup = async () => {
  const m = await import('./all.mjs');
  const { S, routerData } = m;
  S.cfg = { numberOffset: 1, filterSets: [] };
  routerData('').levelBy = new Map([
    [1, {}],
    [2, {}],
  ]);
  routerData('').srcBy = new Map();
  routerData('').dstBy = new Map();
  routerData('').tally = new Map();
  return m;
};

test('histRows marks the take and its note says by which undo', async () => {
  const { histRows, S } = await setup();
  S.hist = [take(2, { undo: 1 }), take(1)];
  const rows = histRows(S.hist);
  assert.equal(rows.length, 2);
  assert.equal(rows[1].e.undone.text, 'Undone');
  assert.deepEqual(rows[1].e.undone.by, [2]);
  assert.equal(rows[0].e.undone, undefined);
});

test('histRows marks a stacked row from the undo of one of its changes', async () => {
  const { histRows, S } = await setup();
  const close = (id, o = {}) => take(id, { time: new Date(T0 + id * 1000).toISOString(), ...o });
  S.hist = [close(5, { undo: 3, dst: 9 }), close(4, { dst: 5 }), close(3, { dst: 5 }), close(2, { dst: 5 })].reverse().reverse();
  const row = histRows(S.hist).find((r) => r.e.chain);
  assert.equal(row.e.chain.length, 3);
  assert.equal(row.e.undone.state, 'undone');
});

test('histRows marks each route of a Group and the header', async () => {
  const { histRows, S } = await setup();
  const g = (id, o = {}) => take(id, { group: 'g1', groupN: 2, dst: id, levels: [1], ...o });
  const u = (id, of, o = {}) => take(id, { group: 'g2', groupN: 2, dst: of, levels: [1], undo: of, ...o });
  S.hist = [u(14, 12), u(13, 11), g(12), g(11)];
  const rows = histRows(S.hist);
  const f = rows.find((r) => r.f && r.f.group === 'g1').f;
  assert.equal(f.undone.text, 'Undone');
  assert.ok(f.entries.every((e) => e.undone?.state === 'undone'));
  S.hist = [u(13, 11), g(12), g(11)].map((e) => (e.id === 13 ? { ...e, groupN: 1 } : e));
  const half = histRows(S.hist).find((r) => r.f).f;
  assert.equal(half.undone.text, '1 of 2 undone');
});

test('an undo of a partly failed take that puts back every confirmed level marks it Undone', () => {
  // Level 2 failed and the router answered for it, so the undo (untouchedLevels in plans.js) leaves it out.
  const list = [take(2, { undo: 1, levels: [1] }), take(1, { ok: false, failed: [2], now: [[2, 4]] })];
  assert.equal(takeMark(list[1], undoIndex(list)).state, 'undone');
  assert.deepEqual(takeLevels(take(1, { ok: false, failed: [1, 2] })), [1, 2], 'a take that confirmed nothing keeps its levels');
});

test('histRows marks a partly failed take Undone when its undo put back the levels it confirmed', async () => {
  const { histRows, S } = await setup();
  S.hist = [take(2, { undo: 1, levels: [1] }), take(1, { ok: false, failed: [2], now: [[2, 4]] })];
  assert.equal(histRows(S.hist)[1].e.undone.text, 'Undone');
});
