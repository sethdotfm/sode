import test from 'node:test';
import assert from 'node:assert/strict';
import { thumbFor, thumbAt, dragTo, pageDir, wheelPx } from '../bigscrollrules.js';

test('the thumb is in scale with the view, never under the minimum', () => {
  assert.deepEqual(thumbFor(300, 1200, 300, 60), { len: 75, travel: 225 });
  assert.deepEqual(thumbFor(300, 60000, 300, 60), { len: 60, travel: 240 });
  assert.deepEqual(thumbFor(300, 300, 300, 60), { len: 300, travel: 0 });
  assert.deepEqual(thumbFor(300, 1200, 40, 60), { len: 40, travel: 0 });
});

test('the thumb position follows the scroll and stays on the track', () => {
  assert.equal(thumbAt(0, 900, 225), 0);
  assert.equal(thumbAt(450, 900, 225), 113);
  assert.equal(thumbAt(900, 900, 225), 225);
  assert.equal(thumbAt(5000, 900, 225), 225);
  assert.equal(thumbAt(-20, 900, 225), 0);
  assert.equal(thumbAt(10, 0, 225), 0);
});

test('a drag maps through travel, so a minimum thumb still reaches both ends', () => {
  assert.equal(dragTo(0, 240, 59700, 240), 59700);
  assert.equal(dragTo(0, 120, 59700, 240), 29850);
  assert.equal(dragTo(100, -500, 59700, 240), 0);
  assert.equal(dragTo(100, 30, 900, 0), 100);
});

test('a track press pages toward the press, not on the thumb', () => {
  assert.equal(pageDir(10, 50, 60), -1);
  assert.equal(pageDir(50, 50, 60), 0);
  assert.equal(pageDir(109, 50, 60), 0);
  assert.equal(pageDir(110, 50, 60), 1);
});

test('a vertical wheel moves a sideways bar, and lines and pages become px', () => {
  assert.equal(wheelPx(0, 120, 0, 'x', 500), 120);
  assert.equal(wheelPx(30, 120, 0, 'x', 500), 30);
  assert.equal(wheelPx(30, 120, 0, 'y', 500), 120);
  assert.equal(wheelPx(0, 3, 1, 'y', 500), 120);
  assert.equal(wheelPx(0, 1, 2, 'y', 500), 500);
});
