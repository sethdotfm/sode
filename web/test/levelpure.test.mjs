// Levels and numbering come in as arguments: a panel page has no socket.
import test from 'node:test';
import assert from 'node:assert/strict';
import { levelLabelOf, levelStyle, parseLevelsIn, levelInputOf, levelTextOf, relabelLevelsFor, levelRouterOf } from '../levels.js';

const apiLevels = (n, mode = '1') => Array.from({ length: n }, (_, i) => ({ level: i + 1, label: levelLabelOf([], mode, i + 1), swatch: 'x' }));

test('levelLabelOf: the level own label, a mode preview, or the numbering', () => {
  const lv = [
    { level: 1, label: 'V' },
    { level: 2, label: '1' },
  ];
  assert.equal(levelLabelOf(lv, 'va', 2), '1');
  assert.equal(levelLabelOf(lv, 'va', 3), '2');
  assert.equal(levelLabelOf(lv, '1', 1, '0'), '0');
  assert.equal(levelLabelOf(lv, '1', 1, 'va'), 'V');
  assert.equal(levelLabelOf([{ ui: 4, label: 'x' }], '1', 4), 'x');
  assert.equal(levelLabelOf([], '0', 5), '4');
});

test('levelStyle: a hex color gets black or white ink, a swatch gets a token', () => {
  assert.deepEqual(levelStyle({ color: '#ffffff', swatch: '01' }), { color: '#ffffff', ink: '#000' });
  assert.deepEqual(levelStyle({ color: '#000000', swatch: '01' }), { color: '#000000', ink: '#fff' });
  assert.deepEqual(levelStyle({ swatch: '03' }), { color: 'var(--pb-signal-03)', ink: null });
});

test('parseLevelsIn: blank means the default levels, typed in 1 numbering', () => {
  const ctx = { levels: apiLevels(17), numbering: 'va', defaultLevels: '1-3' };
  assert.deepEqual(parseLevelsIn('', ctx), [1, 2, 3]);
  assert.deepEqual(parseLevelsIn('v,1', ctx), [1, 2]);
  assert.equal(parseLevelsIn('', ctx, true), null);
  assert.deepEqual(parseLevelsIn('', { ...ctx, levels: apiLevels(4), defaultLevels: '' }), [1, 2, 3, 4], 'blank default levels are every level');
  assert.equal(parseLevelsIn('18', { ...ctx, numbering: '1' }), null);
});

test('parseLevelsIn: on a router with one level, any typed levels mean its one level', () => {
  const one = { levels: apiLevels(1, 'va'), numbering: 'va', defaultLevels: '1-1' };
  assert.deepEqual(parseLevelsIn('1-17', { ...one, numbering: '1' }), [1]);
  assert.deepEqual(parseLevelsIn('a3', one), [1]);
  assert.deepEqual(parseLevelsIn('v', one), [1]);
  assert.deepEqual(parseLevelsIn('', one), [1]);
  assert.equal(parseLevelsIn('', one, true), null);
  assert.equal(parseLevelsIn('abc', one), null);
  // A router with more levels still refuses a level past its count.
  assert.equal(parseLevelsIn('1-18', { levels: apiLevels(17), numbering: '1', defaultLevels: '1-17' }), null);
});

test('levelInputOf and levelTextOf follow the numbering', () => {
  assert.equal(levelInputOf([1, 2, 3, 5], apiLevels(17, 'va'), 'va'), 'V,1-2,4', 'V never starts a range');
  assert.equal(levelTextOf([1, 2, 3, 5], apiLevels(17, 'va'), 'va'), 'V, 1–2, 4');
  assert.equal(levelTextOf([1, 2], apiLevels(17, 'va'), 'va'), 'V, 1');
  assert.equal(levelInputOf([1, 2], apiLevels(17, 'va'), 'va', '1'), '1-2');
  assert.equal(levelTextOf([1, 2, 3, 5], apiLevels(17, '0'), '0'), '0–2, 4');
});

test('relabelLevelsFor: text keeps its levels when the numbering changes', () => {
  const lv = apiLevels(17);
  assert.equal(relabelLevelsFor('1-3', apiLevels(17, 'va'), 'va', { mode: '1', max: 17 }), 'V,1-2');
  assert.equal(relabelLevelsFor('1-3', lv, '1', { mode: '1', max: 17 }), '1-3');
  assert.equal(relabelLevelsFor('1-3', lv, 'va', null), '1-3');
});

test('levelRouterOf: typed levels follow the router of the destination, else the typed prefix, else the source', () => {
  const routerOf = (it) => it.router || '';
  const routerOfText = (t) => (t.startsWith('B.') ? 'b1' : '');
  const a = { n: 1 };
  const b = { n: 2, router: 'b1' };
  assert.equal(levelRouterOf({ dsts: [b, a], src: a }, routerOf), 'b1');
  assert.equal(levelRouterOf({ dsts: [a, b], src: b }, routerOf), '');
  assert.equal(levelRouterOf({ dsts: [], dstText: 'B.4', src: a }, routerOf, routerOfText), 'b1');
  assert.equal(levelRouterOf({ dsts: [], src: b }, routerOf), 'b1');
  assert.equal(levelRouterOf({}, routerOf), '');
  // The phone and the desktop parse with that router: typed 1-17 on a router with one level is its one level.
  const ctx = { b1: { levels: apiLevels(1), numbering: '1', defaultLevels: '1' }, '': { levels: apiLevels(17), numbering: '1', defaultLevels: '1-17' } };
  assert.deepEqual(parseLevelsIn('1-17', ctx[levelRouterOf({ dsts: [b] }, routerOf)]), [1]);
  assert.deepEqual(
    parseLevelsIn('1-17', ctx[levelRouterOf({ dsts: [a] }, routerOf)]),
    apiLevels(17).map((l) => l.level),
  );
});
