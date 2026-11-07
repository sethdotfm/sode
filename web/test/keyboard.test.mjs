import test from 'node:test';
import assert from 'node:assert/strict';
import { kbdUp } from '../keyboard.js';

test('kbdUp: the visual viewport well under the layout one, not zoomed', () => {
  assert.equal(kbdUp({ height: 844, scale: 1 }, 844), false);
  assert.equal(kbdUp({ height: 500, scale: 1 }, 844), true);
  assert.equal(kbdUp({ height: 790, scale: 1 }, 844), false); // a toolbar, not a keyboard
  assert.equal(kbdUp({ height: 420, scale: 2 }, 844), false); // pinch-zoomed in
  assert.equal(kbdUp({ height: 500, scale: 1 }, 500), false); // Android: the layout shrank with it
});
