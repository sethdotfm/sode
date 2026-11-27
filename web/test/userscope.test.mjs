import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = new Proxy({ baseURI: 'http://sode.test/' }, { get: (t, k) => (k in t ? t[k] : node()) });
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };
const si = globalThis.setInterval;
globalThis.setInterval = (...a) => {
  const t = si(...a);
  t.unref?.();
  return t;
};
const sent = [];
class FakeWS {
  static OPEN = 1;
  constructor() {
    this.readyState = 1;
    this.lastRx = Date.now();
    FakeWS.last = this;
  }
  send(m) {
    sent.push(JSON.parse(m));
  }
  close() {}
}
globalThis.WebSocket = FakeWS;

const m = await import('./all.mjs');
const { profileSaver, profileProblem, PROFILE_MAX_BYTES } = await import('../globalsave.js');
m.connect();
const deliver = (msg) => FakeWS.last.onmessage({ data: JSON.stringify(msg) });
const f = (o) => ({ src: '', dst: '', sode: true, ext: true, hidden: false, ...o });
const signIn = (name, filterSets = [], presets = []) => deliver({ t: 'profile', name, prefs: {}, filterSets, presets });
const reset = () => {
  mem.clear();
  m.S.cfg = { filterSets: [], presets: [] };
  Object.assign(m.routerData(''), { off: 1, numbering: '1', defaultLevels: '1' });
  signIn('');
  m.pickSet('all', 'All');
};

test('user sets come from the profile and sit between Everyone and This browser', () => {
  reset();
  m.S.cfg.filterSets = [f({ id: 'g1', name: 'Shared', dst: 'MV' })];
  m.saveLocalSet('Mine');
  assert.deepEqual(
    m.allSets().map((s) => `${s.scope}:${s.name}`),
    ['all:All', 'global:Shared', 'local:Mine'],
  );
  signIn('Ana', [f({ id: 'a1', name: 'Ana set', src: 'CAM' })]);
  assert.deepEqual(
    m.allSets().map((s) => `${s.scope}:${s.name}`),
    ['all:All', 'global:Shared', 'user:Ana set', 'local:Mine'],
  );
  signIn('');
  assert.deepEqual(m.userSets(), []);
});

test('a name used in two scopes is two sets; picking goes by scope', () => {
  reset();
  m.S.cfg.filterSets = [f({ id: 'gc', name: 'Cams', src: 'GLOBAL' })];
  signIn('Ana', [f({ id: 'ac', name: 'cams', src: 'ANA' })]);
  m.pickSet('user', 'ac');
  assert.equal(m.filterState().src, 'ANA');
  assert.equal(m.activeRef().user, 'Ana');
  assert.equal(m.isActive('user', 'ac'), true);
  assert.equal(m.isActive('global', 'gc'), false);
  m.pickSet('global', 'gc');
  assert.equal(m.filterState().src, 'GLOBAL');
});

test("the active set carries its user: Bob never picks up Ana's same-named set", () => {
  reset();
  signIn('Ana', [f({ id: 'm1', name: 'Mine', src: 'ANA' })]);
  m.pickSet('user', 'm1');
  assert.equal(m.activeSet().name, 'Mine');
  assert.equal(JSON.parse(mem.get('sode-filter-active')).user, 'Ana');
  signIn('Bob', [f({ id: 'm1', name: 'Mine', src: 'BOB' })]);
  assert.equal(m.activeSet().scope, 'all');
  assert.equal(m.filterState().src, '');
});

test('logging out drops a user set', () => {
  reset();
  signIn('Ana', [f({ id: 'm1', name: 'Mine', src: 'ANA' })]);
  m.pickSet('user', 'm1');
  signIn('');
  assert.equal(m.activeSet().scope, 'all');
  assert.equal(m.filterState().src, '');
  assert.equal(
    m.allSets().some((s) => s.scope === 'user'),
    false,
  );
});

test('a stack drops the sets of a user that left, keeps the rest', () => {
  reset();
  m.S.cfg.filterSets = [f({ id: 'g', name: 'G', dst: 'MV' })];
  signIn('Ana', [f({ id: 'a', name: 'A', src: 'ANA' })]);
  m.toggleSet('global', 'g');
  m.toggleSet('user', 'a');
  assert.equal(m.activeStack().length, 2);
  assert.deepEqual(
    JSON.parse(mem.get('sode-filter-stack')).map((r) => r.user),
    [undefined, 'Ana'],
  );
  signIn('Bob', []);
  assert.equal(m.activeStack().length, 0);
  assert.equal(m.activeSet().name, 'G');
});

test('a set edited on another device is followed unless this browser edited it', () => {
  reset();
  signIn('Ana', [f({ id: 'm1', name: 'Mine', src: 'ONE' })]);
  m.pickSet('user', 'm1');
  signIn('Ana', [f({ id: 'm1', name: 'Mine', src: 'TWO' })]);
  assert.equal(m.filterState().src, 'TWO');
  m.setFilter(f({ src: 'HAND' }));
  signIn('Ana', [f({ id: 'm1', name: 'Mine', src: 'THREE' })]);
  assert.equal(m.filterState().src, 'HAND');
  assert.equal(m.activeSet().edited, true);
});

test('a renamed set stays active and stacked: the hidden id names it', () => {
  reset();
  m.S.cfg.filterSets = [f({ id: 'g', name: 'Cams', dst: 'MV' })];
  signIn('Ana', [f({ id: 'a', name: 'Mine', src: 'ANA' })]);
  m.pickSet('global', 'g');
  m.S.cfg = { ...m.S.cfg, filterSets: [f({ id: 'g', name: 'Cameras', dst: 'MV' })] };
  m.emit('helloData');
  assert.deepEqual([m.activeSet().name, m.activeSet().edited], ['Cameras', false]);
  assert.equal(JSON.parse(mem.get('sode-filter-active')).id, 'g');
  m.toggleSet('user', 'a');
  signIn('Ana', [f({ id: 'a', name: 'Ours', src: 'ANA' })]);
  assert.deepEqual(
    m.activeStack().map((s) => s.name),
    ['Cameras', 'Ours'],
  );
  // This browser: a save under a new name keeps the id, so the active set and bookmarks follow.
  m.pickSet('all', 'All');
  const id = m.saveLocalSet('Desk');
  assert.ok(id);
  assert.equal(m.saveLocalSet('Desk 2', id), id);
  assert.deepEqual(
    m.localSets().map((s) => [s.id, s.name]),
    [[id, 'Desk 2']],
  );
  assert.deepEqual([m.activeSet().id, m.activeSet().name], [id, 'Desk 2']);
});

test('user presets: scoped after Everyone', () => {
  reset();
  m.S.cfg.presets = [{ id: 'g1', name: 'Show', items: [{ src: { router: 'r1', n: 1 }, dsts: [{ router: 'r1', n: 0 }], levels: [] }] }];
  signIn('Ana', [], [{ id: 'u1', name: 'Show', items: [{ src: { router: 'r1', n: 2 }, dsts: [{ router: 'r1', n: 1 }], levels: [] }] }]);
  assert.deepEqual(
    m.allPresets().map((s) => `${s.scope}:${s.name}`),
    ['global:Show', 'user:Show'],
  );
  assert.equal(m.findPreset('user', 'show').items[0].src, 2);
  assert.equal(m.findPreset('user', 'show').id, 'u1');
  signIn('');
  assert.deepEqual(m.userPresets(), []);
});

test('profileProblem refuses an oversized profile; saves send the whole list', async () => {
  reset();
  assert.equal(profileProblem({ prefs: {}, filterSets: [], presets: [] }, { filterSets: [f({ name: 'a' })] }), '');
  const big = Array.from({ length: 400 }, (_, i) => ({ name: `s${i}`, src: 'x'.repeat(200), dst: '', sode: true, ext: true, hidden: false }));
  assert.match(profileProblem({ prefs: {}, filterSets: [], presets: [] }, { filterSets: big }), /too big/);
  assert.equal(PROFILE_MAX_BYTES, 48 * 1024);

  signIn('Ana');
  const notes = [];
  const saver = profileSaver({ setNote: (t, e) => notes.push([t, !!e]), isOpen: () => true, failTitle: 'x' });
  saver.change({ filterSets: big }, () => assert.fail('too big'));
  assert.match(notes.at(-1)[0], /too big/);
  assert.equal(saver.busy(), false);
  sent.length = 0;
  let done = false;
  saver.change({ presets: [{ name: 'P', items: [], pinned: true }] }, () => {
    done = true;
  });
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
  signIn('Ana');
  sent.length = 0;
  const a = saveProfile({ prefs: { theme: 'dark' } });
  const b = saveProfile({ prefs: { ...profile().prefs, clock: '12' } });
  // The own copy gets each save when the call is made, so the second save builds on the first.
  assert.deepEqual(profile().prefs, { theme: 'dark', clock: '12' });
  await new Promise((r) => setImmediate(r));
  assert.equal(sent.length, 1);
  deliver({ t: 'profileSaveResult', id: sent[0].id, ok: true });
  assert.equal((await a).ok, true);
  await new Promise((r) => setImmediate(r));
  assert.equal(sent.length, 2);
  // The socket drops before the second answer: the save settles, and the local copy goes back.
  m.emit('wsclose');
  const r = await b;
  assert.equal(r.ok, false);
  assert.match(r.err, /Server link down/);
  assert.deepEqual(profile().prefs, { theme: 'dark' });
});
