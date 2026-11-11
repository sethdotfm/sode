// The panel page's own chrome: kiosk on and off for one open page, and the header buttons' wording (panelchromerules.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { kioskNow, headHidden, leavesKiosk, fullscreenWords, kioskWords } from '../panelchromerules.js';

test('a press on the Kiosk button wins over the panel setting and the link; none follows them', () => {
  assert.equal(kioskNow(null, true), true);
  assert.equal(kioskNow(null, false), false);
  assert.equal(kioskNow(true, false), true);
  assert.equal(kioskNow(false, true), false);
});

test('the header hides for a kiosk or a panel without a title bar', () => {
  assert.equal(headHidden(false, true), false);
  assert.equal(headHidden(true, true), true);
  assert.equal(headHidden(false, false), true);
});

test('Escape leaves a kiosk unless something already took the key', () => {
  assert.equal(leavesKiosk({ key: 'Escape', defaultPrevented: false }, true), true);
  assert.equal(leavesKiosk({ key: 'Escape', defaultPrevented: true }, true), false);
  assert.equal(leavesKiosk({ key: 'Escape', defaultPrevented: false }, false), false);
  assert.equal(leavesKiosk({ key: 'Enter', defaultPrevented: false }, true), false);
});

test('the buttons name what the next press does', () => {
  assert.deepEqual(fullscreenWords(false), { icon: 'maximize', label: 'Full screen' });
  assert.deepEqual(fullscreenWords(true), { icon: 'minimize', label: 'Leave full screen' });
  assert.deepEqual(kioskWords(false), { icon: 'monitor', label: 'Kiosk mode' });
  assert.deepEqual(kioskWords(true), { icon: 'monitor-off', label: 'Leave kiosk mode' });
});
