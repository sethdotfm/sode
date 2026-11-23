import test from 'node:test';
import assert from 'node:assert/strict';
import { suggest, acceptName, endsKey } from '../tielinesuggest.js';

const A = {
  uid: 'a',
  dst: [
    [0, 'Stage Mon'],
    [1, '  '],
  ],
  src: [[0, 'CAM 1']],
};
const B = {
  uid: 'b',
  dst: [[2, 'Other']],
  src: [
    [5, ' stage mon '],
    [6, ''],
  ],
};

test('exact name match across routers, trimmed and case ignored', () => {
  const got = suggest([A, B], [], new Set());
  assert.equal(got.length, 1);
  assert.deepEqual(got[0].from, { router: 'a', n: 0 });
  assert.deepEqual(got[0].to, { router: 'b', n: 5 });
  assert.equal(got[0].name, 'stage mon');
});

test('blank names and the same router give nothing', () => {
  assert.deepEqual(suggest([A], [], new Set()), []);
  const blankA = { uid: 'a', dst: [[0, '']], src: [] };
  const blankB = { uid: 'b', dst: [], src: [[0, '']] };
  assert.deepEqual(suggest([blankA, blankB], [], new Set()), []);
});

test('an existing tie line or a dismissal hides the suggestion', () => {
  const t = { from: { router: 'a', n: 0 }, to: { router: 'b', n: 5 }, name: 'x' };
  assert.deepEqual(suggest([A, B], [t], new Set()), []);
  assert.deepEqual(suggest([A, B], [], new Set([endsKey(t.from, t.to)])), []);
});

test('accept name stays unique and short', () => {
  assert.equal(acceptName('TIE', [{ name: 'tie' }]), 'TIE 2');
  assert.equal(acceptName('TIE', [{ name: 'tie' }, { name: 'TIE 2' }]), 'TIE 3');
  assert.equal([...acceptName('x'.repeat(40), [])].length, 32);
});
