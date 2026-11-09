import test from 'node:test';
import assert from 'node:assert/strict';
import { cleanSlug, slugProblem, SLUG_MAX } from '../slugrules.js';

test('typing folds to a valid short name', () => {
  assert.equal(cleanSlug('Studio A'), 'studio-a');
  assert.equal(cleanSlug('a_b'), 'a-b');
  assert.equal(cleanSlug('Café 9!'), 'caf-9');
  assert.equal(cleanSlug('x'.repeat(50)).length, SLUG_MAX);
  assert.equal(cleanSlug(undefined), '');
});

test('an empty short name is allowed and an edge dash or a reserved word is not', () => {
  assert.equal(slugProblem(''), '');
  assert.equal(slugProblem('studio-a'), '');
  assert.notEqual(slugProblem('-a'), '');
  assert.notEqual(slugProblem('a-'), '');
  assert.notEqual(slugProblem('editor'), '');
  assert.notEqual(slugProblem('p-0123abcd'), '');
});
