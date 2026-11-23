import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };

const UID = '7f3c9a1e2b4d5f60';
const setup = async () => {
  const m = await import('./all.mjs');
  const it = (n, name) => ({ n, name, num: String(n + 1), label: `${n + 1} ${name}` });
  m.S.cfg = { numberOffset: 1, levelNumbering: '1', defaultLevels: '1', filterSets: [], presets: [] };
  m.S.routerUid = UID;
  m.S.levels = [{ ui: 1 }, { ui: 2 }];
  m.S.levelBy = new Map(m.S.levels.map((l) => [l.ui, l]));
  m.S.srcs = [it(0, 'CAM 1'), it(1, 'CAM 2')];
  m.S.dsts = [it(0, 'MON 1'), it(1, 'MON 2')];
  m.S.srcBy = new Map(m.S.srcs.map((x) => [x.n, x]));
  m.S.dstBy = new Map(m.S.dsts.map((x) => [x.n, x]));
  mem.clear();
  return m;
};

test('This browser presets store {router, n, name} and an id', async () => {
  const m = await setup();
  // 7 is not on the router: it keeps the name it came with.
  m.saveLocalPreset({ name: 'Show', items: [{ src: 1, dsts: [0, 7], levels: [], dstNames: { 7: 'OLD MON' } }] });
  const [s] = JSON.parse(mem.get('sode-presets'));
  assert.match(s.id, /^[0-9a-f]{16}$/);
  assert.deepEqual(s.items, [
    {
      src: { router: UID, n: 1, name: 'CAM 2' },
      dsts: [
        { router: UID, n: 0, name: 'MON 1' },
        { router: UID, n: 7, name: 'OLD MON' },
      ],
      levels: [],
    },
  ]);
});

test('a stored preset resolves to the same routes as its numbers', async () => {
  const m = await setup();
  const has = { src: (n) => n < 2, dst: (n) => n < 2, level: (u) => u <= 2 };
  const nums = { name: 'P', items: [{ src: 1, dsts: [0, 1], levels: [2], router: UID }] };
  const stored = {
    id: 'p1',
    name: 'P',
    items: [
      {
        src: { router: UID, n: 1, name: 'CAM 2' },
        dsts: [
          { router: UID, n: 0 },
          { router: UID, n: 1 },
        ],
        levels: [2],
      },
    ],
  };
  assert.deepEqual(m.resolvePreset(m.normPreset(stored), has, [1]), m.resolvePreset(m.normPreset(nums), has, [1]));
  assert.deepEqual(m.resolvePreset(m.normPreset(stored), has, [1]).routes, [
    { dst: 0, dstRouter: UID, groups: [{ src: 1, router: UID, levels: [2] }] },
    { dst: 1, dstRouter: UID, groups: [{ src: 1, router: UID, levels: [2] }] },
  ]);
});

// An item across routers (docs/tielines-design.md, Presets) keeps the router of its destinations and its tie lines.
// They go through the stored form and back, and resolvePreset hands them to the route.
test('an item across routers keeps its tie lines', async () => {
  const m = await setup();
  const B = 'b2b2b2b2b2b2b2b2';
  const stored = {
    id: 'p2',
    name: 'Far',
    items: [{ src: { router: UID, n: 1, name: 'CAM 2' }, dsts: [{ router: B, n: 3 }], levels: [1], ties: ['t1'] }],
  };
  const p = m.normPreset(stored);
  assert.deepEqual(p.items[0], { src: 1, dsts: [3], levels: [1], router: UID, srcName: 'CAM 2', dstNames: {}, dstRouter: B, ties: ['t1'] });
  assert.deepEqual(m.saved(p).items[0], stored.items[0]);
  assert.equal(m.presetProblem(p), '');
  assert.match(m.presetProblem({ items: [{ ...p.items[0], ties: [] }] }), /needs its tie lines/);
  assert.match(m.presetProblem({ items: [{ ...p.items[0], dstRouter: undefined }] }), /only for a route across routers/);
  const has = { src: () => true, dst: () => true, level: () => true, tie: (id) => id === 't1' };
  assert.deepEqual(m.resolvePreset(p, has, [1]).routes, [{ dst: 3, dstRouter: B, groups: [{ src: 1, router: UID, levels: [1], ties: ['t1'] }] }]);
  const gone = m.resolvePreset(p, { ...has, tie: () => false }, [1]);
  assert.deepEqual(gone, { routes: [], missing: [{ kind: 'tie', n: 't1' }] });
  // A panel reads the tie lines through itemPlain.
  const { itemPlain } = await import('../presetref.js');
  assert.deepEqual(itemPlain(stored.items[0]), { src: 1, dsts: [3], levels: [1], ties: ['t1'] });
});

test('a rename keeps the id, so a bookmark key still finds the preset', async () => {
  const m = await setup();
  m.saveLocalPreset({ name: 'Mine', items: [{ src: 0, dsts: [0], levels: [] }] });
  const mine = m.findPreset('local', 'Mine');
  m.saveLocalPreset({ ...mine, name: 'Renamed' }, 'Mine');
  assert.equal(m.findPreset('local', 'Renamed').id, mine.id);
  assert.equal('pinned' in m.saved(mine), false);
});

test('a Replace import into This browser keeps the replaced preset id', async () => {
  const m = await setup();
  m.saveLocalPreset({ name: 'Show', items: [{ src: 0, dsts: [0], levels: [] }] });
  m.saveLocalPreset({ name: 'Late', items: [{ src: 0, dsts: [1], levels: [] }] });
  const show = m.findPreset('local', 'Show').id;
  const late = m.findPreset('local', 'Late').id;
  // As reviewsheet.js writes it: the whole list, and the replaced entries have no id.
  m.putLocalPresets([
    { name: 'show', items: [{ src: 1, dsts: [1], levels: [] }] },
    { name: 'Late', items: [{ src: 0, dsts: [1], levels: [] }] },
    { id: late, name: 'Late 2', items: [{ src: 0, dsts: [0], levels: [] }] },
  ]);
  const got = m.localPresets();
  assert.equal(got[0].id, show);
  // The entry that names Late's id keeps it, so the other Late gets a new id.
  assert.notEqual(got[1].id, late);
  assert.match(got[1].id, /^[0-9a-f]{16}$/);
  assert.equal(got[2].id, late);
});

test('a sent list keeps ids; a new Everyone preset goes without one', async () => {
  const m = await setup();
  const list = [{ id: 'g1', name: 'A', items: [{ src: 0, dsts: [1], levels: [] }] }];
  const next = m.replacePreset(list, { name: 'B', items: [{ src: 1, dsts: [0], levels: [] }] });
  assert.deepEqual(
    next.map((s) => s.id),
    ['g1', ''],
  );
  // Replacing by name keeps the replaced entry's id.
  assert.equal(m.replacePreset(list, { name: 'A2', items: list[0].items }, 'a')[0].id, 'g1');
});

test('a stored preset reads no bare number as a reference', async () => {
  const m = await setup();
  const ref = { router: UID, n: 1, name: 'MON 2' };
  mem.set('sode-presets', JSON.stringify([{ id: 'a1', name: 'Old', items: [{ src: 1, dsts: [0, ref], levels: [] }] }]));
  const [s] = m.localPresets();
  assert.equal(s.items[0].src, -1, 'a bare source is no source');
  assert.deepEqual(s.items[0].dsts, [1], 'a bare destination is left out');
  assert.ok(m.presetProblem(s), 'the preset needs a fix; it fires nothing');
  m.S.cfg.presets = [{ id: 'g1', name: 'Wall', items: [{ src: { router: UID, n: 0 }, dsts: [2], levels: [] }] }];
  assert.deepEqual(m.globalPresets()[0].items[0].dsts, []);
});

// An editor row names a router by prefix; a bare number is on the router of the number before it.
test('readSide reads router prefixes in an editor row', async () => {
  const m = await setup();
  const routers = [
    { uid: 'a', prefix: 'A', off: 1 },
    { uid: 'b', prefix: 'B', off: 1 },
  ];
  const lookup = (t, uid) => (t === 'cam' ? (uid === 'b' ? 40 : 4) : null);
  assert.deepEqual(m.readSide('B.3, 5, A.2', { routers, at: 'a', lookup }), { router: 'b', ns: [2, 4, 1], bad: [], mixed: true });
  assert.deepEqual(m.readSide('B3, cam', { routers, at: 'a', lookup }), { router: 'b', ns: [2, 40], bad: [], mixed: false });
  assert.deepEqual(m.readSide('7', { routers, at: 'b', lookup }), { router: 'b', ns: [6], bad: [], mixed: false });
  assert.deepEqual(m.readSide('C.1, nope', { routers, at: 'a', lookup }), { router: 'a', ns: [], bad: ['C.1', 'nope'], mixed: false });
  // With one router a bare A12 is a name, as in the patch boxes.
  assert.deepEqual(m.readSide('A12', { routers: [routers[0]], at: 'a', lookup }).bad, ['A12']);
});
