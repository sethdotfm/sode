// The shared profile module: what the server's `profile` message becomes.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };

const { toProfile } = await import('../profile.js');

test('toProfile: a clear or junk is the empty profile', () => {
  for (const m of [null, undefined, {}, { name: '' }, { name: 5 }]) {
    assert.equal(toProfile(m).name, '');
  }
});

test('toProfile: missing parts become empty ones', () => {
  const p = toProfile({ name: 'Ana', prefs: { theme: 'dark' } });
  assert.deepEqual(p, { name: 'Ana', prefs: { theme: 'dark' }, filterSets: [], presets: [] });
});
