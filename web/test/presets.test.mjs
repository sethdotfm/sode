import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const cases = JSON.parse(readFileSync(new URL('./preset_cases.json', import.meta.url), 'utf8'));

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };

const setup = async () => {
  const m = await import('./all.mjs');
  const it = (n, name) => ({ n, name, num: String(n + 1), label: `${n + 1} ${name}` });
  m.S.cfg = { filterSets: [], presets: [] };
  Object.assign(m.routerData(''), { off: 1, numbering: '1', defaultLevels: '1,2' });
  m.routerData('').uid = '0123456789abcdef';
  m.routerData('').levels = [{ ui: 1 }, { ui: 2 }, { ui: 3 }];
  m.routerData('').levelBy = new Map(m.routerData('').levels.map((l) => [l.ui, l]));
  m.routerData('').srcs = [it(0, 'Cam 1'), it(1, 'Cam 2'), it(2, 'Cam 3')];
  m.routerData('').dsts = [it(0, 'Rec 1'), it(1, 'Rec 2'), it(2, 'Rec 3')];
  m.routerData('').srcBy = new Map(m.routerData('').srcs.map((x) => [x.n, x]));
  m.routerData('').dstBy = new Map(m.routerData('').dsts.map((x) => [x.n, x]));
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

test('normPreset cleans numbers (shared cases)', async () => {
  const { normPreset } = await setup();
  for (const c of cases.norm) assert.deepEqual(normPreset(c.in), c.out);
});

// preset_cases.json: the Go tests read the same cases.
const build = (c) => {
  if (!c.gen) return c;
  const { items, perItem } = c.gen;
  return {
    name: c.name,
    items: Array.from({ length: items }, (_, i) => ({ src: i, dsts: Array.from({ length: perItem }, (_, j) => i * perItem + j), levels: [1] })),
  };
};

test('preset validation matches the shared cases', async () => {
  const { normPreset, presetProblem, nameProblem } = await setup();
  for (const c of cases.presets) {
    const s = normPreset(build(c.preset));
    const err = nameProblem(s.name, [], { noun: 'preset' }) || presetProblem(s);
    assert.equal(err === '', c.ok, `${c.name}: ${err || 'accepted'}`);
  }
});

test('preset lists match the shared cases (count and 48 KiB)', async () => {
  const { presetsProblem } = await setup();
  for (const c of cases.lists) {
    const list = Array.from({ length: c.fill.presets }, (_, k) => ({
      name: `s${k}`,
      items: Array.from({ length: c.fill.items }, (_, i) => ({ src: i, dsts: [i], levels: [1] })),
    }));
    assert.equal(presetsProblem(list) === '', c.ok, c.name);
  }
});

test('local presets: add, replace, rename, delete, names ignoring case', async () => {
  const m = await setup();
  const a = { name: 'Morning', items: [{ src: 0, dsts: [1], levels: [] }] };
  m.saveLocalPreset(a);
  m.saveLocalPreset({ name: 'Evening', items: a.items });
  assert.deepEqual(
    m.localPresets().map((s) => s.name),
    ['Morning', 'Evening'],
  );
  m.saveLocalPreset({ name: 'Morn', items: a.items }, 'morning');
  assert.deepEqual(
    m.localPresets().map((s) => s.name),
    ['Morn', 'Evening'],
  );
  assert.ok(m.findPreset('local', 'EVENING'));
  const names = m.localPresets().map((x) => x.name);
  assert.ok(m.nameProblem('evening', names, { noun: 'preset' }));
  assert.equal(m.nameProblem('EVENING', names, { noun: 'preset', old: 'Evening' }), '');
  assert.ok(m.nameProblem('  ', names));
  assert.ok(m.nameProblem('All', [], { reserved: 'all' }));
  m.deleteLocalPreset('morn');
  assert.deepEqual(
    m.localPresets().map((s) => s.name),
    ['Evening'],
  );
});

test('global presets come from the config, ahead of local ones', async () => {
  const m = await setup();
  m.S.cfg.presets = [{ name: 'Show', items: [{ src: 1, dsts: [0, 2], levels: [] }] }];
  m.saveLocalPreset({ name: 'Mine', items: [{ src: 0, dsts: [0], levels: [] }] });
  assert.deepEqual(
    m.allPresets().map((s) => `${s.scope}:${s.name}`),
    ['global:Show', 'local:Mine'],
  );
});

test('presetProblem', async () => {
  const { presetProblem } = await setup();
  assert.ok(presetProblem({ name: 'a', items: [] }));
  assert.ok(presetProblem({ name: 'a', items: [{ src: -1, dsts: [1], levels: [] }] }));
  assert.ok(presetProblem({ name: 'a', items: [{ src: 1, dsts: [], levels: [] }] }));
  assert.equal(presetProblem({ name: 'a', items: [{ src: 1, dsts: [1], levels: [] }] }), '');
});

test('presetPlan: one item per destination, default levels, unresolved left out and said', async () => {
  const { presetPlan } = await setup();
  const plan = presetPlan(
    {
      name: 'Show',
      items: [
        { src: 0, dsts: [0, 1], levels: [1, 2] },
        { src: 1, dsts: [1], levels: [3] },
        { src: 99, dsts: [2], levels: [] },
        { src: 2, dsts: [77], levels: [1, 9] },
      ],
    },
    [1, 2],
  );
  assert.equal(plan.verb, 'preset');
  assert.equal(plan.title, 'Preset: Show');
  assert.equal(plan.preset, 'Show');
  assert.deepEqual(
    plan.items.map((i) => [i.dst.n, i.groups.map((g) => [g.src.n, g.levels])]),
    [
      [0, [[0, [1, 2]]]],
      [
        1,
        [
          [0, [1, 2]],
          [1, [3]],
        ],
      ],
    ],
  );
  assert.deepEqual(plan.warnings, ['Source 100: not on the router', 'Level 9: not on the router', 'Destination 78: not on the router']);
});

test('presetPlan: the same source twice on one destination merges levels', async () => {
  const { presetPlan } = await setup();
  const plan = presetPlan(
    {
      name: 'x',
      items: [
        { src: 0, dsts: [0], levels: [1] },
        { src: 0, dsts: [0], levels: [3] },
      ],
    },
    [1, 2],
  );
  assert.deepEqual(
    plan.items[0].groups.map((g) => g.levels),
    [[1, 3]],
  );
});

test('itemsFromEntries', async () => {
  const { itemsFromEntries } = await setup();
  const takes = [
    { src: 1, dst: 5, levels: [1, 2] },
    { src: 1, dst: 6, levels: [1, 2] },
    { src: 2, dst: 7, levels: [3] },
  ];
  assert.deepEqual(itemsFromEntries(takes), [
    { src: 1, dsts: [5, 6], levels: [1, 2] },
    { src: 2, dsts: [7], levels: [3] },
  ]);
});

test('a preset plan can be confirmed: wording, summary and verb', async () => {
  const { presetPlan, planVerb, planSummary, VERBS } = await setup();
  const plan = presetPlan(
    {
      name: 'Show',
      items: [
        { src: 0, dsts: [0, 1], levels: [1] },
        { src: 99, dsts: [2], levels: [] },
      ],
    },
    [1, 2],
  );
  assert.equal(planVerb(plan), 'take');
  assert.equal(VERBS.preset.resets, false);
  // Nodes are stand-ins here; what matters is that it builds and the head lines come first.
  const nodes = planSummary(plan);
  assert.ok(nodes.length >= 4);
  assert.equal(plan.warnings.length, 1);
});

test('replacePreset adds, replaces by old name ignoring case, and leaves the rest', async () => {
  const { replacePreset } = await setup();
  const a = { name: 'A', items: [] },
    b = { name: 'B', items: [] };
  assert.deepEqual(
    replacePreset([a], b).map((x) => x.name),
    ['A', 'B'],
  );
  assert.deepEqual(
    replacePreset([a, b], { name: 'C', items: [] }, 'a').map((x) => x.name),
    ['C', 'B'],
  );
  assert.deepEqual(
    replacePreset([a], { name: 'C', items: [] }, 'gone').map((x) => x.name),
    ['A', 'C'],
  );
});

test('conflicts: different sources, one destination, overlapping levels', async () => {
  const { presetProblem } = await setup();
  const s = (...items) => ({ name: 'x', items });
  const it = (src, dsts, levels = []) => ({ src, dsts, levels });
  assert.match(presetProblem(s(it(0, [1]), it(2, [1], [1]))), /Destination 2 gets both source 1 and source 3/);
  assert.ok(presetProblem(s(it(0, [1], [1, 2]), it(2, [1], [2, 3]))));
  assert.equal(presetProblem(s(it(0, [1], [1, 2]), it(2, [1], [3, 4]))), '');
  assert.equal(presetProblem(s(it(0, [1]), it(2, [3]))), '');
  assert.equal(presetProblem(s(it(0, [1]), it(0, [1]))), '');
});

test('presetTip: one or many sources and destinations', async () => {
  const { presetTip } = await setup();
  const s = (items) => ({ items });
  const sl = (n) => `S${n}`,
    dl = (n) => `D${n}`;
  assert.equal(presetTip(s([{ src: 1, dsts: [2], levels: [] }]), sl, dl), 'S1 to D2');
  assert.equal(presetTip(s([{ src: 1, dsts: [2, 3, 4], levels: [] }]), sl, dl), 'S1 to 3 Destinations');
  assert.equal(
    presetTip(
      s([
        { src: 1, dsts: [2], levels: [] },
        { src: 1, dsts: [2, 5], levels: [] },
      ]),
      sl,
      dl,
    ),
    'S1 to 2 Destinations',
  );
  assert.equal(
    presetTip(
      s([
        { src: 1, dsts: [2], levels: [] },
        { src: 4, dsts: [3], levels: [] },
      ]),
      sl,
      dl,
    ),
    '2 Sources to 2 Destinations',
  );
  assert.equal(presetTip(s([]), sl, dl), 'No rows');
});

test('isBlankRow: only a row with nothing typed', async () => {
  const { isBlankRow } = await setup();
  assert.equal(isBlankRow({ src: '', dsts: ' ', levels: '' }), true);
  assert.equal(isBlankRow({ src: '1', dsts: '', levels: '' }), false);
  assert.equal(isBlankRow({ src: '', dsts: '', levels: '2' }), false);
});

test('fillFrom: the first take that still exists, with what was left out', async () => {
  const { fillFrom } = await setup();
  const has = { src: (n) => n < 3, dst: (n) => n < 3, level: (u) => u <= 3 };
  const p = (...items) => ({ name: 'P', items });
  // Whole: one route, nothing missing; its own [] levels stay [].
  let r = fillFrom(p({ src: 0, dsts: [1, 2], levels: [] }), has, [1, 2]);
  assert.deepEqual(r, { fill: { src: 0, dsts: [1, 2], levels: [] }, partial: false, many: false, missing: [] });
  // A destination gone: filled without it, and partial.
  r = fillFrom(p({ src: 0, dsts: [1, 9], levels: [2] }), has, [1, 2]);
  assert.deepEqual([r.fill, r.partial], [{ src: 0, dsts: [1], levels: [2] }, true]);
  // Every level of the first route gone: it is skipped, not filled with defaults.
  r = fillFrom(p({ src: 0, dsts: [1], levels: [5] }, { src: 1, dsts: [2], levels: [1] }), has, [1, 2]);
  assert.deepEqual([r.fill, r.many], [{ src: 1, dsts: [2], levels: [1] }, true]);
  // Nothing fits: no route, and what is missing is listed.
  r = fillFrom(p({ src: 9, dsts: [1], levels: [] }, { src: 0, dsts: [1], levels: [7] }), has, [1, 2]);
  assert.equal(r.fill, null);
  assert.deepEqual(r.missing.map((m) => m.kind).sort(), ['level', 'source']);
});
