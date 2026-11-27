// History rows of tie takes: one row for each destination asked for, filters on the typed ends only, and stacking.
import test from 'node:test';
import assert from 'node:assert/strict';
import { isHop, typedEnds, soloTies, hopEntriesOf, stackVia, hopsBefore } from '../tiehistoryrules.js';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const store = new Map();
globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };

const setup = async () => {
  const m = await import('./all.mjs');
  const { S, routerData } = m;
  S.cfg = { filterSets: [] };
  Object.assign((await import('./all.mjs')).routerData(''), { off: 1 });
  routerData('').levelBy = new Map([[1, {}]]);
  routerData('').srcBy = new Map();
  routerData('').dstBy = new Map();
  routerData('').tally = new Map();
  S.hist = [];
  return m;
};

const at = (s) => new Date(Date.UTC(2026, 0, 1, 0, 0, s)).toISOString();
// A tie take of router A source 7 (CAM 8) to router B destination 41 (B.42) through TIE 3. The hop routes A.7 to
// A.20, the upstream destination of TIE 3. The destination entry routes B.30, its downstream source, to B.41.
function tieTake({ id, s = 0, dst = 41, tie = 't3', tieName = 'TIE 3', src = 7, srcName = 'CAM 8', group = `g${id}`, ok = true } = {}) {
  const hop = {
    id,
    time: at(s),
    router: 'rA',
    src,
    srcName,
    dst: 20,
    dstName: 'TO B',
    levels: [1],
    prev: [[1, 2]],
    ok: true,
    tie,
    group,
    groupN: 2,
  };
  const hops = [{ router: 'rA', src, dst: 20, levels: [1], tie, tieName, entry: id, result: 'connected', prev: [[1, 2]] }];
  const last = {
    id: id + 1,
    time: at(s),
    router: 'rB',
    src: 30,
    srcName: 'FROM A',
    dst,
    dstName: 'B MON',
    levels: [1],
    prev: [[1, 5]],
    ok,
    origin: { router: 'rA', n: src, name: srcName },
    hops,
    group,
    groupN: 2,
  };
  return [last, hop];
}
const rowKey = (r) => (r.f ? `f${r.f.shown.map((x) => x.id)}` : `${r.e.id}${r.e.chain ? '<' + r.e.chain.map((c) => c.id).join(',') : ''}`);
const rows = (m, list) => {
  m.emit('history');
  return m.histRows(list);
};
const shown = (m, list) => rows(m, list).map(rowKey).join(' ');

test('rules: a hop entry has no typed ends; a tie take matches by its typed source on its own router', () => {
  const [last, hop] = tieTake({ id: 1 });
  assert.equal(isHop(hop), true);
  assert.equal(isHop({ ...hop, ext: true }), false);
  assert.equal(typedEnds(hop), null);
  assert.deepEqual(typedEnds(last), { src: { n: 7, name: 'CAM 8', router: 'rA' }, dst: { n: 41, name: 'B MON', router: 'rB' } });
  const plain = { src: 3, srcName: 'X', dst: 4, dstName: 'Y', router: '' };
  assert.deepEqual(typedEnds(plain), { src: { n: 3, name: 'X', router: '' }, dst: { n: 4, name: 'Y', router: '' } });
});

test('rules: soloTies finds a whole tie take of one destination only', () => {
  const one = tieTake({ id: 1 });
  assert.deepEqual([...soloTies(one)], [['g1', 2]]);
  // Not whole yet: the destination entry has not come.
  assert.equal(soloTies([one[1]]).size, 0);
  // Two destinations through one hop.
  const [a, hop] = tieTake({ id: 1 });
  const two = [
    { ...a, id: 3, dst: 42, groupN: 3 },
    { ...a, groupN: 3 },
    { ...hop, groupN: 3 },
  ];
  assert.equal(soloTies(two).size, 0);
  // A take with a Protect of its tie line keeps its Group.
  const prot = { ...hop, id: 5, tie: undefined, protect: 'set', groupN: 3 };
  assert.equal(soloTies([prot, { ...a, groupN: 3 }, { ...hop, groupN: 3 }]).size, 0);
  assert.deepEqual(
    hopEntriesOf(a, one).map((e) => e.id),
    [1],
  );
});

test('rules: stackVia keeps tie lines apart, and hopsBefore says what each tie line carried before', () => {
  const [a] = tieTake({ id: 1 });
  const [b] = tieTake({ id: 3, tie: 't9' });
  assert.notEqual(stackVia(a), stackVia(b));
  assert.equal(stackVia(a), stackVia(tieTake({ id: 5 })[0]));
  assert.equal(stackVia({ src: 1 }), '');
  assert.deepEqual(hopsBefore(a), [{ name: 'TIE 3', router: 'rA', entry: 1, groups: [{ src: 2, levels: [1] }] }]);
  const already = { ...a, hops: [{ ...a.hops[0], entry: undefined, result: 'already', prev: null }] };
  assert.deepEqual(hopsBefore(already), [{ name: 'TIE 3', router: 'rA', entry: null, groups: [] }]);
});

test('histRows: a tie take of one destination is one row that holds its hop entries', async () => {
  const m = await setup();
  const list = tieTake({ id: 1 });
  const [r] = rows(m, list);
  assert.equal(rowKey(r), '2');
  assert.deepEqual(
    r.e.hopEntries.map((e) => e.id),
    [1],
  );
  const v = m.rowView(r).route(r.e);
  assert.equal(v.via, 'Via TIE 3');
  assert.match(v.src.label, /CAM 8/);
  // Only the hop entry of the Group is on its way: it shows as the Group until the destination entry comes.
  assert.equal(shown(m, [list[1]]), 'f1');
});

test('histRows: a tie take of two destinations is one Group with a row for each destination, no hop rows', async () => {
  const m = await setup();
  const [a, hop] = tieTake({ id: 1 });
  const list = [
    { ...a, id: 3, dst: 42, groupN: 3 },
    { ...a, groupN: 3 },
    { ...hop, groupN: 3 },
  ];
  const [r] = rows(m, list);
  assert.equal(rowKey(r), 'f3,2');
  assert.equal(r.f.entries.length, 3);
  assert.equal(m.rowView(r).route(r.f).notes.length, 0);
});

test('histRows: a failed tie row of one destination offers Put back and Retry; Redo of one that worked opens the picker', async () => {
  const m = await setup();
  const [last, hop] = tieTake({ id: 1, ok: false });
  const [r] = rows(m, [last, hop]);
  const a = m.rowActions(r);
  assert.equal(a.putBack.length, 1);
  assert.match(a.putBack[0].label, /^Put TIE 3 back/);
  assert.equal(a.again.plan.verb, 'retry');
  assert.equal(a.again.plan.items[0].retry.id, 2);
  // The Undo puts back the destination asked for only, never the tie line.
  assert.deepEqual(
    a.undo.plan.items.map((i) => i.dst.n),
    [41],
  );
  const [ok] = rows(m, tieTake({ id: 1 }));
  // Redo is a route from the typed source A.8 with the tie lines of the take, so the confirm opens the picker on it.
  const redo = m.rowActions(ok).again.plan;
  assert.equal(redo.verb, 'redo');
  assert.equal(redo.items.length, 1);
  const [g] = redo.items[0].groups;
  assert.deepEqual([redo.items[0].dst.n, g.src.n, g.ties, g.last], [41, 7, ['t3'], true]);
  assert.equal(m.rowActions(ok).putBack.length, 0);
});

test('Undone: an undone tie row says its tie lines are left as they are; hops do not count in a Group', async () => {
  const m = await setup();
  const [last, hop] = tieTake({ id: 1 });
  const undo = { id: 3, time: at(5), router: 'rB', src: 5, dst: 41, dstName: 'B MON', levels: [1], prev: [[1, 30]], ok: true, undo: 2 };
  const [, r] = rows(m, [undo, last, hop]);
  assert.equal(r.e.undone.text, 'Undone');
  // entryNote then adds "tie lines left as they are".
  assert.equal(r.e.tiesKept, true);
  // A put-back of the hop too: the tie line went back, so the note leaves that out.
  const put = { ...undo, id: 4, router: 'rA', dst: 20, prev: [[1, 7]], undo: 1, putBack: true };
  const [, , r2] = rows(m, [put, undo, last, hop]);
  assert.equal(r2.e.undone.text, 'Undone');
  assert.equal(r2.e.tiesKept, false);
  // A Group of two destinations through one hop: the hop is not a route, so one of two is undone.
  const two = [
    { ...last, id: 3, dst: 42, groupN: 3 },
    { ...last, groupN: 3 },
    { ...hop, groupN: 3 },
  ];
  const [, g] = rows(m, [{ ...undo, id: 4 }, ...two]);
  assert.equal(g.f.undone.text, '1 of 2 undone');
  // Undo all puts back the destinations only.
  assert.deepEqual(
    m.rowActions(g).undo.plan.items.map((i) => i.dst.n),
    [42, 41],
  );
});

test('filters match the typed source and destination, never the hops', async () => {
  const m = await setup();
  const list = tieTake({ id: 1 });
  const f = (o) => m.setFilter({ src: '', dst: '', sode: true, ext: true, hidden: false, ...o });
  try {
    // The typed source A.8 (number 7 + offset 1) and the typed destination B.42.
    f({ src: '8' });
    assert.equal(shown(m, list), '2');
    f({ dst: '42' });
    assert.equal(shown(m, list), '2');
    f({ src: 'CAM 8' });
    assert.equal(shown(m, list), '2');
    // The hop's upstream destination (A.21) and the downstream source of the tie line (B.31) are hops.
    f({ dst: '21' });
    assert.equal(shown(m, list), '');
    f({ dst: 'TO B' });
    assert.equal(shown(m, list), '');
    f({ src: '31' });
    assert.equal(shown(m, list), '');
    f({ src: 'FROM A' });
    assert.equal(shown(m, list), '');
    // A filter on router A matches the typed source, which is on router A; the destination is on router B.
    f({ src: '8', router: 'rA' });
    assert.equal(shown(m, list), '2');
    f({ dst: '42', router: 'rA' });
    assert.equal(shown(m, list), '');
    // In a Group of two destinations, a hop entry never makes a row show.
    const [a, hop] = list;
    const two = [
      { ...a, id: 3, dst: 42, groupN: 3 },
      { ...a, groupN: 3 },
      { ...hop, groupN: 3 },
    ];
    f({ dst: '21' });
    assert.equal(shown(m, two), '');
    f({ dst: '43' });
    assert.equal(shown(m, two), 'f3');
  } finally {
    f({});
  }
});

test('quick search matches the typed ends, never the hops', async () => {
  const m = await setup();
  const list = tieTake({ id: 1 });
  try {
    m.setSearch({ src: 'CAM 8', dst: '' });
    assert.equal(shown(m, list), '2');
    m.setSearch({ src: 'FROM A', dst: '' });
    assert.equal(shown(m, list), '');
    m.setSearch({ src: '', dst: 'TO B' });
    assert.equal(shown(m, list), '');
  } finally {
    m.setSearch({ src: '', dst: '' });
  }
});

test('stacking: a tie row stacks with a tie row through the same tie line, never through another one', async () => {
  const m = await setup();
  const same = [...tieTake({ id: 3, s: 10 }), ...tieTake({ id: 1, s: 0 })];
  assert.equal(shown(m, same), '4<4,2');
  const other = [...tieTake({ id: 3, s: 10, tie: 't9', tieName: 'TIE 9' }), ...tieTake({ id: 1, s: 0 })];
  assert.equal(shown(m, other), '4 2');
  // A route on one router to the same destination does not stack with a tie row.
  const plain = { id: 9, time: at(20), router: 'rB', src: 30, srcName: 'FROM A', dst: 41, dstName: 'B MON', levels: [1], prev: [[1, 0]], ok: true };
  assert.equal(shown(m, [plain, ...tieTake({ id: 1, s: 10 })]), '9 2');
  // A tie take of two destinations is a Group, which never stacks.
  const [a, hop] = tieTake({ id: 5, s: 20 });
  const group = [
    { ...a, id: 7, dst: 42, groupN: 3 },
    { ...a, groupN: 3 },
    { ...hop, groupN: 3 },
  ];
  assert.equal(shown(m, [...group, ...tieTake({ id: 1, s: 10 })]), 'f7,6 2');
  // A failed tie take stays apart for its Retry.
  assert.equal(shown(m, [...tieTake({ id: 3, s: 10, ok: false }), ...tieTake({ id: 1, s: 0 })]), '4 2');
});

// Seams of the Undone mark and the Ctrl+Z step back (stepback.js) with tie takes.
test('Undone: a Group whose destinations are all undone is Undone, though its hop entry was never undone', async () => {
  const m = await setup();
  const [last, hop] = tieTake({ id: 1 });
  const two = [
    { ...last, id: 3, dst: 42, groupN: 3 },
    { ...last, groupN: 3 },
    { ...hop, groupN: 3 },
  ];
  const undo = (id, of, dst) => ({ id, time: at(5), router: 'rB', src: 5, dst, dstName: 'B MON', levels: [1], prev: [[1, 30]], ok: true, undo: of });
  const [, , g] = rows(m, [undo(5, 3, 42), undo(4, 2, 41), ...two]);
  assert.equal(g.f.undone.text, 'Undone');
  // The hop entry has no mark: the tie line is left as it is.
  assert.equal(g.f.entries.find((e) => e.id === 1).undone, undefined);
});

test('Ctrl+Z steps back over a tie take; Ctrl+Shift+Z gives the old tie lines to the picker', async () => {
  const m = await setup();
  const { backStep, forwardStep, entryKey } = await import('../stepback.js');
  const plain = { id: 1, time: at(0), router: 'rB', src: 9, srcName: 'X', dst: 41, dstName: 'B MON', levels: [1], prev: [[1, 0]], ok: true };
  const [last, hop] = tieTake({ id: 2, s: 10 });
  let hist = [last, hop, plain];
  const own = hist.map(entryKey);
  // The step is the destination entry alone: the hop is part of the route asked for.
  const first = backStep(hist, own);
  assert.deepEqual(
    first.entries.map((x) => x.id),
    [3],
  );
  // Its Undo puts back the destination only, never the tie line.
  assert.deepEqual(
    m.rowActions({ e: first.entries[0] }).undo.plan.items.map((i) => [i.undo.id, i.dst.n]),
    [[3, 41]],
  );
  // After the undo, the next Ctrl+Z is the plain take. The hop entry is never a step of its own.
  const undo = { id: 4, time: at(20), router: 'rB', src: 5, dst: 41, dstName: 'B MON', levels: [1], prev: [[1, 30]], ok: true, undo: 3 };
  hist = [undo, ...hist];
  assert.deepEqual(
    backStep(hist, [...own, entryKey(undo)], [first.entries.map(entryKey)]).entries.map((x) => x.id),
    [1],
  );
  // Ctrl+Shift+Z: the Redo of the tie row is a route from the typed source with the tie lines of the take. They are
  // marked as used last time, so the picker opens with them chosen.
  const fwd = forwardStep(hist, [first.entries.map(entryKey)]);
  assert.deepEqual(
    fwd.entries.map((x) => x.id),
    [3],
  );
  const redo = m.rowActions({ e: fwd.entries[0] }).again.plan;
  assert.equal(redo.verb, 'redo');
  const [g] = redo.items[0].groups;
  assert.deepEqual([redo.items[0].dst.n, g.src.n, g.ties, g.last], [41, 7, ['t3'], true]);
});
