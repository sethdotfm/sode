import { test } from 'node:test';
import assert from 'node:assert/strict';
import { toProfile } from '../profile.js';

test('toProfile: a clear or junk is the empty profile', () => {
  for (const m of [null, undefined, {}, { name: '' }, { name: 5 }]) {
    assert.equal(toProfile(m).name, '');
  }
});

test('toProfile: missing parts become empty ones', () => {
  const p = toProfile({ name: 'Ana', prefs: { theme: 'dark' } });
  assert.deepEqual(p, { name: 'Ana', prefs: { theme: 'dark' }, filterSets: [], presets: [] });
});
