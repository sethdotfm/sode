// The shared profile module: what the server's `profile` message becomes.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };

const { toProfile, editTarget } = await import('../profile.js');

test('toProfile: a clear or junk is the empty profile', () => {
  for (const m of [null, undefined, {}, { name: '' }, { name: 5 }]) {
    assert.equal(toProfile(m).name, '');
  }
});

test('toProfile: missing parts become empty ones', () => {
  const p = toProfile({ name: 'Ana', prefs: { theme: 'dark' } });
  assert.deepEqual(p, { name: 'Ana', prefs: { theme: 'dark' }, filterSets: [], presets: [] });
});

test('editTarget: one shape for both editors, entry found ignoring case', () => {
  const m = { user: 'Ana', name: 'ONE', profile: { prefs: { theme: 'dark' }, presets: [{ name: 'One', items: [] }] } };
  const t = editTarget(m, 'presets');
  assert.equal(t.user, 'Ana');
  assert.equal(t.old, 'One');
  assert.equal(t.lists, t.profile.presets);
  assert.deepEqual(t.profile.filterSets, []);
  assert.deepEqual(t.profile.prefs, { theme: 'dark' });
  assert.equal(editTarget({ user: 'Ana', profile: {} }, 'filterSets').found, null);
  assert.equal(editTarget({ user: '' }, 'presets'), null);
});
