// Preset UX: pinning, the popup's search order, and the history trigger.
import test from 'node:test';
import assert from 'node:assert/strict';

const fake = () => {
  const n = { kids: [], className: '', setAttribute() {}, addEventListener() {}, classList: { toggle() {}, add() {}, remove() {} }, style: {}, dataset: {}, append(...k) { this.kids.push(...k); }, set textContent(v) { this.kids = [String(v)]; } };
  Object.defineProperty(n, 'text', { get() { return this.kids.map((k) => (typeof k === 'string' ? k : k.text ?? '')).join(''); } });
  return n;
};
globalThis.document = { createElement: fake, createElementNS: fake, addEventListener() {}, querySelector: () => null, body: fake(), addEventListener() {}, querySelectorAll: () => [], documentElement: { dataset: {} } };
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
const sv = (name, pinned) => ({ name, items: [{ src: 0, dsts: [1], levels: [] }], ...(pinned === undefined ? {} : { pinned }) });
const names = (l) => l.map((s) => s.name).join(',');

test('pinning: global flag, local override, unknown names, default', async () => {
  const m = await setup();
  m.S.cfg.presets = [sv('Morning'), sv('Evening', false), sv('Late', true)];
  assert.equal(names(m.pinnedPresets()), 'Morning,Late');
  const g = (n) => m.globalPresets().find((s) => s.name === n);
  m.setLocalPin(g('Evening'), true);
  m.setLocalPin(g('Morning'), false);
  assert.equal(names(m.pinnedPresets()), 'Evening,Late');
  assert.deepEqual(JSON.parse(mem.get('sode-preset-pins')), { evening: true, morning: false });
  // An override equal to the flag is dropped again.
  m.setLocalPin(g('Morning'), true);
  assert.deepEqual(JSON.parse(mem.get('sode-preset-pins')), { evening: true });
  // Unknown names, junk values and a non-object are ignored.
  mem.set('sode-preset-pins', JSON.stringify({ evening: 'yes', gone: false, late: false }));
  assert.deepEqual(m.pinOverrides(), { late: false });
  assert.equal(names(m.pinnedPresets()), 'Morning');
  mem.set('sode-preset-pins', '[1]');
  assert.deepEqual(m.pinOverrides(), {});
  mem.set('sode-preset-pins', 'not json');
  assert.equal(names(m.pinnedPresets()), 'Morning,Late');
  // Pin for everyone changes the flag in the list sent; this browser then follows it.
  assert.equal(m.withGlobalPin('Evening', true).find((s) => s.name === 'Evening').pinned, true);
  mem.set('sode-preset-pins', JSON.stringify({ evening: false }));
  m.clearPinOverride('Evening');
  assert.equal(mem.has('sode-preset-pins'), false);
});

test('pinning: own presets carry their flag; the strip filters', async () => {
  const m = await setup();
  mem.set('sode-presets', JSON.stringify([sv('A'), sv('B', false)]));
  m.S.cfg.presets = [sv('G')];
  assert.deepEqual(m.localPresets().map((s) => s.pinned), [true, false]);
  assert.equal(names(m.pinnedPresets()), 'G,A');
  m.setLocalPin(m.localPresets()[1], true);
  assert.equal(names(m.pinnedPresets()), 'G,A,B');
  const next = m.replacePreset(m.localPresets(), { ...m.normPreset(sv('C')), pinned: false });
  assert.equal(next[2].pinned, false);
  assert.equal(m.presetsProblem(next), '');
});

test('popup search: all when empty, best name first when typed', async () => {
  const m = await setup();
  const l = ['Morning show', 'Evening', 'Late morning', 'Mono'].map((name) => ({ scope: 'global', name, items: [], pinned: true }));
  assert.equal(m.rankPresets(l, '  ').length, 4);
  assert.deepEqual(m.rankPresets(l, 'morning').map((s) => s.name), ['Morning show', 'Late morning']);
  assert.deepEqual(m.rankPresets(l, 'zzz'), []);
  assert.equal(m.rankPresets(l, 'mng')[0].name, 'Morning show');
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
  const f = { entries: [e({ id: 2, src: 0, dst: 1, group: 'g', groupN: 2, preset: 'Morning' }), e({ id: 3, src: 0, dst: 2, group: 'g', groupN: 2, preset: 'Morning' })] };
  f.shown = f.entries;
  const title = m.folderTitle(f).map((n) => (typeof n === 'string' ? n : text(n))).join('');
  assert.match(title, /Preset: Morning/);
  // Quick search by the preset name.
  const hist = [e({ id: 5, src: 0, dst: 1, srcName: 'Cam 1', dstName: 'Rec 2', preset: 'Morning' }), e({ id: 6, src: 1, dst: 2, srcName: 'Cam 2', dstName: 'Rec 3' })];
  m.S.hist = hist;
  m.setSearch({ src: 'morning', dst: '' });
  assert.deepEqual(m.histRows(hist).map((r) => r.e.id), [5]);
  m.setSearch({ src: '', dst: '' });
});
