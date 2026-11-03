import test from 'node:test';
import assert from 'node:assert/strict';
import { padKey, sendDelay, idleDue, pageType, liftFrom, dots, MORE_DIGITS_WAIT } from '../lockrules.js';

test('padKey adds digits, deletes and clears, capped at the longest PIN', () => {
  assert.equal(padKey('', '1', [4]), '1');
  assert.equal(padKey('12', '3', [4]), '123');
  assert.equal(padKey('123', 'back', [4]), '12');
  assert.equal(padKey('', 'back', [4]), '');
  assert.equal(padKey('123', 'clear', [4]), '');
  assert.equal(padKey('1', 'x', [4]), '1');
  assert.equal(padKey('1', 'Enter', [4]), '1');
  assert.equal(padKey('1234', '5', [4]), '1234');
  assert.equal(padKey('1234', '5', [4, 6]), '12345');
  assert.equal(padKey('123456', '7', [4, 6]), '123456');
});

test('sendDelay: only lengths some user has, and waits if a longer PIN exists', () => {
  assert.equal(sendDelay('123', [4]), null);
  assert.equal(sendDelay('1234', [4]), 0);
  assert.equal(sendDelay('12345', [4]), null);
  assert.equal(sendDelay('1234', [4, 6]), MORE_DIGITS_WAIT);
  assert.equal(sendDelay('123456', [4, 6]), 0);
  assert.equal(sendDelay('12345', [4, 6]), null);
  assert.equal(sendDelay('1234', []), null);
});

test('idleDue', () => {
  assert.equal(idleDue(0, 10 * 60_000, 0), false);
  assert.equal(idleDue(0, 4 * 60_000 + 59_999, 5), false);
  assert.equal(idleDue(0, 5 * 60_000, 5), true);
  assert.equal(idleDue(1000, 5 * 60_000, 5), false);
});

test('pageType', () => {
  assert.equal(pageType('/'), 'main');
  assert.equal(pageType('/sode/'), 'main');
  assert.equal(pageType('/log'), 'log');
  assert.equal(pageType('/sode/log'), 'log');
  assert.equal(pageType('/log.html'), 'log');
  assert.equal(pageType('/grid'), 'grid');
  assert.equal(pageType('/grid/camera-1'), 'grid');
  assert.equal(pageType('/sode/grid/camera-1'), 'grid');
  assert.equal(pageType('/settings/grids'), 'main');
  assert.equal(pageType('/catalog'), 'main');
});

test('liftFrom moves the centred logo back over the page logo', () => {
  const from = { left: 10, top: 20, width: 100, height: 40 };
  const to = { left: 300, top: 400, width: 400, height: 160 };
  const t = liftFrom(from, to);
  assert.equal(t.scale, 0.25);
  assert.equal(t.x, 60 - 500);
  assert.equal(t.y, 40 - 480);
  assert.deepEqual(liftFrom(null, to), { x: 0, y: 0, scale: 1 });
  assert.deepEqual(liftFrom({ left: 0, top: 0, width: 0, height: 0 }, to), { x: 0, y: 0, scale: 1 });
});

test('dots', () => {
  assert.equal(dots(''), '');
  assert.equal(dots('123').length, 3);
});
