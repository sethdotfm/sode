import { test } from 'node:test';
import assert from 'node:assert/strict';
import { myDefaultButtons } from '../mydefaults.js';

test('nothing saved: only Save', () => {
  assert.deepEqual(myDefaultButtons('dark', ''), { save: true, clear: false });
  assert.deepEqual(myDefaultButtons('dark', undefined), { save: true, clear: false });
});
test('in force equals the profile: only Clear', () => {
  assert.deepEqual(myDefaultButtons('dark', 'dark'), { save: false, clear: true });
});
test('differs from the profile: both', () => {
  assert.deepEqual(myDefaultButtons('light', 'dark'), { save: true, clear: true });
});
