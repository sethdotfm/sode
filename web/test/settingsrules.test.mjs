import test from 'node:test';
import assert from 'node:assert/strict';
import { intProblem, killShown } from '../settingsrules.js';

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
test('Kill shows only with the setting on, a PIN set and this connection unlocked', () => {
  assert.equal(killShown({ killEnabled: true }, true, true), true);
  assert.equal(killShown({ killEnabled: false }, true, true), false);
  assert.equal(killShown({ killEnabled: true }, false, true), false);
  assert.equal(killShown({ killEnabled: true }, true, false), false);
  assert.equal(killShown(null, true, true), false);
});
