import test from 'node:test';
import assert from 'node:assert/strict';
import { buildChanged } from '../buildrules.js';

test('a page open across a build change reloads on the next hello', () => {
  assert.equal(buildChanged('Beta 2.0.0 (abc1234)', 'Beta 2.0.1 (def5678)'), true);
  assert.equal(buildChanged('Beta 2.0.0 (abc1234)', 'Beta 2.0.0 (def5678)'), true);
});

test('the first hello and a reconnect to the same build do not reload', () => {
  assert.equal(buildChanged(undefined, 'Beta 2.0.0'), false);
  assert.equal(buildChanged('', 'Beta 2.0.0'), false);
  assert.equal(buildChanged('Beta 2.0.0', 'Beta 2.0.0'), false);
});

test('a hello without a build label does not reload', () => {
  assert.equal(buildChanged('Beta 2.0.0', ''), false);
  assert.equal(buildChanged('Beta 2.0.0', undefined), false);
});
