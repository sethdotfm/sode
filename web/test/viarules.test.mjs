import { test } from 'node:test';
import assert from 'node:assert/strict';
import { viaNames, viaWord, viaText, originText, viaGroups, pieGroups, fedThrough, hopsVia, alsoNote } from '../viarules.js';

const label = (e) => `${e.router.toUpperCase()}.${e.n}${e.name ? ` ${e.name}` : ''}`;
const t3 = { id: 't1', name: 'TIE 3' };
const t9 = { id: 't9', name: 'TIE 9' };
const row = (x) => ({ dst: 20, level: 1, src: 5, via: [t3], origin: { router: 'a', n: 1 }, ...x });

test('the Via text names the origin and the tie lines, upstream first', () => {
  assert.equal(viaText(row(), label), 'A.1 Via TIE 3');
  assert.equal(viaText(row({ via: [t3, t9] }), label), 'A.1 Via TIE 3, TIE 9');
  assert.equal(viaNames(row({ via: [{ id: 't7', name: '' }] })), 't7');
  assert.equal(viaWord(row()), 'Via TIE 3');
});

test('the marks: mixed, nothing, unknown and loop', () => {
  assert.equal(viaText(row({ mixed: true, levels: [] }), label), 'A.1 Via TIE 3 (mixed)');
  assert.equal(viaText(row({ origin: null, nothing: true }), label), 'nothing Via TIE 3');
  assert.equal(viaText(row({ origin: null, unknown: 'a' }), label), 'unknown Via TIE 3');
  assert.equal(viaText(row({ origin: null, loop: true }), label), '∞ loop Via TIE 3');
  // A Hidden caller keeps the tie line names and loses the origin.
  assert.equal(viaText(row({ origin: null }), label), 'Via TIE 3');
  assert.equal(originText(row({ origin: null }), label), '');
});

test('levels with the same source and the same Via share a group', () => {
  const src = new Map([
    [1, 5],
    [2, 5],
    [3, 5],
    [4, 7],
  ]);
  const via = new Map([
    [1, row()],
    [2, row()],
    [3, row({ mixed: true })],
  ]);
  const g = viaGroups(
    [1, 2, 3, 4],
    (u) => src.get(u),
    (u) => via.get(u),
  );
  assert.deepEqual(
    g.map((x) => [x.src, x.levels, !!x.via, !!x.via?.mixed]),
    [
      [5, [1, 2], true, false],
      [5, [3], true, true],
      [7, [4], false, false],
    ],
  );
});

test('the level pie groups the upstream levels by origin', () => {
  const r = row({ mixed: true, levels: [{ router: 'a', n: 1 }, { router: 'a', n: 4 }, { router: 'a', n: 1 }, null] });
  assert.deepEqual(pieGroups(r), [
    { origin: { router: 'a', n: 1 }, levels: [1, 3] },
    { origin: { router: 'a', n: 4 }, levels: [2] },
    { origin: null, levels: [4] },
  ]);
  assert.deepEqual(pieGroups(row()), []);
});

test('a source feeds the destinations whose chain starts at it, on any router', () => {
  const b = new Map([
    [
      21,
      new Map([
        [1, row({ dst: 21 })],
        [2, row({ dst: 21, origin: { router: 'a', n: 2 } })],
      ]),
    ],
    [20, new Map([[2, row({ level: 2 })]])],
    [22, new Map([[1, row({ dst: 22, origin: null, nothing: true })]])],
  ]);
  const c = new Map([[1, new Map([[1, row({ dst: 1, via: [t3, t9] })]])]]);
  const fed = fedThrough({ router: 'a', n: 1 }, [
    ['b', b],
    ['c', c],
  ]);
  assert.deepEqual(
    fed.map((f) => [f.router, f.dst, f.levels, viaNames(f.row)]),
    [
      ['b', 20, [2], 'TIE 3'],
      ['b', 21, [1], 'TIE 3'],
      ['c', 1, [1], 'TIE 3, TIE 9'],
    ],
  );
  assert.deepEqual(fedThrough({ router: 'b', n: 1 }, [['b', b]]), []);
});

test('History: the Via of a tie take, and what an External on a tie line also changes', () => {
  const e = { origin: { router: 'a', n: 1 }, hops: [{ tie: 't1', tieName: 'TIE 3' }] };
  assert.equal(hopsVia(e), 'Via TIE 3');
  assert.equal(hopsVia({ hops: [] }), '');
  assert.equal(hopsVia({}), '');
  const ext = {
    also: [
      { router: 'b', n: 42, via: [t3] },
      { router: 'b', n: 50, via: [t3] },
      { router: 'c', n: 1, via: [t3, t9] },
    ],
  };
  assert.equal(alsoNote(ext, label), 'also changes B.42, B.50 Via TIE 3; C.1 Via TIE 3, TIE 9');
  assert.equal(alsoNote({ also: [{ router: 'b', n: 42, via: [t3] }], alsoOthers: 2 }, label), 'also changes B.42 Via TIE 3; 2 others');
  assert.equal(alsoNote({}, label), '');
});
