import test from 'node:test';
import assert from 'node:assert/strict';
import { routerLevelsOf, defaultLevelsText, readRouterLevels, routerFields, routerLevelsProblem } from '../routerlevelrules.js';

const levels = (n, offset) => Array.from({ length: n }, (_, i) => ({ ui: i + 1, wire: i + 1 + offset }));

test('routerLevelsOf: the saved values of a router from the hello', () => {
  assert.deepEqual(routerLevelsOf({ uid: 'b1', levels: levels(17, -1), levelNumbering: 'va', defaultLevels: '1,2,3' }), {
    levels: 17,
    levelWireOffset: -1,
    levelNumbering: 'va',
    defaultLevels: '1-3',
  });
  assert.deepEqual(routerLevelsOf({ uid: 'b1', levels: levels(1, 4), defaultLevels: '1-1' }), {
    levels: 1,
    levelWireOffset: 4,
    levelNumbering: '1',
    defaultLevels: '1',
  });
});

test('the default levels box shows the router numbering and reads back in 1 numbering', () => {
  const saved = { levels: 17, levelWireOffset: -1, levelNumbering: 'va', defaultLevels: '1-3' };
  assert.equal(defaultLevelsText(saved), 'V-2');
  const now = readRouterLevels({ levels: '17', levelWireOffset: '-1', levelNumbering: 'va', defaultText: 'V-2' });
  assert.deepEqual(now, saved);
  assert.deepEqual(routerFields('b1', now), {
    'router/b1/levels': 17,
    'router/b1/levelWireOffset': -1,
    'router/b1/levelNumbering': 'va',
    'router/b1/defaultLevels': '1-3',
  });
});

test('a new level count reads the default levels against it', () => {
  assert.equal(readRouterLevels({ levels: '1', levelWireOffset: '0', levelNumbering: '1', defaultText: '1' }).defaultLevels, '1');
  assert.equal(readRouterLevels({ levels: '1', levelWireOffset: '0', levelNumbering: '1', defaultText: '1-3' }).defaultLevels, null);
  assert.equal(readRouterLevels({ levels: '4', levelWireOffset: '0', levelNumbering: '1', defaultText: '' }).defaultLevels, '');
});

test('routerLevelsProblem names the router and the field', () => {
  const ok = { levels: 4, levelWireOffset: 0, levelNumbering: '1', defaultLevels: '1-4' };
  assert.equal(routerLevelsProblem('B', ok), '');
  assert.match(routerLevelsProblem('B', { ...ok, levels: NaN }), /^Router B: Levels/);
  assert.match(routerLevelsProblem('B', { ...ok, levels: 300 }), /^Router B: Levels/);
  assert.match(routerLevelsProblem('B', { ...ok, levelWireOffset: NaN }), /^Router B: Level offset/);
  assert.match(routerLevelsProblem('B', { ...ok, defaultLevels: null }), /^Router B: Default levels/);
});
