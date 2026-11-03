// Salvos: storage, typed references, and turning a salvo into a plan.
// Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };

const setup = async () => {
  const m = await import('./all.mjs');
  const it = (n, name) => ({ n, name, num: String(n + 1), label: `${n + 1} ${name}` });
  m.S.cfg = { numberOffset: 1, levelNumbering: '1', defaultLevels: '1,2', filterSets: [], salvos: [] };
  m.S.levels = [{ ui: 1 }, { ui: 2 }, { ui: 3 }];
  m.S.levelBy = new Map(m.S.levels.map((l) => [l.ui, l]));
  m.S.srcs = [it(0, 'Cam 1'), it(1, 'Cam 2'), it(2, 'Cam 3')];
  m.S.dsts = [it(0, 'Rec 1'), it(1, 'Rec 2'), it(2, 'Rec 3')];
  m.S.srcBy = new Map(m.S.srcs.map((x) => [x.n, x]));
  m.S.dstBy = new Map(m.S.dsts.map((x) => [x.n, x]));
  mem.clear();
  return m;
};

test('parseRefs: display numbers, ranges, names, bad tokens', async () => {
  const { parseRefs } = await setup();
  const lookup = (t) => (t === 'cam' ? 7 : null);
  assert.deepEqual(parseRefs('1, 3-5, cam, nope, 3', 1, lookup), { ns: [0, 2, 3, 4, 7], bad: ['nope'] });
  assert.deepEqual(parseRefs('', 1, lookup), { ns: [], bad: [] });
  assert.deepEqual(parseRefs('9-5', 0, lookup).bad, ['9-5']);
});

test('normSalvo cleans numbers', async () => {
  const { normSalvo } = await setup();
  assert.deepEqual(normSalvo({ name: ' A ', items: [{ src: '2', dsts: [1, 1, 'x', -4, 3], levels: [3, 1, 3, 0] }] }),
    { name: 'A', items: [{ src: 2, dsts: [1, 3], levels: [1, 3] }] });
});

test('local salvos: add, replace, rename, delete, names ignoring case', async () => {
  const m = await setup();
  const a = { name: 'Morning', items: [{ src: 0, dsts: [1], levels: [] }] };
  m.saveLocalSalvo(a);
  m.saveLocalSalvo({ name: 'Evening', items: a.items });
  assert.deepEqual(m.localSalvos().map((s) => s.name), ['Morning', 'Evening']);
  m.saveLocalSalvo({ name: 'Morn', items: a.items }, 'morning');
  assert.deepEqual(m.localSalvos().map((s) => s.name), ['Morn', 'Evening']);
  assert.ok(m.findSalvo('local', 'EVENING'));
  assert.ok(m.nameProblem(m.localSalvos(), 'evening'));
  assert.equal(m.nameProblem(m.localSalvos(), 'EVENING', 'Evening'), '');
  assert.ok(m.nameProblem(m.localSalvos(), '  '));
  m.deleteLocalSalvo('morn');
  assert.deepEqual(m.localSalvos().map((s) => s.name), ['Evening']);
});

test('global salvos come from the config, ahead of local ones', async () => {
  const m = await setup();
  m.S.cfg.salvos = [{ name: 'Show', items: [{ src: 1, dsts: [0, 2], levels: [] }] }];
  m.saveLocalSalvo({ name: 'Mine', items: [{ src: 0, dsts: [0], levels: [] }] });
  assert.deepEqual(m.allSalvos().map((s) => `${s.scope}:${s.name}`), ['global:Show', 'local:Mine']);
});

test('salvoProblem', async () => {
  const { salvoProblem } = await setup();
  assert.ok(salvoProblem({ name: 'a', items: [] }));
  assert.ok(salvoProblem({ name: 'a', items: [{ src: -1, dsts: [1], levels: [] }] }));
  assert.ok(salvoProblem({ name: 'a', items: [{ src: 1, dsts: [], levels: [] }] }));
  assert.equal(salvoProblem({ name: 'a', items: [{ src: 1, dsts: [1], levels: [] }] }), '');
});

test('salvoPlan: one item per destination, default levels, unresolved left out and said', async () => {
  const { salvoPlan } = await setup();
  const plan = salvoPlan({ name: 'Show', items: [
    { src: 0, dsts: [0, 1], levels: [1, 2] },
    { src: 1, dsts: [1], levels: [3] },
    { src: 99, dsts: [2], levels: [] },
    { src: 2, dsts: [77], levels: [1, 9] },
  ] }, [1, 2]);
  assert.equal(plan.verb, 'salvo');
  assert.equal(plan.salvo, 'Show');
  assert.deepEqual(plan.items.map((i) => [i.dst.n, i.groups.map((g) => [g.src.n, g.levels])]), [
    [0, [[0, [1, 2]]]],
    [1, [[0, [1, 2]], [1, [3]]]],
  ]);
  assert.deepEqual(plan.missing, [
    'Source 100 does not exist on the router now',
    'Level 9 does not exist on the router now',
    'Destination 78 does not exist on the router now',
  ]);
});

test('salvoPlan: the same source twice on one destination merges levels', async () => {
  const { salvoPlan } = await setup();
  const plan = salvoPlan({ name: 'x', items: [{ src: 0, dsts: [0], levels: [1] }, { src: 0, dsts: [0], levels: [3] }] }, [1, 2]);
  assert.deepEqual(plan.items[0].groups.map((g) => g.levels), [[1, 3]]);
});

test('itemsFromTakes and latestFolderTakes', async () => {
  const { itemsFromTakes, latestFolderTakes } = await setup();
  const takes = [
    { src: 1, dst: 5, levels: [1, 2] }, { src: 1, dst: 6, levels: [1, 2] }, { src: 2, dst: 7, levels: [3] },
  ];
  assert.deepEqual(itemsFromTakes(takes), [
    { src: 1, dsts: [5, 6], levels: [1, 2] }, { src: 2, dsts: [7], levels: [3] },
  ]);
  const hist = [ // newest first
    { id: 4, src: 2, dst: 7, levels: [3], group: 'g', groupN: 2 },
    { id: 3, src: 1, dst: 5, levels: [1], group: 'g', groupN: 2 },
    { id: 2, src: 9, dst: 9, levels: [1], ext: true },
  ];
  assert.deepEqual(latestFolderTakes(hist).map((t) => t.dst), [5, 7]);
  assert.deepEqual(latestFolderTakes([{ id: 2, src: 9, dst: 9, levels: [1], ext: true }]), []);
});

test('conflicts: different sources, one destination, overlapping levels', async () => {
  const { salvoProblem, salvoPlan } = await setup();
  const s = (...items) => ({ name: "x", items });
  const it = (src, dsts, levels = []) => ({ src, dsts, levels });
  const bad = s(it(0, [1]), it(2, [1], [1]));
  assert.match(salvoProblem(bad), /Destination 2 is taken from both source 1 and source 3/);
  assert.ok(salvoProblem(s(it(0, [1], [1, 2]), it(2, [1], [2, 3]))));
  // Disjoint levels, other destinations and the same source are fine.
  assert.equal(salvoProblem(s(it(0, [1], [1, 2]), it(2, [1], [3, 4]))), "");
  assert.equal(salvoProblem(s(it(0, [1]), it(2, [3]))), "");
  assert.equal(salvoProblem(s(it(0, [1]), it(0, [1]))), "");
  // An old salvo saved before the check: the plan carries the error and no takes.
  const p = salvoPlan(bad, [1, 2]);
  assert.ok(p.error);
  assert.equal(p.items.length, 0);
  assert.equal(salvoPlan(s(it(0, [1], [1, 2]), it(2, [1], [3])), [1, 2]).error, "");
});

test('total destinations are capped at 200', async () => {
  const { salvoProblem, salvoPlan } = await setup();
  const dsts = (a, n) => Array.from({ length: n }, (_, i) => a + i);
  const ok = { name: "x", items: [{ src: 0, dsts: dsts(0, 100), levels: [] }, { src: 0, dsts: dsts(100, 100), levels: [] }] };
  assert.equal(salvoProblem(ok), "");
  const big = { name: "x", items: [...ok.items, { src: 0, dsts: [500], levels: [] }] };
  assert.match(salvoProblem(big), /at most 200 destinations in total/);
  assert.ok(salvoPlan(big, [1]).error);
});
