import test from 'node:test';
import assert from 'node:assert/strict';
import { suggest, suggestSig, acceptName, endsKey, SUGGEST_MAX } from '../tielinesuggest.js';

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
  const got = suggest([A, B], [], new Set()).shown;
  assert.equal(got.length, 1);
  assert.deepEqual(got[0].from, { router: 'a', n: 0 });
  assert.deepEqual(got[0].to, { router: 'b', n: 5 });
  assert.equal(got[0].name, 'stage mon');
});

test('blank names and the same router give nothing', () => {
  assert.deepEqual(suggest([A], [], new Set()).shown, []);
  const blankA = { uid: 'a', dst: [[0, '']], src: [] };
  const blankB = { uid: 'b', dst: [], src: [[0, '']] };
  assert.deepEqual(suggest([blankA, blankB], [], new Set()).shown, []);
});

test('an existing tie line or a dismissal hides the suggestion', () => {
  const t = { from: { router: 'a', n: 0 }, to: { router: 'b', n: 5 }, name: 'x' };
  assert.deepEqual(suggest([A, B], [t], new Set()), { shown: [], more: 0 });
  assert.deepEqual(suggest([A, B], [], new Set([endsKey(t.from, t.to)])).shown, []);
});

test('many matches give a short list and a count of the rest, in the same order each time', () => {
  const spare = (uid, n0) => ({
    uid,
    dst: Array.from({ length: 50 }, (_, i) => [n0 + i, 'SPARE']),
    src: Array.from({ length: 50 }, (_, i) => [n0 + i, 'SPARE']),
  });
  const a = spare('a', 0);
  const b = spare('b', 0);
  const got = suggest([a, b], [], new Set());
  assert.equal(got.shown.length, SUGGEST_MAX);
  assert.equal(got.more, 2 * 50 * 50 - SUGGEST_MAX);
  assert.deepEqual(got.shown[0].from, { router: 'a', n: 0 });
  assert.deepEqual(got.shown[0].to, { router: 'b', n: 0 });
  assert.deepEqual(got.shown[1].to, { router: 'b', n: 1 });
  // The names can come in any order; the list does not change.
  const rev = (r) => ({ ...r, dst: [...r.dst].reverse(), src: [...r.src].reverse() });
  assert.deepEqual(suggest([rev(a), rev(b)], [], new Set()), got);
  // A dismissal shows the next one.
  const next = suggest([a, b], [], new Set([got.shown[0].key]));
  assert.equal(next.shown.at(-1).key, endsKey({ router: 'a', n: 0 }, { router: 'b', n: SUGGEST_MAX }));
  assert.equal(next.more, got.more - 1);
});

test('the same suggestions give the same signature, so the list is not built again', () => {
  const one = suggest([A, B], [], new Set());
  assert.equal(suggestSig(one), suggestSig(suggest([A, B], [], new Set())));
  assert.notEqual(suggestSig(one), suggestSig(suggest([A, B], [], new Set([one.shown[0].key]))));
  const renamed = { ...B, src: [[5, 'STAGE MON']] };
  assert.notEqual(suggestSig(one), suggestSig(suggest([A, renamed], [], new Set())));
});

test('accept name stays unique and short', () => {
  assert.equal(acceptName('TIE', [{ name: 'tie' }]), 'TIE 2');
  assert.equal(acceptName('TIE', [{ name: 'tie' }, { name: 'TIE 2' }]), 'TIE 3');
  assert.equal([...acceptName('x'.repeat(40), [])].length, 32);
});
