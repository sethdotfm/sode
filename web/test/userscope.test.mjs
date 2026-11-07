// The 'user' (Me) scope of filter sets and presets: from the profile message, carry their user, never leak to another user.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = new Proxy({ baseURI: 'http://sode.test/' }, { get: (t, k) => (k in t ? t[k] : node()) });
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };
const si = globalThis.setInterval;
globalThis.setInterval = (...a) => { const t = si(...a); t.unref?.(); return t; };
const sent = [];
class FakeWS {
  static OPEN = 1;
  constructor() { this.readyState = 1; this.lastRx = Date.now(); FakeWS.last = this; }
  send(m) { sent.push(JSON.parse(m)); }
  close() {}
}
globalThis.WebSocket = FakeWS;

const m = await import('./all.mjs');
const { profileSaver, profileProblem, PROFILE_MAX_BYTES } = await import('../globalsave.js');
m.connect();
const deliver = (msg) => FakeWS.last.onmessage({ data: JSON.stringify(msg) });
const f = (o) => ({ src: '', dst: '', sode: true, ext: true, hidden: false, ...o });
const login = (name, filterSets = [], presets = []) => deliver({ t: 'profile', name, prefs: {}, filterSets, presets });
const reset = () => {
  mem.clear();
  m.S.cfg = { numberOffset: 1, levelNumbering: '1', defaultLevels: '1', filterSets: [], presets: [] };
  login('');
  m.pickSet('all', 'All');
};

test('user sets come from the profile and sit between Everyone and This browser', () => {
  reset();
  m.S.cfg.filterSets = [f({ name: 'Shared', dst: 'MV' })];
  m.saveLocalSet('Mine');
  assert.deepEqual(m.allSets().map((s) => `${s.scope}:${s.name}`), ['all:All', 'global:Shared', 'local:Mine']);
  login('Ana', [f({ name: 'Ana set', src: 'CAM' })]);
  assert.deepEqual(m.allSets().map((s) => `${s.scope}:${s.name}`), ['all:All', 'global:Shared', 'user:Ana set', 'local:Mine']);
  login('');
  assert.deepEqual(m.userSets(), []);
});

test('a name used in two scopes is two sets; picking goes by scope', () => {
  reset();
  m.S.cfg.filterSets = [f({ name: 'Cams', src: 'GLOBAL' })];
  login('Ana', [f({ name: 'cams', src: 'ANA' })]);
  m.pickSet('user', 'Cams');
  assert.equal(m.filterState().src, 'ANA');
  assert.equal(m.activeRef().user, 'Ana');
  assert.equal(m.isActive('user', 'Cams'), true);
  assert.equal(m.isActive('global', 'Cams'), false);
  m.pickSet('global', 'Cams');
  assert.equal(m.filterState().src, 'GLOBAL');
});

test("the active set carries its user: Bob never picks up Ana's same-named set", () => {
  reset();
  login('Ana', [f({ name: 'Mine', src: 'ANA' })]);
  m.pickSet('user', 'Mine');
  assert.equal(m.activeSet().name, 'Mine');
  assert.equal(JSON.parse(mem.get('sode-filter-active')).user, 'Ana');
  login('Bob', [f({ name: 'Mine', src: 'BOB' })]);
  assert.equal(m.activeSet().scope, 'all');
  assert.equal(m.filterState().src, '');
});

test('logging out drops a user set', () => {
  reset();
  login('Ana', [f({ name: 'Mine', src: 'ANA' })]);
  m.pickSet('user', 'Mine');
  login('');
  assert.equal(m.activeSet().scope, 'all');
  assert.equal(m.filterState().src, '');
  assert.equal(m.allSets().some((s) => s.scope === 'user'), false);
});

test('a stack drops the sets of a user that left, keeps the rest', () => {
  reset();
  m.S.cfg.filterSets = [f({ name: 'G', dst: 'MV' })];
  login('Ana', [f({ name: 'A', src: 'ANA' })]);
  m.toggleSet('global', 'G');
  m.toggleSet('user', 'A');
  assert.equal(m.activeStack().length, 2);
  assert.deepEqual(JSON.parse(mem.get('sode-filter-stack')).map((r) => r.user), [undefined, 'Ana']);
  login('Bob', []);
  assert.equal(m.activeStack().length, 0);
  assert.equal(m.activeSet().name, 'G');
});

test('a set edited on another device is followed unless this browser edited it', () => {
  reset();
  login('Ana', [f({ name: 'Mine', src: 'ONE' })]);
  m.pickSet('user', 'Mine');
  login('Ana', [f({ name: 'Mine', src: 'TWO' })]);
  assert.equal(m.filterState().src, 'TWO');
  m.setFilter(f({ src: 'HAND' }));
  login('Ana', [f({ name: 'Mine', src: 'THREE' })]);
  assert.equal(m.filterState().src, 'HAND');
  assert.equal(m.activeSet().edited, true);
});

test('user presets: own pinned flag, scoped after Everyone', () => {
  reset();
  m.S.cfg.presets = [{ name: 'Show', items: [{ src: 1, dsts: [0], levels: [] }] }];
  login('Ana', [], [{ name: 'Show', pinned: false, items: [{ src: 2, dsts: [1], levels: [] }] }]);
  assert.deepEqual(m.allPresets().map((s) => `${s.scope}:${s.name}`), ['global:Show', 'user:Show']);
  assert.equal(m.findPreset('user', 'show').items[0].src, 2);
  // a browser override of the Everyone preset never touches the Me preset of the same name
  m.setLocalPin(m.findPreset('global', 'Show'), false);
  assert.deepEqual(m.pinnedPresets().map((s) => s.scope), []);
  m.setLocalPin(m.findPreset('global', 'Show'), true);
  assert.deepEqual(m.pinnedPresets().map((s) => s.scope), ['global']);
  assert.equal(m.effectivePin(m.findPreset('user', 'Show')), false);
  m.setLocalPin(m.findPreset('user', 'Show'), true); // a Me preset is saved to the profile instead: no local effect
  assert.equal(m.effectivePin(m.findPreset('user', 'Show')), false);
  const next = m.withUserPin('Show', true);
  assert.equal(next.length, 1);
  assert.equal(next[0].pinned, true);
  assert.equal('scope' in next[0], false);
  login('');
  assert.deepEqual(m.userPresets(), []);
});

test('profileProblem refuses an oversized profile; saves send the whole list', async () => {
  reset();
  assert.equal(profileProblem({ prefs: {}, filterSets: [], presets: [] }, { filterSets: [f({ name: 'a' })] }), '');
  const big = Array.from({ length: 400 }, (_, i) => ({ name: `s${i}`, src: 'x'.repeat(200), dst: '', sode: true, ext: true, hidden: false }));
  assert.match(profileProblem({ prefs: {}, filterSets: [], presets: [] }, { filterSets: big }), /too big/);
  assert.equal(PROFILE_MAX_BYTES, 48 * 1024);

  login('Ana');
  const notes = [];
  const saver = profileSaver({ setNote: (t, e) => notes.push([t, !!e]), isOpen: () => true, failTitle: 'x' });
  saver.change({ filterSets: big }, () => assert.fail('too big'));
  assert.match(notes.at(-1)[0], /too big/);
  assert.equal(saver.busy(), false);
  sent.length = 0;
  let done = false;
  saver.change({ presets: [{ name: 'P', items: [], pinned: true }] }, () => { done = true; });
  assert.equal(saver.busy(), true);
  await new Promise((r) => setImmediate(r));
  const out = sent.at(-1);
  assert.deepEqual([out.t, out.name, out.presets.length], ['profileSave', 'Ana', 1]);
  deliver({ t: 'profileSaveResult', id: out.id, ok: true });
  await new Promise((r) => setImmediate(r));
  assert.equal(done, true);
  assert.equal(saver.busy(), false);
});

test('saveProfile: one at a time, the second built on the first; a lost socket settles them', async () => {
  const { saveProfile, profile } = await import('../profile.js');
  reset();
  login('Ana');
  sent.length = 0;
  const a = saveProfile({ prefs: { theme: 'dark' } });
  const b = saveProfile({ prefs: { ...profile().prefs, clock: '12' } });
  // The own copy takes each save at call time, so the second was built on the first.
  assert.deepEqual(profile().prefs, { theme: 'dark', clock: '12' });
  await new Promise((r) => setImmediate(r));
  assert.equal(sent.length, 1);
  deliver({ t: 'profileSaveResult', id: sent[0].id, ok: true });
  assert.equal((await a).ok, true);
  await new Promise((r) => setImmediate(r));
  assert.equal(sent.length, 2);
  // The socket drops before the second is answered: it settles, not hangs,
  // and the local copy goes back to what it was.
  m.emit('wsclose');
  const r = await b;
  assert.equal(r.ok, false);
  assert.match(r.err, /lost/i);
  assert.deepEqual(profile().prefs, { theme: 'dark' });
});
