// The ordered list's move rules (orderedlistrules.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { moveItem, dropTo, removeItem, canMove } from '../orderedlistrules.js';

const abcd = () => ['a', 'b', 'c', 'd'];

test('a move puts the item at the new place', () => {
  assert.deepEqual(moveItem(abcd(), 0, 2), ['b', 'c', 'a', 'd']);
  assert.deepEqual(moveItem(abcd(), 3, 1), ['a', 'd', 'b', 'c']);
  assert.deepEqual(moveItem(abcd(), 1, 2), ['a', 'c', 'b', 'd']);
});

test('a move out of range or onto itself gives the same list back', () => {
  const list = abcd();
  assert.equal(moveItem(list, 0, -1), list);
  assert.equal(moveItem(list, 3, 4), list);
  assert.equal(moveItem(list, 2, 2), list);
  assert.equal(moveItem(list, -1, 0), list);
  assert.equal(moveItem(list, 4, 0), list);
  assert.deepEqual(list, ['a', 'b', 'c', 'd']);
});

test('a drop before or after a row gives the place the item ends at', () => {
  // Down: before c lands between b and c, after c lands between c and d.
  assert.equal(dropTo(0, 2, false), 1);
  assert.equal(dropTo(0, 2, true), 2);
  // Up: before a lands first, after a lands second.
  assert.equal(dropTo(3, 0, false), 0);
  assert.equal(dropTo(3, 0, true), 1);
  // Onto itself, or next to itself, it stays.
  assert.equal(dropTo(1, 1, false), 1);
  assert.equal(dropTo(1, 1, true), 1);
  assert.equal(dropTo(1, 0, true), 1);
  assert.equal(dropTo(1, 2, false), 1);
  assert.deepEqual(moveItem(abcd(), 0, dropTo(0, 2, false)), ['b', 'a', 'c', 'd']);
  assert.deepEqual(moveItem(abcd(), 3, dropTo(3, 0, true)), ['a', 'd', 'b', 'c']);
});

test('with no side given, a drop takes the place of the row under it', () => {
  assert.equal(dropTo(0, 2), 2);
  assert.equal(dropTo(2, 0), 0);
  assert.deepEqual(moveItem(abcd(), 0, dropTo(0, 2)), ['b', 'c', 'a', 'd']);
  assert.deepEqual(moveItem(abcd(), 2, dropTo(2, 0)), ['c', 'a', 'b', 'd']);
});

test('a remove takes out one item, and out of range changes nothing', () => {
  assert.deepEqual(removeItem(abcd(), 1), ['a', 'c', 'd']);
  assert.deepEqual(removeItem(abcd(), 3), ['a', 'b', 'c']);
  const list = abcd();
  assert.equal(removeItem(list, 4), list);
  assert.equal(removeItem(list, -1), list);
  assert.deepEqual(removeItem(['a'], 0), []);
});

test('Move up stops at the first row and Move down at the last', () => {
  assert.deepEqual(canMove(0, 4), { up: false, down: true });
  assert.deepEqual(canMove(1, 4), { up: true, down: true });
  assert.deepEqual(canMove(3, 4), { up: true, down: false });
  assert.deepEqual(canMove(0, 1), { up: false, down: false });
});
