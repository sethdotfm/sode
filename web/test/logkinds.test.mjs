import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeKinds, allKinds, pressAll, pressKind } from '../logkinds.js';

const ids = ['a', 'b', 'c'];

test('normalizeKinds: default, unknown ids and empty lists', () => {
  assert.deepEqual(normalizeKinds(undefined, ids, ['a']), ['a']);
  assert.deepEqual(normalizeKinds('x', ids, ['a']), ['a']);
  assert.deepEqual(normalizeKinds(['c', 'zzz', 'a'], ids, ['a']), ['a', 'c']);
  assert.deepEqual(normalizeKinds([], ids, ['a']), ids);
  assert.deepEqual(normalizeKinds(['zzz'], ids, ['a']), ids);
});

test('All is every kind selected', () => {
  assert.equal(allKinds(ids, ids), true);
  assert.equal(allKinds(['a', 'b'], ids), false);
  assert.deepEqual(pressAll(ids), ids);
});

test('pressing a kind with All on selects only it', () => {
  assert.deepEqual(pressKind(ids, ids, 'b'), ['b']);
});

test('kinds toggle, and the last one off goes back to All', () => {
  assert.deepEqual(pressKind(['a'], ids, 'c'), ['a', 'c']);
  assert.deepEqual(pressKind(['a', 'c'], ids, 'a'), ['c']);
  assert.deepEqual(pressKind(['c'], ids, 'c'), ids);
  // Selecting the rest one by one reaches All.
  assert.equal(allKinds(pressKind(['a', 'b'], ids, 'c'), ids), true);
});
