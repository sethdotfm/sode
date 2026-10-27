import test from 'node:test';
import assert from 'node:assert/strict';
import { PREFS, resolve, own, fromGlobal, toGlobal } from '../prefs.js';

test('own value wins, then global, then built-in', () => {
  assert.equal(resolve('theme', 'dark', { theme: 'light' }), 'dark');
  assert.equal(resolve('theme', null, { theme: 'light' }), 'light');
  assert.equal(resolve('theme', null, {}), 'system');
  assert.equal(resolve('theme', null, null), 'system');
});

test('unknown stored or global values are ignored', () => {
  assert.equal(resolve('theme', 'purple', { theme: 'light' }), 'light');
  assert.equal(resolve('theme', null, { theme: 'purple' }), 'system');
  assert.equal(resolve('clock', 'toString', null), '24');
  assert.equal(own('layout', 'constructor'), '');
  assert.equal(own('layout', 'phone'), 'phone');
});

test('on/off prefs take booleans from the config', () => {
  assert.equal(resolve('flip', null, { flip: true }), 'on');
  assert.equal(resolve('scrollTop', null, { scrollTop: false }), 'off');
  assert.equal(resolve('scrollTop', 'on', { scrollTop: false }), 'on');
  assert.equal(resolve('searchFilters', null, undefined), 'off');
  assert.equal(fromGlobal('flip', 'maybe'), null);
  assert.equal(toGlobal('flip', 'on'), true);
  assert.equal(toGlobal('clock', '12'), '12');
});

test('built-in defaults match the old behaviour', () => {
  const d = Object.fromEntries(Object.keys(PREFS).map((k) => [k, resolve(k, null, null)]));
  assert.deepEqual(d, { theme: 'system', layout: 'auto', clock: '24', flip: 'off', scrollTop: 'on', searchFilters: 'off' });
});
