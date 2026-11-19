import { test } from 'node:test';
import assert from 'node:assert/strict';
import { copyDoc, toClear, moveText } from '../browsercopyrules.js';

const values = {
  theme: 'dark',
  flip: 'on',
  filterSets: [{ name: 'Cams', src: 'cam' }],
  presets: [{ id: 'p1', name: 'Open', items: [] }],
};

test('every This browser pref has a Me counterpart today', () => {
  const doc = copyDoc(values);
  assert.deepEqual(doc.prefs, { theme: 'dark', flip: 'on' });
  assert.deepEqual(doc.browserOnly, []);
  assert.deepEqual(doc.filterSets, values.filterSets);
  assert.deepEqual(doc.presets, values.presets);
});

test('a pref the profile does not hold stays on this browser only', () => {
  const doc = copyDoc(values, ['theme']);
  assert.deepEqual(doc.prefs, { theme: 'dark' });
  assert.deepEqual(doc.browserOnly, [{ name: 'flip', value: 'on' }]);
});

test('a move clears what reached Me; Skip and Use current stay', () => {
  const got = toClear(
    {
      prefs: [
        { name: 'theme', on: true },
        { name: 'flip', on: false },
      ],
      rows: [
        { kind: 'filter', name: 'Cams', choice: 'import' },
        { kind: 'filter', name: 'Mons', choice: 'skip' },
        { kind: 'preset', name: 'Open', choice: 'replace' },
        { kind: 'preset', name: 'Shut', choice: 'both', as: 'Shut 2' },
        { kind: 'filter', name: 'Wide', choice: 'both', as: 'Wide 2' },
        { kind: 'preset', name: 'Wide', choice: 'current' },
      ],
    },
    false,
  );
  assert.deepEqual(got, {
    prefs: ['theme'],
    filterSets: [
      { from: 'Cams', to: 'Cams' },
      { from: 'Wide', to: 'Wide 2' },
    ],
    presets: ['Open', 'Shut'],
  });
});

test('Keep on this browser clears nothing', () => {
  const got = toClear({ prefs: [{ name: 'theme', on: true }], rows: [{ kind: 'preset', name: 'Open', choice: 'import' }] }, true);
  assert.deepEqual(got, { prefs: [], filterSets: [], presets: [] });
});

test('the result says moved or copied', () => {
  assert.equal(moveText('2 presets', false), '2 presets moved to Me.');
  assert.equal(moveText('2 presets', true), '2 presets copied to Me. This browser keeps them too.');
});
