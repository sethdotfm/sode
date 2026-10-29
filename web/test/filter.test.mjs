// Tests for the Recent filter language and history filtering. Not embedded.
// docker run --rm -v "$PWD/web":/w node:22-alpine node --test '/w/test/*.test.mjs'
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseFilter, filterText, compileFilter, filterFn, entryAllows, stackAllows } from '../filter.js';

const kinds = (t) => parseFilter(t).rules.map((r) => `${r.not ? '!' : ''}${r.kind}:${r.text ?? `${r.a}${r.b != null ? '-' + r.b : ''}`}`);

test('terms', () => {
  assert.deepEqual(kinds('65, 1-50, 50 thru 1, 3 to 4, PIP, "5D", /^DSM\\d/, !65, !CAM 1, ! MV'),
    ['num:65', 'range:1-50', 'range:1-50', 'range:3-4', 'name:PIP', 'exact:5D', 're:^DSM\\d', '!num:65', '!name:CAM 1', '!name:MV']);
  // "not" only before a number, quote or regex; otherwise it's part of a name.
  assert.deepEqual(kinds("not 65, not 'MV', not /x/, NOT USED, 'NOT USED', 'it''s'"),
    ['!num:65', '!name:MV', '!re:x', 'name:NOT USED', 'name:NOT USED', "name:it's"]);
  assert.deepEqual(kinds(''), []);
  assert.deepEqual(kinds(' , ,'), []);
  assert.deepEqual(kinds('"a ""b"", c"'), ['exact:a "b", c']);
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
  assert.equal(filterFn('/^dsm/s')(1, 'DSM3'), false); // flags given: as written
  // g and y would make the regex remember where it stopped: dropped.
  const g = filterFn('/MV/g');
  assert.deepEqual([g(1, 'MV 1'), g(2, 'MV 2'), g(3, 'MV 3')], [true, true, true]);
  // Exact names, and literal quoted text even when searching.
  const ex = filterFn('"CAM 1"');
  assert.deepEqual([ex(1, 'cam 1'), ex(2, 'CAM 10')], [true, false]);
  assert.equal(filterFn("'NOT USED'")(3, 'MV NOT USED'), true);
  assert.equal(filterFn("'mv28'", { fuzzy: true })(4, 'MV 28'), false);
  const fz = filterFn('mv28, prodmon, dsm3, cam 1, !pgm', { fuzzy: true });
  assert.equal(fz(600, 'MV 28'), true); // spaces, case and zeros don't count
  assert.equal(fz(601, 'PROD MON03'), true);
  assert.equal(fz(602, 'DSM 03'), true);
  assert.equal(fz(603, 'CAM 12'), true); // every word in the name
  assert.equal(fz(604, 'MV 76'), false);
  assert.equal(fz(605, 'MV08 PIP028'), false); // not just letters in order
  assert.equal(fz(606, 'PGM DSM3'), false);
  assert.equal(filterFn('mv28')(600, 'MV 28'), false); // Filters stay exact
});

test('round trip', () => {
  for (const t of ['65, 1-50, PIP, "5D", "1-2", "!x", "a, b", /x\\/y/i, !CAM 1', "'1-2', 'a, b', 'it''s', NOT USED, not 'x', '!y'"]) {
    const r = parseFilter(t).rules;
    assert.deepEqual(parseFilter(filterText(r)).rules, r, t);
  }
  assert.equal(filterText(parseFilter("50 thru 1, not  'CAM'").rules), "1-50, !'CAM'");
  assert.equal(filterText([{ not: false, kind: 'name', text: 'say "hi"' }]), `'say "hi"'`);
  assert.equal(filterText([{ not: false, kind: 'exact', text: 'say "hi"' }]), '"say ""hi"""');
  assert.equal(filterText([{ not: false, kind: 'name', text: '12' }]), "'12'");
  assert.equal(compileFilter([])(1, ''), true);
  // A regex from Visual mode with a slash in it still reads back.
  const re = [{ not: false, kind: 're', text: '^MV/PIP', flags: '' }];
  assert.equal(filterText(re), '/^MV\\/PIP/');
  assert.equal(compileFilter(parseFilter(filterText(re)).rules)(1, 'MV/PIP 3'), true);
});

test('histRows', async () => {
  const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
  globalThis.document = node();
  const store = new Map();
  globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, v), removeItem: (k) => store.delete(k) };
  store.set('sode-ext', 'none'); // first beta's setting migrates
  const { histRows, S, setFilter, filterState, filterCount, FILTER_DEFAULT } = await import('./all.mjs');
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
  const core = await import('./all.mjs'); // same instance as the histRows test
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
  const core = await import('./all.mjs');
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
  S.ws = { readyState: 1, lastRx: Date.now(), send: (m) => sent.push(JSON.parse(m)) };
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
  const core = await import('./all.mjs');
  const { S, histRows, setSearch, setFilter, setPref, FILTER_DEFAULT } = core;
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
  // Names logged with the entry count as well as the router's now.
  S.dstBy = new Map([[2, { n: 2, name: 'MV 76', num: '3', lname: 'mv 76', label: '3 MV 76' }]]);
  setSearch({ src: '', dst: 'dsm1' });
  assert.equal(ids(), '4');
  S.dstBy = new Map();
  setSearch({ src: 'CAM 2', dst: '' });
  setPref('searchFilters', 'on');
  assert.equal(ids(), '');
  setPref('searchFilters', 'off');
  setSearch({ src: '', dst: '' });
  setFilter(FILTER_DEFAULT);
  assert.equal(ids(), '4 3x2');
});

test('follows whose take is hidden, and trimming after a search', async () => {
  const core = await import('./all.mjs');
  const { S, histRows, setFilter, setSearch, FILTER_DEFAULT } = core;
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.srcBy = new Map(); S.dstBy = new Map();
  const T = (s) => new Date(Date.UTC(2026, 9, 23, 15, 0, s)).toISOString();
  const e = (o) => ({ levels: [1], prev: [], ok: true, time: T(0), ...o });
  const hist = [
    e({ id: 2, ext: true, dst: 40, dstName: 'MV 3', src: 3, srcName: 'CAM 3', follow: 1, time: T(1) }),
    e({ id: 1, dst: 2, dstName: 'DSM 1', src: 3, srcName: 'CAM 3', time: T(0) }),
  ];
  const rows = () => histRows(hist).map((r) => `${r.e.id}${r.e.follows ? '+' + r.e.follows.length : ''}`).join(' ');
  setFilter(FILTER_DEFAULT);
  assert.equal(rows(), '1+1');
  // Watching multiviewers only: the DSM take is hidden, its MV follow isn't.
  setFilter({ ...FILTER_DEFAULT, dst: 'MV' });
  assert.equal(rows(), '2');
  setFilter(FILTER_DEFAULT);

  // A search reads far back; clearing it keeps the newest entries only,
  // whole groups, and Load more takes it from there.
  S.hist = Array.from({ length: 450 }, (_, i) => e({ id: 450 - i, dst: 1, src: 1, ...(i >= 198 && i < 203 ? { group: 'g', groupN: 5 } : {}) }));
  S.histMore = false;
  setSearch({ src: 'x', dst: '' });
  setSearch({ src: '', dst: '' });
  assert.equal(S.hist.length, 203);
  assert.equal(S.histMore, true);
});

test('history pages', async () => {
  const { S, takePage, setSearch } = await import('./all.mjs');
  const ents = (from, n) => Array.from({ length: n }, (_, i) => ({ id: from - i, levels: [1], prev: [], ok: true, src: 0, dst: 0, time: new Date(0).toISOString() }));
  // A search read far back; the next page (before #100) is on its way.
  S.hist = ents(999, 900);
  S.histMore = true;
  S.histLoading = true;
  setSearch({ src: 'x', dst: '' });
  setSearch({ src: '', dst: '' }); // cleared: trimmed to the newest 200
  assert.equal(S.hist.length, 200);
  assert.equal(S.histLoading, false);
  // The page asked for before the trim lands late: ignored.
  assert.equal(takePage({ before: 100, entries: ents(99, 10), more: true }), false);
  assert.equal(S.hist.length, 200);
  // The page now asked for (before the oldest kept, #800) is taken.
  S.histLoading = true;
  assert.equal(takePage({ before: 800, entries: ents(799, 10), more: true }), true);
  assert.equal(S.hist.length, 210);
  assert.equal(S.hist.at(-1).id, 790);
  assert.equal(S.histLoading, false);
  // Not asked for at all: ignored.
  assert.equal(takePage({ before: 790, entries: ents(789, 10), more: true }), false);
  // History cleared while a page was on its way: ignored, even once
  // another request is out.
  S.hist = [];
  S.histLoading = true;
  assert.equal(takePage({ before: 790, entries: ents(789, 10), more: true }), false);
  assert.equal(S.hist.length, 0);
});

test('undo warns about changes since', async () => {
  const { S, undoPlan, restorePlan, changedSince } = await import('./all.mjs');
  S.cfg = { ...S.cfg, numberOffset: 1 };
  S.levelBy = new Map([1, 2, 3].map((ui) => [ui, { ui, label: String(ui) }]));
  // #20 put src 7 on dst 5, levels 1-3, replacing src 1.
  const e = { id: 20, src: 7, dst: 5, levels: [1, 2, 3], prev: [[1, 1], [2, 1], [3, 1]], ok: true, srcName: '', dstName: '', time: new Date(0).toISOString() };
  S.tally = new Map([[5, new Map([[1, 7], [2, 7], [3, 7]])]]);
  assert.deepEqual(changedSince(undoPlan(e).items[0]), []);
  // Since then, levels 2 and 3 went to src 9 (a panel, or found at resync).
  S.tally.get(5).set(2, 9).set(3, 9);
  assert.deepEqual(changedSince(undoPlan(e).items[0]), [{ src: 9, levels: [2, 3] }]);
  // A failed take may not have switched them: no warning.
  assert.deepEqual(changedSince(undoPlan({ ...e, ok: false }).items[0]), []);
  // A patch item undoes nothing.
  assert.deepEqual(changedSince({ ...undoPlan(e).items[0], undo: null }), []);
  assert.ok(restorePlan(e, e).at); // Restore plans skip the warning (planSummary)
});

test('found and announced changes stack apart', async () => {
  const { histRows, setFilter, FILTER_DEFAULT } = await import('./all.mjs');
  setFilter(FILTER_DEFAULT);
  const T = (s) => new Date(Date.UTC(2026, 9, 23, 12, 0, s)).toISOString();
  const x = (o) => ({ ext: true, dst: 2, levels: [1], prev: [[1, 0]], ok: true, srcName: 'CAM', dstName: 'MV', ...o });
  const rows = histRows([x({ id: 3, src: 3, time: T(20) }), x({ id: 2, src: 2, time: T(10), found: true }), x({ id: 1, src: 1, time: T(5), found: true })]);
  assert.deepEqual(rows.map((r) => `${r.e.id}${r.e.merged ? 'x' + r.e.merged : ''}`), ['3', '2x2']);
});

test('stacked sets combine', () => {
  const set = (o) => {
    const f = { src: '', dst: '', sode: true, ext: true, hidden: false, ...o };
    return { f, srcOk: filterFn(f.src), dstOk: filterFn(f.dst) };
  };
  const x = (o) => ({ ext: false, sn: 1, sname: 'CAM', dn: 5, dname: 'MV 5', hiddenDst: false, ...o });
  const mv = set({ dst: 'MV' }), noPip = set({ dst: '!PIP' }), extOnly = set({ sode: false });
  assert.equal(entryAllows(mv.f, mv, x()), true);
  // AND: every set must allow; OR: any may.
  const mon = set({ dst: 'MON' });
  assert.equal(stackAllows('and', [mv, mon], x()), false);
  assert.equal(stackAllows('or', [mv, mon], x()), true);
  assert.equal(stackAllows('or', [mon, set({ dst: 'PGM' })], x()), false);
  // An exclude counts only inside its own set, so OR still shows what
  // another set wants; AND hides it.
  const pip = x({ dname: 'PIP 1' });
  assert.equal(stackAllows('and', [noPip, set({ dst: 'PIP' })], pip), false);
  assert.equal(stackAllows('or', [noPip, set({ dst: 'PIP' })], pip), true);
  // Kinds are per set too.
  const ext = x({ ext: true });
  assert.equal(stackAllows('and', [extOnly, mv], x()), false);
  assert.equal(stackAllows('or', [extOnly, mv], x()), true);
  assert.equal(stackAllows('and', [extOnly, mv], ext), true);
  // Hidden destinations show only in a set that asks for them.
  const hid = x({ ext: true, hiddenDst: true });
  assert.equal(stackAllows('or', [mv, set({ hidden: true })], hid), true);
  assert.equal(stackAllows('and', [mv, set({ hidden: true })], hid), false);
});

test('stacking sets in the store', async () => {
  const core = await import('./all.mjs');
  const { S, setFilter, filterState, activeSet, activeStack, pickSet, toggleSet, allSets, normActive, setPref, filterAllows, FILTER_DEFAULT } = core;
  // An older browser kept one object; the list is the same with one item.
  assert.deepEqual(normActive({ scope: 'local', name: 'A', base: { dst: 'x' } }).map((r) => [r.scope, r.name, r.base.dst]), [['local', 'A', 'x']]);
  assert.deepEqual(normActive(null), []);
  assert.deepEqual(normActive([{ scope: 'global', name: 'B' }, 3, {}]).map((r) => r.name), ['B']);

  const f = (o) => ({ name: '', src: '', dst: '', sode: true, ext: true, hidden: false, ...o });
  S.cfg = { numberOffset: 1, filterSets: [f({ name: 'MV', dst: 'MV' }), f({ name: 'PGM', dst: 'PGM' }), f({ name: 'Ext', sode: false })] };
  const e = (o) => ({ levels: [1], prev: [], ok: true, src: 1, srcName: 'CAM', dst: 4, dstName: 'MV 5', ...o });
  const names = () => allSets().filter((s) => s.scope === 'global').map((s) => s.name);
  assert.deepEqual(names(), ['MV', 'PGM', 'Ext']);

  pickSet('all', 'All');
  toggleSet('global', 'MV');
  assert.equal(activeSet().name, 'MV');
  assert.equal(activeStack().length, 0);
  toggleSet('global', 'PGM');
  assert.deepEqual(activeStack().map((s) => s.name), ['MV', 'PGM']);
  assert.equal(activeSet(), null);
  assert.equal(filterState().dst, 'MV'); // the first one's
  assert.equal(JSON.parse(localStorage.getItem('sode-filter-active')).length, 2);
  setPref('setStack', 'and');
  assert.equal(filterAllows(e()), false);
  setPref('setStack', 'or');
  assert.equal(filterAllows(e()), true);
  assert.equal(filterAllows(e({ dstName: 'DSM 1' })), false);
  setPref('setStack', 'and');
  // Taking one off leaves the other, alone; the last off is All.
  toggleSet('global', 'MV');
  assert.deepEqual([activeSet().name, filterState().dst], ['PGM', 'PGM']);
  toggleSet('global', 'PGM');
  assert.equal(activeSet().scope, 'all');
  // Adding to a single edited set drops the edit.
  pickSet('global', 'MV');
  setFilter({ ...filterState(), dst: 'MV, X' });
  toggleSet('global', 'Ext');
  assert.equal(filterState().dst, 'MV');
  assert.equal(activeStack().length, 2);
  // Any edit leaves the stack, keeping what was typed.
  setFilter({ ...filterState(), src: 'CAM' });
  assert.deepEqual([activeStack().length, activeSet(), filterState().src], [0, null, 'CAM']);
  // Stack, then All.
  pickSet('global', 'MV');
  toggleSet('global', 'PGM');
  toggleSet('all', 'All');
  assert.deepEqual([activeStack().length, activeSet().scope, filterState()], [0, 'all', { ...FILTER_DEFAULT }]);
  // Sets removed in Settings drop out of a stack; none left is All.
  toggleSet('global', 'MV');
  toggleSet('global', 'PGM');
  S.cfg = { ...S.cfg, filterSets: S.cfg.filterSets.slice(2) };
  core.emit('helloData');
  assert.equal(activeSet().scope, 'all');
});
