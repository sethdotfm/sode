import test from 'node:test';
import assert from 'node:assert/strict';
import { topDialog } from '../dialogkeys.js';

const d = (order, hidden = false, isConnected = true) => ({ order, root: { hidden, isConnected } });

test('topDialog: none open', () => {
  assert.equal(topDialog([]), null);
  assert.equal(topDialog([d(1, true), d(2, false, false)]), null);
});

test('topDialog: the most recently shown open one wins, whatever the list order', () => {
  const a = d(1), b = d(3), c = d(2);
  assert.equal(topDialog([a, b, c]), b);
  assert.equal(topDialog([b, a, c]), b);
});

test('topDialog: one hidden by someone else is skipped', () => {
  const a = d(1), b = d(2);
  b.root.hidden = true;
  assert.equal(topDialog([a, b]), a);
  b.root.hidden = false;
  assert.equal(topDialog([a, b]), b);
});
