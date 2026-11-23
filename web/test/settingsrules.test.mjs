import test from 'node:test';
import assert from 'node:assert/strict';
import { intProblem, namesTimeText } from '../settingsrules.js';

const e = (key, value, type = 'int') => ({ key, type, value });

test('blank idle time is fine (means 0)', () => {
  assert.equal(intProblem([e('autoLockMinutes', '')]), null);
});
test('whole numbers pass', () => {
  assert.equal(intProblem([e('autoLockMinutes', '5'), e('pollSeconds', ' 60 ')]), null);
});
test('decimals and words fail', () => {
  assert.equal(intProblem([e('autoLockMinutes', '1.5')]).key, 'autoLockMinutes');
  assert.equal(intProblem([e('autoLockMinutes', 'abc')]).key, 'autoLockMinutes');
});
test('other blank numbers fail, non-int fields are ignored', () => {
  assert.equal(intProblem([e('pollSeconds', '')]).key, 'pollSeconds');
  assert.equal(intProblem([e('name', 'x', 'text')]), null);
});
test('a key left out of the config does not matter: only field values are read', () => {
  assert.equal(intProblem([e('autoLockMinutes', '0')]), null);
});

const fmt = (iso) => `T${iso}`;
test('saved names show their time until the first read', () => {
  assert.equal(namesTimeText({ at: '0001-01-01T00:00:00Z', savedAt: '2026-11-12T14:02:00Z' }, fmt), 'Names from T2026-11-12T14:02:00Z');
  assert.equal(namesTimeText({ at: null, savedAt: '2026-11-12T14:02:00Z' }, fmt), 'Names from T2026-11-12T14:02:00Z');
});
test('a read wins over the saved time', () => {
  assert.equal(namesTimeText({ at: '2026-11-18T09:00:00Z', savedAt: '2026-11-12T14:02:00Z' }, fmt), 'Re-read T2026-11-18T09:00:00Z');
});
test('no names yet', () => {
  assert.equal(namesTimeText({ at: '0001-01-01T00:00:00Z', savedAt: null }, fmt), 'Not read yet');
});
