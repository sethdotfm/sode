import test from 'node:test';
import assert from 'node:assert/strict';
import { scopeLabel, scopeIds, usableScopes, scopeOptions } from '../scopes.js';

test('scopes: labels, order, and Me only with a user', () => {
  assert.deepEqual(scopeIds(), ['global', 'user', 'local']);
  assert.deepEqual(scopeIds(true), ['all', 'global', 'user', 'local']);
  assert.equal(scopeLabel('all'), 'Default');
  assert.equal(scopeLabel('local'), 'This browser');
  assert.deepEqual(usableScopes(false), ['global', 'local']);
  assert.deepEqual(scopeOptions(true), [['local', 'This browser'], ['global', 'Everyone'], ['user', 'Me']]);
  assert.deepEqual(scopeOptions(false).map((o) => o[0]), ['local', 'global']);
});
