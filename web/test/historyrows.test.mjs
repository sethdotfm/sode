// History rows: the wording and plans that the two layouts share.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const store = new Map();
globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };

const setup = async () => {
  const m = await import('./all.mjs');
  const { S } = m;
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}]]);
  S.srcBy = new Map([[7, { n: 7, num: 8, name: 'CAM 8', label: 'CAM 8' }]]);
  S.dstBy = new Map([[5, { n: 5, num: 6, name: 'MON 6', label: 'MON 6' }]]);
  S.tally = new Map();
  S.hist = [];
  return m;
};
const take = (o = {}) => ({ id: 1, src: 7, srcName: 'CAM 8', dst: 5, dstName: 'MON 6', levels: [1], prev: [[1, 0]], ok: true, ...o });

test('rowActions: a take', async () => {
  const { rowActions } = await setup();
  const a = rowActions({ e: take() });
  assert.equal(a.undo.text, 'Undo');
  assert.equal(a.undo.tip, undefined);
  assert.match(a.undo.label, /^Undo .+ to .+$/);
  assert.match(a.again.label, /^Redo .+ to .+$/);
  assert.equal(a.again.plan.verb, 'redo');
  assert.equal(rowActions({ e: take({ ok: false }) }).again.label.startsWith('Retry '), true);
});

test('rowActions: a stack of changes', async () => {
  const { rowActions } = await setup();
  const a = rowActions({ e: take({ chain: [{}, {}] }) });
  assert.equal(a.undo.text, 'Undo 2');
  assert.match(a.undo.label, /^Undo 2 changes on /);
  assert.equal(a.undo.tip, 'Restores the source from before these changes');
});

test('rowActions: a folder', async () => {
  const { rowActions } = await setup();
  const { UNDO_TIP_ALL } = await import('../widgets.js');
  const es = [take({ id: 1 }), take({ id: 2 })];
  const f = { group: 'g', entries: es, shown: es };
  const a = rowActions({ f });
  assert.match(a.undo.label, /^Undo all 2 routes of /);
  assert.equal(a.undo.text, 'Undo all');
  assert.equal(a.undo.tip, UNDO_TIP_ALL);
  assert.match(a.again.label, /^Redo 2 routes of /);
  const one = rowActions({ f: { group: 'g', entries: [take()], shown: [] } });
  assert.match(one.again.label, /^Redo 1 route of /);
});

// Sode's own Protect entries; t is seconds after a fixed start.
const prot = (id, dst, t, o = {}) => ({
  id,
  time: new Date(Date.UTC(2026, 0, 1, 0, 0, t)).toISOString(),
  client: 'Ana',
  src: -1,
  dst,
  dstName: `MON ${dst + 1}`,
  levels: [1],
  prev: [],
  ok: true,
  protect: 'set',
  ...o,
});

test('histRows: one Protect action on many destinations is one Group, and quick actions fold', async () => {
  const { histRows, emit, rowView } = await setup();
  const a = [prot(3, 2, 0, { group: 'pA', groupN: 2 }), prot(2, 1, 0, { group: 'pA', groupN: 2 })];
  const b = [prot(5, 4, 10, { group: 'pB', groupN: 2 }), prot(4, 3, 10, { group: 'pB', groupN: 2 })];
  emit('history');
  let rows = histRows([...b, ...a]);
  assert.equal(rows.length, 1);
  assert.equal(rows[0].f.group, 'pA');
  assert.equal(rows[0].f.entries.length, 4);
  assert.equal(rows[0].f.folded, 2);
  const route = (f) => rowView({ f }).route(f);
  assert.deepEqual([route(rows[0].f).dst, route(rows[0].f).notes], ['4 destinations', ['2 actions']]);
  // Two actions of one destination each say no more than "2 destinations".
  emit('history');
  const two = histRows([prot(8, 6, 1), prot(7, 5, 0)])[0].f;
  assert.deepEqual([route(two).dst, route(two).notes], ['2 destinations', []]);
  // A Free Protect stays apart.
  emit('history');
  rows = histRows([prot(6, 1, 12, { protect: 'clear' }), ...b, ...a]);
  assert.equal(rows.length, 2);
  assert.ok(rows[0].e && rows[1].f);
});

test('rowActions: a Group of Protect entries offers Undo all of what it can, and no Redo', async () => {
  const { rowActions, S } = await setup();
  S.ws = {};
  S.wsState = 'up';
  S.router = 'up';
  S.cfg.protectDevice = 126;
  S.myRights = { protect: true };
  S.levels = [{ ui: 1 }];
  S.dstBy = new Map([1, 2, 3].map((n) => [n, { n, num: n + 1, name: `MON ${n + 1}`, label: `MON ${n + 1}` }]));
  S.protBy = new Map();
  // 1 and 2 still hold Sode's Protect; 3 was freed since.
  S.prot = new Map([1, 2].map((d) => [d, new Map([[1, { dev: 126, kind: 1 }]])]));
  const es = [1, 2, 3].map((d) => prot(10 - d, d, 0, { group: 'pA', groupN: 3 }));
  S.hist = [prot(20, 3, 5, { protect: 'clear' }), ...es];
  const a = rowActions({ f: { group: 'pA', n: 3, entries: es, shown: es } });
  assert.equal(a.undo.text, 'Undo all');
  assert.match(a.undo.label, /^Undo all 3 Protects of #7–#9$/);
  assert.equal(a.again.plan.items.length, 0);
  const p = a.undo.plan.protect;
  assert.equal(p.on, false);
  assert.deepEqual(
    p.dsts.map((d) => d.dst.n),
    [1, 2],
  );
  assert.deepEqual(
    p.dsts.map((d) => d.undo.id),
    [9, 8],
  );
  assert.equal(a.undo.plan.skipped, 1);
  // Another system's Group has nothing to undo.
  const ext = es.map((e) => ({ ...e, ext: true, client: '' }));
  assert.equal(rowActions({ f: { group: 'x', n: 3, entries: ext, shown: ext } }).undo.plan.protect, undefined);
});

test('disclosureAria and followersHead', async () => {
  const { disclosureAria, followersHead } = await setup();
  assert.equal(disclosureAria(true, 3, 'takes'), 'Hide the 3 takes');
  assert.equal(disclosureAria(false, 2, 'External follow entries'), 'Show the 2 External follow entries');
  assert.deepEqual(followersHead({ group: 'g', followers: [1, 2] }, false, 'm'), {
    count: '2',
    aria: 'Show the 2 External follow entries',
    kidsId: 'm-followers-g',
  });
});

test('lastFill: nothing, empty side, and a plain fill', async () => {
  const { lastFill } = await setup();
  assert.deepEqual(lastFill('src', false, null), { none: true });
});

// Stacked and folded rows (docs/agents/history.md): a row's key is its entry ids, a stack as newest<chain.
const rowKey = (r) => (r.f ? `f${r.f.entries.map((x) => x.id)}` : `${r.e.id}${r.e.chain ? '<' + r.e.chain.map((c) => c.id).join(',') : ''}`);
const shown = (m, list) => {
  m.emit('history');
  return m.histRows(list).map(rowKey).join(' ');
};

test('histRows: a stack runs on through rows of other destinations', async () => {
  const m = await setup();
  const t = (id, dst, s, src) => take({ id, dst, src, time: new Date(Date.UTC(2026, 0, 1, 0, 0, s)).toISOString() });
  assert.equal(shown(m, [t(3, 5, 20, 7), t(2, 4, 10, 7), t(1, 5, 0, 6)]), '3<3,1 2');
});

test('histRows: Protect rows fold at exactly the gap, not past it, and never with the gap off', async () => {
  const m = await setup();
  assert.equal(shown(m, [prot(2, 2, 30), prot(1, 1, 0)]), 'f2,1');
  assert.equal(shown(m, [prot(2, 2, 31), prot(1, 1, 0)]), '2 1');
  try {
    m.setPref('stackGap', '0');
    assert.equal(shown(m, [prot(2, 2, 1), prot(1, 1, 0)]), '2 1');
  } finally {
    m.setPref('stackGap', '');
  }
});

test('histRows: only the same direction folds; a Group of both directions stays apart', async () => {
  const m = await setup();
  assert.equal(shown(m, [prot(3, 3, 2, { protect: 'clear' }), prot(2, 2, 1), prot(1, 1, 0)]), '3 f2,1');
  const mixed = [prot(3, 3, 1, { group: 'pM', groupN: 2 }), prot(2, 2, 1, { group: 'pM', groupN: 2, protect: 'clear' })];
  assert.equal(shown(m, [prot(4, 4, 2), ...mixed, prot(1, 1, 0)]), '4 f3,2 1');
});

test('histRows: a folded row keeps its id when a new action joins it, so an open row stays open', async () => {
  const m = await setup();
  const old = [prot(2, 2, 10), prot(1, 1, 0)];
  m.emit('history');
  const before = m.histRows(old)[0].f.group;
  m.emit('history');
  assert.equal(m.histRows([prot(3, 3, 20), ...old])[0].f.group, before);
  const g = [prot(2, 2, 0, { group: 'pA', groupN: 2 }), prot(1, 1, 0, { group: 'pA', groupN: 2 })];
  m.emit('history');
  assert.equal(m.histRows(g)[0].f.group, 'pA');
  m.emit('history');
  assert.equal(m.histRows([prot(3, 3, 5), ...g])[0].f.group, 'pA');
});
