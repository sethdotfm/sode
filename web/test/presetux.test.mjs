import test from 'node:test';
import assert from 'node:assert/strict';

const fake = () => {
  const n = {
    kids: [],
    className: '',
    setAttribute() {},
    addEventListener() {},
    classList: { toggle() {}, add() {}, remove() {} },
    style: {},
    dataset: {},
    append(...k) {
      this.kids.push(...k);
    },
    set textContent(v) {
      this.kids = [String(v)];
    },
  };
  Object.defineProperty(n, 'text', {
    get() {
      return this.kids.map((k) => (typeof k === 'string' ? k : (k.text ?? ''))).join('');
    },
  });
  return n;
};
globalThis.document = {
  createElement: fake,
  createElementNS: fake,
  addEventListener() {},
  querySelector: () => null,
  body: fake(),
  addEventListener() {},
  querySelectorAll: () => [],
  documentElement: { dataset: {} },
};
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };

const setup = async () => {
  const m = await import('./all.mjs');
  const it = (n, name) => ({ n, name, num: String(n + 1), label: `${n + 1} ${name}` });
  m.S.cfg = { numberOffset: 1, levelNumbering: '1', defaultLevels: '1,2', filterSets: [], presets: [] };
  m.S.levels = [{ ui: 1 }, { ui: 2 }];
  m.S.levelBy = new Map(m.S.levels.map((l) => [l.ui, l]));
  m.S.srcs = [it(0, 'Cam 1'), it(1, 'Cam 2')];
  m.S.dsts = [it(0, 'Rec 1'), it(1, 'Rec 2'), it(2, 'Rec 3')];
  m.S.srcBy = new Map(m.S.srcs.map((x) => [x.n, x]));
  m.S.dstBy = new Map(m.S.dsts.map((x) => [x.n, x]));
  mem.clear();
  return m;
};
const sv = (name, pinned) => ({ id: name.toLowerCase(), name, items: [{ src: 0, dsts: [1], levels: [] }], ...(pinned === undefined ? {} : { pinned }) });
const names = (l) => l.map((s) => s.name).join(',');

test('presets carry no pinned flag: an old one is dropped on save', async () => {
  const m = await setup();
  mem.set('sode-presets', JSON.stringify([sv('A'), sv('B', false)]));
  m.S.cfg.presets = [sv('G')];
  assert.equal(names(m.allPresets()), 'G,A,B');
  const next = m.replacePreset(m.localPresets(), m.normPreset(sv('C')));
  assert.equal(
    next.some((s) => 'pinned' in s),
    false,
  );
  assert.equal(m.presetsProblem(next), '');
});

test('popup search: all when empty, best name first when typed', async () => {
  const m = await setup();
  const l = ['Morning show', 'Evening', 'Late morning', 'Mono'].map((name) => ({ scope: 'global', name, items: [] }));
  assert.equal(m.rankPresets(l, '  ').length, 4);
  assert.deepEqual(
    m.rankPresets(l, 'morning').map((s) => s.name),
    ['Morning show', 'Late morning'],
  );
  assert.deepEqual(m.rankPresets(l, 'zzz'), []);
  assert.equal(m.rankPresets(l, 'mng')[0].name, 'Morning show');
});

test('a Broken part is left out though its number is still shown', async () => {
  const m = await setup();
  const s = { ...sv('Studio'), broken: [{ kind: 'source', ref: { router: 'r', n: 0, name: 'Cam 1' } }] };
  s.items.push({ src: 1, dsts: [2], levels: [] });
  const plan = m.presetPlan(m.normPreset(s), [1]);
  assert.deepEqual(
    plan.items.map((it) => [it.groups[0].src.n, it.dst.n]),
    [[1, 2]],
  );
  assert.equal(plan.warnings.length, 0, 'the Broken banner names it, not the confirm');
  assert.equal(m.presetPlan(m.normPreset(sv('Studio')), [1]).items.length, 1);
});

test('preset trigger: plan carries the name, history shows it, search finds it', async () => {
  const m = await setup();
  const plan = m.presetPlan(m.normPreset(sv('Morning')), [1]);
  assert.equal(plan.preset, 'Morning');
  const text = (n) => (n ? n.text : '');
  const e = (o) => ({ levels: [1], prev: [], ok: true, time: '2026-10-23T14:00:00Z', ...o });
  assert.match(text(m.entryNote(e({ id: 1, src: 0, dst: 1, preset: 'Morning' }))), /Preset: Morning/);
  assert.equal(text(m.entryNote(e({ id: 1, src: 0, dst: 1, group: 'g', groupN: 2, preset: 'Morning' }))), '');
  assert.equal(text(m.entryNote(e({ id: 4, src: 0, dst: 1 }))), '');
  const f = {
    entries: [e({ id: 2, src: 0, dst: 1, group: 'g', groupN: 2, preset: 'Morning' }), e({ id: 3, src: 0, dst: 2, group: 'g', groupN: 2, preset: 'Morning' })],
  };
  f.shown = f.entries;
  assert.ok(m.rowView({ f }).route(f).notes.includes('Preset: Morning'));
  const hist = [
    e({ id: 5, src: 0, dst: 1, srcName: 'Cam 1', dstName: 'Rec 2', preset: 'Morning' }),
    e({ id: 6, src: 1, dst: 2, srcName: 'Cam 2', dstName: 'Rec 3' }),
  ];
  m.S.hist = hist;
  m.setSearch({ src: 'morning', dst: '' });
  assert.deepEqual(
    m.histRows(hist).map((r) => r.e.id),
    [5],
  );
  m.setSearch({ src: '', dst: '' });
});
