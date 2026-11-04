import test from 'node:test';
import assert from 'node:assert/strict';
import { PREFS, resolve, own, mine, fromGlobal, toGlobal } from '../prefs.js';

test('own value wins, then global, then built-in', () => {
  assert.equal(resolve('theme', 'dark', null, { theme: 'light' }), 'dark');
  assert.equal(resolve('theme', null, null, { theme: 'light' }), 'light');
  assert.equal(resolve('theme', null, null, {}), 'system');
  assert.equal(resolve('theme', null, null, null), 'system');
});

test('unknown stored or global values are ignored', () => {
  assert.equal(resolve('theme', 'purple', null, { theme: 'light' }), 'light');
  assert.equal(resolve('theme', null, null, { theme: 'purple' }), 'system');
  assert.equal(resolve('clock', 'toString', null, null), '24');
  assert.equal(own('layout', 'constructor'), '');
  assert.equal(own('layout', 'phone'), 'phone');
});

test('on/off prefs take booleans from the config', () => {
  assert.equal(resolve('flip', null, null, { flip: true }), 'on');
  assert.equal(resolve('scrollTop', null, null, { scrollTop: false }), 'off');
  assert.equal(resolve('scrollTop', 'on', null, { scrollTop: false }), 'on');
  assert.equal(resolve('searchFilters', null, undefined, undefined), 'off');
  assert.equal(fromGlobal('flip', 'maybe'), null);
  assert.equal(toGlobal('flip', 'on'), true);
  assert.equal(toGlobal('clock', '12'), '12');
});

test('built-in defaults match the old behaviour', () => {
  const d = Object.fromEntries(Object.keys(PREFS).map((k) => [k, resolve(k, null, null, null)]));
  assert.deepEqual(d, { theme: 'system', layout: 'auto', clock: '24', flip: 'off', scrollTop: 'on', searchFilters: 'off', setStack: 'and', nextMode: 'number' });
});

test('three layers: own, then profile, then global, then built-in', () => {
  const g = { theme: 'light', flip: false };
  assert.equal(resolve('theme', 'dark', { theme: 'system' }, g), 'dark');
  assert.equal(resolve('theme', null, { theme: 'dark' }, g), 'dark');
  assert.equal(resolve('theme', null, {}, g), 'light');
  assert.equal(resolve('theme', null, { theme: 'dark' }, null), 'dark');
  assert.equal(resolve('clock', null, {}, null), '24');
});

test('junk profile values are ignored', () => {
  const g = { theme: 'light' };
  assert.equal(resolve('theme', null, { theme: 'purple' }, g), 'light');
  assert.equal(resolve('theme', null, { theme: 7 }, g), 'light');
  assert.equal(resolve('theme', null, { theme: 'toString' }, g), 'light');
  assert.equal(resolve('theme', null, 'junk', g), 'light');
  assert.equal(resolve('theme', null, null, g), 'light');
});

test('profile bools are on/off strings (booleans tolerated)', () => {
  assert.equal(resolve('flip', null, { flip: 'on' }, { flip: false }), 'on');
  assert.equal(resolve('flip', null, { flip: 'off' }, { flip: true }), 'off');
  assert.equal(resolve('flip', null, { flip: true }, null), 'on');
  assert.equal(resolve('flip', null, { flip: 'maybe' }, { flip: true }), 'on');
  assert.equal(resolve('flip', 'off', { flip: 'on' }, null), 'off');
  assert.equal(mine('flip', { flip: 'on' }), 'on');
  assert.equal(mine('flip', null), '');
});
