import { test } from 'node:test';
import assert from 'node:assert/strict';
import { whenText, ago, keyFacts, expiryFrom, keyNameProblem, createReq, ownerText, keysByOwner } from '../keyrules.js';

const NOW = Date.parse('2026-11-07T12:00:00Z');

test('ago says how long since, in the largest whole unit', () => {
  assert.equal(ago(NOW - 20_000, NOW), 'just now');
  assert.equal(ago(NOW - 5 * 60_000, NOW), '5 minutes ago');
  assert.equal(ago(NOW - 60_000, NOW), '1 minute ago');
  assert.equal(ago(NOW - 3 * 3_600_000, NOW), '3 hours ago');
  assert.equal(ago(NOW - 2 * 86_400_000, NOW), '2 days ago');
});

test('whenText treats the server zero time as never', () => {
  assert.equal(whenText('0001-01-01T00:00:00Z', NOW, 'never'), 'never');
  assert.equal(whenText(undefined, NOW, 'never'), 'never');
  assert.equal(whenText('2026-11-07T11:00:00Z', NOW, 'never'), '1 hour ago');
});

test('keyFacts gives created, last used and expiry in words, and flags an expired key', () => {
  const k = { id: 'ab', name: 'Panel 21', created: '2026-11-05T12:00:00Z', lastUsed: '0001-01-01T00:00:00Z', count: 0, expires: '0001-01-01T00:00:00Z' };
  assert.deepEqual(keyFacts(k, NOW), { name: 'Panel 21', created: '2 days ago', used: 'never used', expires: 'never expires', expired: false });
  const live = { ...k, lastUsed: '2026-11-07T11:30:00Z', count: 3, expires: '2026-11-09T12:00:00Z' };
  const f = keyFacts(live, NOW);
  assert.equal(f.used, 'used 30 minutes ago (3 requests)');
  assert.match(f.expires, /^expires in 2 days/);
  assert.equal(keyFacts({ ...k, expires: '2026-11-07T11:00:00Z' }, NOW).expired, true);
});

test('expiryFrom turns a date into unix ms at the end of that day, and refuses the past', () => {
  assert.deepEqual(expiryFrom('', NOW), { ms: 0 });
  const r = expiryFrom('2026-12-01', NOW);
  assert.equal(new Date(r.ms).getFullYear(), 2026);
  assert.equal(new Date(r.ms).getHours(), 23);
  assert.ok(expiryFrom('2026-11-01', NOW).error);
  assert.ok(expiryFrom('soon', NOW).error);
});

test('a key needs a key name', () => {
  assert.ok(keyNameProblem('   '));
  assert.equal(keyNameProblem('Panel 21'), '');
});

test('createReq carries the key name, the PIN and the expiry', () => {
  assert.deepEqual(createReq({ name: ' Panel 21 ', pin: '1234', expires: 0, id: 3 }), { t: 'keyCreate', id: 3, keyName: 'Panel 21', pin: '1234', expires: 0 });
});

test('ownerText names a user or an API client, and keysByOwner groups for the admin list', () => {
  assert.equal(ownerText({ kind: 'user', name: 'Ana' }), 'Ana (user)');
  assert.equal(ownerText({ kind: 'client', name: 'PanelsBot' }), 'PanelsBot (API client)');
  const keys = [
    { id: '1', owner: { kind: 'user', name: 'Ana' } },
    { id: '2', owner: { kind: 'client', name: 'PanelsBot' } },
    { id: '3', owner: { kind: 'user', name: 'Ana' } },
  ];
  const g = keysByOwner(keys);
  assert.equal(g.length, 2);
  assert.deepEqual(
    g[0].keys.map((k) => k.id),
    ['1', '3'],
  );
});
