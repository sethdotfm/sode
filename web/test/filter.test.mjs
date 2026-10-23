// Tests for the Recent filter language and history filtering. Not embedded.
// docker run --rm -v "$PWD/web":/w node:22-alpine node --test '/w/test/*.test.mjs'
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseFilter, filterText, compileFilter, filterFn } from '../filter.js';

const kinds = (t) => parseFilter(t).rules.map((r) => `${r.not ? '!' : ''}${r.kind}:${r.text ?? `${r.a}${r.b != null ? '-' + r.b : ''}`}`);

test('terms', () => {
  assert.deepEqual(kinds('65, 1-50, 50 thru 1, 3 to 4, PIP, "5D", /^DSM\\d/, !65, not CAM 1, ! MV'),
    ['num:65', 'range:1-50', 'range:1-50', 'range:3-4', 'name:PIP', 'name:5D', 're:^DSM\\d', '!num:65', '!name:CAM 1', '!name:MV']);
  assert.deepEqual(kinds(''), []);
  assert.deepEqual(kinds(' , ,'), []);
  assert.deepEqual(kinds('"a ""b"", c"'), ['name:a "b", c']);
  assert.deepEqual(kinds('/a,b/'), ['re:a,b']);
  // Single letters aren't words here: names keep them.
  assert.deepEqual(kinds('TX 1, 1 t 5'), ['name:TX 1', 'name:1 t 5']);
});

test('errors', () => {
  for (const t of ['"open', '/open', '/(/', '!', '"x" y', '/x/ y', '""']) {
    assert.ok(parseFilter(t).error, t);
  }
  assert.equal(filterFn('"open'), null);
});

test('matching', () => {
  const m = filterFn('MV, 1-3, !MV 1, !2');
  assert.equal(m(1, 'PGM'), true);
  assert.equal(m(2, 'PVW'), false); // excluded number
  assert.equal(m(9, 'MV 12'), false); // "MV 1" contains-match excludes MV 12 too
  assert.equal(m(9, 'MV 2'), true);
  assert.equal(m(9, 'DSM'), false);
  assert.equal(filterFn('')(5, 'x'), true);
  assert.equal(filterFn('!x')(5, 'abc'), true);
  assert.equal(filterFn('!x')(5, 'xyz'), false);
  assert.equal(filterFn('/^dsm\\d$/')(1, 'DSM3'), true); // ignores case by default
  assert.equal(filterFn('/^dsm/g')(1, 'DSM3'), false); // flags given: as written
  const fz = filterFn('mv28, !pgm', { fuzzy: true });
  assert.equal(fz(600, 'MV 28'), true); // letters in order
  assert.equal(fz(601, 'MV08 PIP028'), true);
  assert.equal(fz(602, 'DSM 3'), false);
  assert.equal(filterFn('mv28')(600, 'MV 28'), false); // Filters stay exact
});

test('round trip', () => {
  for (const t of ['65, 1-50, PIP, "5D", "1-2", "!x", "not y", "a, b", /x\\/y/i, !CAM 1']) {
    const r = parseFilter(t).rules;
    assert.deepEqual(parseFilter(filterText(r)).rules, r, t);
  }
  assert.equal(filterText(parseFilter('50 thru 1, not  CAM').rules), '1-50, !CAM');
  assert.equal(filterText([{ not: false, kind: 'name', text: 'say "hi"' }]), '"say ""hi"""');
  assert.equal(compileFilter([])(1, ''), true);
});

test('histRows', async () => {
  const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
  globalThis.document = node();
  const store = new Map();
  globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, v), removeItem: (k) => store.delete(k) };
  store.set('sode-ext', 'none'); // first beta's setting migrates
  const { histRows, S, setFilter, filterState, filterCount, FILTER_DEFAULT } = await import('../core.js');
  assert.deepEqual(filterState(), { ...FILTER_DEFAULT, ext: false });
  assert.equal(store.has('sode-ext'), false);

  S.cfg = { numberOffset: 1, hiddenDsts: 'MON' };
  const T = (s) => new Date(Date.UTC(2026, 9, 23, 12, 0, s)).toISOString();
  const L = [1, 2];
  const e = (o) => ({ levels: L, prev: [], ok: true, srcName: 'CAM', dstName: 'MV', ...o });
  const hist = [
    e({ id: 9, ext: true, dst: 2, src: 5, time: T(50), prev: [[1, 4]] }),
    e({ id: 8, ext: true, dst: 2, src: 4, time: T(40), prev: [[1, 2]] }),
    e({ id: 7, ext: true, dst: 8, src: 1, time: T(35), dstName: 'MON 1' }), // globally hidden
    e({ id: 6, ext: true, dst: 2, src: 2, time: T(30), prev: [[1, 0]] }),
    e({ id: 5, dst: 2, src: 0, time: T(20), srcName: 'BARS' }), // a Sode take breaks the chain
    e({ id: 4, ext: true, dst: 2, src: 7, time: T(10) }),
    e({ id: 3, dst: 0, src: 1, time: T(5), dstName: 'PGM', group: 'g', groupN: 2 }),
    e({ id: 2, dst: 1, src: 1, time: T(5), dstName: 'PVW', group: 'g', groupN: 2 }),
  ];
  const show = () => histRows(hist).map((r) => (r.f ? `f${r.f.shown.map((x) => x.id)}/${r.f.entries.length}` : `${r.e.id}${r.e.merged ? 'x' + r.e.merged : ''}`)).join(' ');

  assert.equal(show(), '5 f3,2/2');
  setFilter(FILTER_DEFAULT);
  assert.equal(filterCount(), 0);
  assert.equal(show(), '9x3 5 4 f3,2/2');
  setFilter({ ...FILTER_DEFAULT, hidden: true });
  assert.equal(show(), '9x3 7 5 4 f3,2/2');
  setFilter({ ...FILTER_DEFAULT, dst: 'PGM' }); // number 1 = dst 0
  assert.equal(show(), 'f3/2');
  setFilter({ ...FILTER_DEFAULT, src: '!BARS', sode: false });
  assert.equal(show(), '9x3 4');
  assert.equal(filterCount(), 2);
  setFilter({ ...FILTER_DEFAULT, dst: '"open' }); // bad text keeps the last good matcher
  assert.equal(show(), '9x3 5 4 f3,2/2');
});

test('filter sets', async () => {
  const core = await import('../core.js'); // same instance as the histRows test
  const { S, setFilter, filterState, activeSet, pickSet, saveLocalSet, deleteLocalSet, allSets, FILTER_DEFAULT } = core;
  S.cfg = { numberOffset: 1, filterSets: [{ name: 'No shader', src: '', dst: '!MON', sode: true, ext: true, hidden: false }] };
  assert.deepEqual(allSets().map((s) => `${s.scope}:${s.name}`), ['all:All', 'global:No shader']);

  pickSet('global', 'No shader');
  assert.equal(filterState().dst, '!MON');
  assert.equal(activeSet().edited, false);
  setFilter({ ...filterState(), dst: '!MON, !PVW' });
  assert.equal(activeSet().edited, true);
  saveLocalSet('Mine');
  assert.deepEqual(activeSet() && [activeSet().scope, activeSet().name, activeSet().edited], ['local', 'Mine', false]);
  assert.equal(JSON.parse(localStorage.getItem('sode-filter-sets'))[0].f.dst, '!MON, !PVW');
  saveLocalSet('mine'); // same name, any case: replaced
  assert.equal(allSets().filter((s) => s.scope === 'local').length, 1);
  core.renameLocalSet('mine', 'Desk');
  assert.deepEqual([activeSet().name, activeSet().edited], ['Desk', false]);
  deleteLocalSet('desk');
  assert.equal(activeSet(), null);
  assert.equal(localStorage.getItem('sode-filter-sets'), null);

  pickSet('all', 'All');
  assert.deepEqual(filterState(), { ...FILTER_DEFAULT });
});

test('follows and merged chains', async () => {
  const core = await import('../core.js');
  const { S, histRows, undoPlan, folderUndoPlan, sendTakes, pickSet } = core;
  pickSet('all', 'All');
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}], [2, {}]]);
  const T = (s) => new Date(Date.UTC(2026, 9, 23, 13, 0, s)).toISOString();
  const e = (o) => ({ levels: [1, 2], prev: [[1, 0], [2, 0]], ok: true, srcName: 'CAM', dstName: 'X', time: T(0), ...o });
  const hist = [
    e({ id: 9, ext: true, dst: 40, src: 3, follow: 8, time: T(31) }), // PiP following #8
    e({ id: 8, dst: 2, src: 3, time: T(30) }), // a Sode take
    e({ id: 7, ext: true, dst: 41, src: 5, follow: 4, time: T(21) }), // follow of an unloaded take
    e({ id: 6, ext: true, dst: 5, src: 2, time: T(20), prev: [[1, 1], [2, 1]] }),
    e({ id: 5, ext: true, dst: 5, src: 1, time: T(10), prev: [[1, 9], [2, 9]] }),
    e({ id: 3, dst: 1, src: 4, time: T(5), group: 'g', groupN: 2 }),
    e({ id: 2, dst: 0, src: 4, time: T(5), group: 'g', groupN: 2 }),
    e({ id: 1, ext: true, dst: 42, src: 4, follow: 3, time: T(5) }),
  ];
  S.hist = hist;
  const rows = histRows(hist);
  assert.deepEqual(rows.map((r) => (r.f ? 'f' : r.e.id)), [8, 7, 6, 'f']);
  const take = rows[0].e;
  assert.deepEqual(take.follows.map((f) => f.id), [9]);
  assert.equal(rows[1].e.follow, 4); // shown on its own, noted as a follow
  const merged = rows[2].e;
  assert.deepEqual([merged.merged, merged.chain.map((c) => c.id), merged.prev[0]], [2, [6, 5], [1, 9]]);
  assert.deepEqual(rows[3].f.entries.find((x) => x.id === 3).follows.map((f) => f.id), [1]);

  // Undo leaves follows out unless the box is ticked.
  const sent = [];
  globalThis.WebSocket = { OPEN: 1 };
  S.ws = { readyState: 1, send: (m) => sent.push(JSON.parse(m)) };
  const plan = undoPlan(take);
  assert.equal(plan.extra.items.length, 1);
  sendTakes({ ...plan });
  assert.deepEqual(sent.pop().items.map((i) => i.dst), [2]);
  plan.extra.on = true;
  sendTakes({ ...plan }); // a copy, as desktop's confirm makes
  assert.deepEqual(sent.pop().items.map((i) => i.dst), [2, 40]);
  assert.equal(folderUndoPlan(rows[3].f).extra.items.length, 1);
  assert.equal(undoPlan(rows[2].e).extra, undefined);
  // Two follows of one destination: one undo, back to before the older.
  const twice = undoPlan({ ...take, follows: [e({ id: 12, ext: true, dst: 40, src: 6, prev: [[1, 3], [2, 3]] }), e({ id: 9, ext: true, dst: 40, src: 3 })] });
  assert.deepEqual(twice.extra.items.map((i) => [i.dst.n, i.groups[0].src.n]), [[40, 0]]);
});

test('quick search', async () => {
  const core = await import('../core.js');
  const { S, histRows, setSearch, setFilter, setSearchRespectsFilters, FILTER_DEFAULT } = core;
  S.cfg = { numberOffset: 1, filterSets: [] };
  const T = (s) => new Date(Date.UTC(2026, 9, 23, 14, 0, s)).toISOString();
  const e = (o) => ({ levels: [1], prev: [], ok: true, time: T(0), ...o });
  const hist = [
    e({ id: 5, ext: true, dst: 40, dstName: 'MV 28', src: 3, srcName: 'CAM 3', follow: 4, time: T(31) }),
    e({ id: 4, dst: 2, dstName: 'DSM1', src: 3, srcName: 'CAM 3', time: T(30) }),
    e({ id: 3, ext: true, dst: 7, dstName: 'MON', src: 1, srcName: 'CAM 1', time: T(20) }),
    e({ id: 2, ext: true, dst: 7, dstName: 'MON', src: 2, srcName: 'CAM 2', time: T(10) }),
  ];
  S.hist = hist;
  const ids = () => histRows(hist).map((r) => `${r.e.id}${r.e.merged ? 'x' + r.e.merged : ''}`).join(' ');
  setFilter({ ...FILTER_DEFAULT, ext: false });
  assert.equal(ids(), '4');
  // Searching ignores the filters, finds takes by their follows, and
  // doesn't merge.
  setSearch({ src: '', dst: 'MON' });
  assert.equal(ids(), '3 2');
  setSearch({ src: '', dst: 'MV 28' });
  assert.equal(ids(), '4');
  setSearch({ src: 'CAM 2', dst: '' });
  assert.equal(ids(), '2');
  setSearchRespectsFilters(true);
  assert.equal(ids(), '');
  setSearchRespectsFilters(false);
  setSearch({ src: '', dst: '' });
  setFilter(FILTER_DEFAULT);
  assert.equal(ids(), '4 3x2');
});
