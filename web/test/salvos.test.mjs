// Salvos: storage, typed references, and turning a salvo into a plan.
// Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const cases = JSON.parse(readFileSync(new URL('./salvo_cases.json', import.meta.url), 'utf8'));

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

test('normSalvo cleans numbers (shared cases)', async () => {
  const { normSalvo } = await setup();
  for (const c of cases.norm) assert.deepEqual(normSalvo(c.in), c.out);
});

// The cases the Go side can share: a salvo as sent, and whether it may be saved.
const build = (c) => {
  if (!c.gen) return c;
  const { items, perItem } = c.gen;
  return { name: c.name, items: Array.from({ length: items }, (_, i) => ({ src: i, dsts: Array.from({ length: perItem }, (_, j) => i * perItem + j), levels: [1] })) };
};

test('salvo validation matches the shared cases', async () => {
  const { normSalvo, salvoProblem, nameProblem } = await setup();
  for (const c of cases.salvos) {
    const s = normSalvo(build(c.salvo));
    const err = nameProblem(s.name, [], { noun: 'salvo' }) || salvoProblem(s);
    assert.equal(err === '', c.ok, `${c.name}: ${err || 'accepted'}`);
  }
});

test('salvo lists match the shared cases (count and 48 KiB)', async () => {
  const { salvosProblem } = await setup();
  for (const c of cases.lists) {
    const list = Array.from({ length: c.fill.salvos }, (_, k) => ({
      name: `s${k}`, items: Array.from({ length: c.fill.items }, (_, i) => ({ src: i, dsts: [i], levels: [1] })),
    }));
    assert.equal(salvosProblem(list) === '', c.ok, c.name);
  }
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
  const names = m.localSalvos().map((x) => x.name);
  assert.ok(m.nameProblem('evening', names, { noun: 'salvo' }));
  assert.equal(m.nameProblem('EVENING', names, { noun: 'salvo', old: 'Evening' }), '');
  assert.ok(m.nameProblem('  ', names));
  assert.ok(m.nameProblem('All', [], { reserved: 'all' }));
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
  assert.equal(plan.title, 'Salvo: Show');
  assert.equal(plan.salvo, 'Show'); // the bare name, sent with the batch
  assert.deepEqual(plan.items.map((i) => [i.dst.n, i.groups.map((g) => [g.src.n, g.levels])]), [
    [0, [[0, [1, 2]]]],
    [1, [[0, [1, 2]], [1, [3]]]],
  ]);
  assert.deepEqual(plan.warnings, [
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

test('a salvo plan can be confirmed: wording, summary and verb', async () => {
  const { salvoPlan, planVerb, planSummary, VERBS } = await setup();
  const plan = salvoPlan({ name: 'Show', items: [{ src: 0, dsts: [0, 1], levels: [1] }, { src: 99, dsts: [2], levels: [] }] }, [1, 2]);
  assert.equal(planVerb(plan), 'take');
  assert.equal(VERBS.salvo.resets, false);
  // Nodes are stand-ins here; what matters is that it builds and the head lines come first.
  const nodes = planSummary(plan);
  assert.ok(nodes.length >= 4); // title, warnings, question, the items
  assert.equal(plan.warnings.length, 1);
});

test('replaceSalvo adds, replaces by old name ignoring case, and leaves the rest', async () => {
  const { replaceSalvo } = await setup();
  const a = { name: 'A', items: [] }, b = { name: 'B', items: [] };
  assert.deepEqual(replaceSalvo([a], b).map((x) => x.name), ['A', 'B']);
  assert.deepEqual(replaceSalvo([a, b], { name: 'C', items: [] }, 'a').map((x) => x.name), ['C', 'B']);
  assert.deepEqual(replaceSalvo([a], { name: 'C', items: [] }, 'gone').map((x) => x.name), ['A', 'C']);
});

test('latestFolderTakes leaves out retries like the first find does', async () => {
  const { latestFolderTakes } = await setup();
  const hist = [ // newest first
    { id: 5, src: 1, dst: 5, levels: [1], retry: 3 },
    { id: 4, src: 2, dst: 7, levels: [3], group: 'g', groupN: 3 },
    { id: 3, src: 1, dst: 5, levels: [1], group: 'g', groupN: 3 },
    { id: 2, src: 1, dst: 6, levels: [1], group: 'g', groupN: 3, retry: 1 },
  ];
  assert.deepEqual(latestFolderTakes(hist).map((t) => t.dst), [5, 7]);
});

test('conflicts: different sources, one destination, overlapping levels', async () => {
  const { salvoProblem } = await setup();
  const s = (...items) => ({ name: 'x', items });
  const it = (src, dsts, levels = []) => ({ src, dsts, levels });
  assert.match(salvoProblem(s(it(0, [1]), it(2, [1], [1]))), /Destination 2 is taken from both source 1 and source 3/);
  assert.ok(salvoProblem(s(it(0, [1], [1, 2]), it(2, [1], [2, 3]))));
  assert.equal(salvoProblem(s(it(0, [1], [1, 2]), it(2, [1], [3, 4]))), '');
  assert.equal(salvoProblem(s(it(0, [1]), it(2, [3]))), '');
  assert.equal(salvoProblem(s(it(0, [1]), it(0, [1]))), '');
});
