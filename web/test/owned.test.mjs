// The Settings form's config message leaves out what the server owns.
// Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';
import { OWNED_KEYS, omitOwned } from '../owned.js';

test('omitOwned drops every owned key and keeps the rest', () => {
  const cfg = { router: '1.2.3.4:8910', pinMaxTries: 10 };
  for (const k of OWNED_KEYS) cfg[k] = 'x';
  const out = omitOwned(cfg);
  for (const k of OWNED_KEYS) assert.ok(!(k in out), k);
  assert.deepEqual(out, { router: '1.2.3.4:8910', pinMaxTries: 10 });
  assert.ok('presets' in cfg, 'the config in use is left alone');
});

test('the owned lists are all there, and a huge config stays small', () => {
  assert.deepEqual([...OWNED_KEYS].sort(), ['autoLockMinutes', 'filterSets', 'locks', 'maxPinLen', 'presets', 'users']);
  const big = 'x'.repeat(48 * 1024);
  const cfg = { router: 'r', filterSets: [big], presets: [big], users: [big], locks: { main: true, big }, autoLockMinutes: 5 };
  assert.ok(JSON.stringify(omitOwned(cfg)).length < 200);
});
