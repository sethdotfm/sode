// Level display and parsing as pure functions: levels and numbering come in as arguments (a panel page has no socket).
import test from 'node:test';
import assert from 'node:assert/strict';
import { levelLabelOf, levelStyle, parseLevelsIn, levelInputOf, levelTextOf, relabelLevelsFor } from '../levels.js';

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

test('levelStyle: a hex colour gets black or white ink, a swatch gets a token', () => {
  assert.deepEqual(levelStyle({ color: '#ffffff', swatch: '01' }), { color: '#ffffff', ink: '#000' });
  assert.deepEqual(levelStyle({ color: '#000000', swatch: '01' }), { color: '#000000', ink: '#fff' });
  assert.deepEqual(levelStyle({ swatch: '03' }), { color: 'var(--pb-signal-03)', ink: null });
});

test('parseLevelsIn: blank means the default levels, typed in 1 numbering', () => {
  const ctx = { levels: apiLevels(17), numbering: 'va', defaultLevels: '1-3' };
  assert.deepEqual(parseLevelsIn('', ctx), [1, 2, 3]);
  assert.deepEqual(parseLevelsIn('v,1', ctx), [1, 2]);
  assert.equal(parseLevelsIn('', ctx, true), null);
  assert.equal(parseLevelsIn('18', { ...ctx, numbering: '1' }), null);
});

test('levelInputOf and levelTextOf follow the numbering', () => {
  assert.equal(levelInputOf([1, 2, 3, 5], apiLevels(17, 'va'), 'va'), 'V-2,4');
  assert.equal(levelInputOf([1, 2], apiLevels(17, 'va'), 'va', '1'), '1-2');
  assert.equal(levelTextOf([1, 2, 3, 5], apiLevels(17, '0'), '0'), '0–2, 4');
});

test('relabelLevelsFor: text keeps its levels when the numbering changes', () => {
  const lv = apiLevels(17);
  assert.equal(relabelLevelsFor('1-3', apiLevels(17, 'va'), 'va', { mode: '1', max: 17 }), 'V-2');
  assert.equal(relabelLevelsFor('1-3', lv, '1', { mode: '1', max: 17 }), '1-3');
  assert.equal(relabelLevelsFor('1-3', lv, 'va', null), '1-3');
});
