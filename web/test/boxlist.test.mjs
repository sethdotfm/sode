import test from 'node:test';
import assert from 'node:assert/strict';
import { parseList, namedAs, listMatches, missingNote, nextAdd, LIST_MAX } from '../boxlist.js';

const parts = (text) => parseList(text)?.parts ?? null;

test('parseList reads ranges and lists of numbers and names', () => {
  assert.deepEqual(parts('11-13'), [{ stem: '', from: 11, to: 13 }]);
  assert.deepEqual(parts('11,12,13'), [
    { stem: '', from: 11, to: 11 },
    { stem: '', from: 12, to: 12 },
    { stem: '', from: 13, to: 13 },
  ]);
  assert.deepEqual(parts(' 11 - 13 , 20 '), [
    { stem: '', from: 11, to: 13 },
    { stem: '', from: 20, to: 20 },
  ]);
  assert.deepEqual(parts('Rec 1-4'), [{ stem: 'Rec', from: 1, to: 4 }]);
  assert.deepEqual(parts('Rec 1-Rec 4'), [{ stem: 'Rec', from: 1, to: 4 }]);
  assert.deepEqual(parts('rec 1 - REC 4'), [{ stem: 'rec', from: 1, to: 4 }]);
  assert.deepEqual(parts('Rec 1, Rec 3'), [
    { stem: 'Rec', from: 1, to: 1 },
    { stem: 'Rec', from: 3, to: 3 },
  ]);
  assert.deepEqual(parts('13-11'), [{ stem: '', from: 11, to: 13 }], 'a range written high to low');
  assert.deepEqual(parts('MV 1-4, 11-12'), [
    { stem: 'MV', from: 1, to: 4 },
    { stem: '', from: 11, to: 12 },
  ]);
  assert.equal(parseList('11-13').count, 3);
});

test('parseList: plain searches and half-typed lists are not lists', () => {
  for (const t of ['', '11', 'Rec 1', 'Rec', '11-', '11,', ',11', '-4', 'Rec 1-Cam 4', '1-Rec 4', 'Rec 1-2-3', 'DSM-1', 'Cam 1, Rec']) {
    assert.equal(parseList(t), null, JSON.stringify(t));
  }
});

test('parseList marks a list with too many items', () => {
  assert.equal(parseList(`1-${LIST_MAX}`).tooMany, false);
  assert.equal(parseList(`1-${LIST_MAX + 1}`).tooMany, true);
  assert.equal(parseList('1-99999').tooMany, true);
});

const items = [
  { n: 0, num: '1', name: 'Rec 1' },
  { n: 1, num: '2', name: 'Rec 2' },
  { n: 2, num: '3', name: 'REC3' },
  { n: 4, num: '5', name: 'Rec 5' },
  { n: 9, num: '10', name: 'MV 1-4 PIP' },
  { n: 10, num: '11', name: 'MV 1' },
];

test('listMatches finds names and numbers in the order of the list, and names what is not there', () => {
  const m = listMatches(parseList('Rec 1-5'), items);
  assert.deepEqual(
    m.found.map((it) => it.n),
    [0, 1, 2, 4],
  );
  assert.deepEqual(m.missing, ['Rec 4']);
  const nums = listMatches(parseList('11, 1-3, 4, 2'), items);
  assert.deepEqual(
    nums.found.map((it) => it.n),
    [10, 0, 1, 2],
    'number 2 shows once',
  );
  assert.deepEqual(nums.missing, ['4']);
});

test('namedAs: a name that starts with the text makes it a search', () => {
  assert.equal(namedAs('MV 1-4', items), true);
  assert.equal(namedAs('mv  1-4 pip', items), true);
  assert.equal(namedAs('Rec 1-4', items), false);
  assert.equal(namedAs('  ', items), false);
});

test('missingNote is short', () => {
  assert.equal(missingNote([]), '');
  assert.equal(missingNote(['14', '15']), 'Not found: 14, 15');
  assert.equal(missingNote(['1', '2', '3', '4', '5', '6']), 'Not found: 1, 2, 3, 4 and 2 more');
});

test('nextAdd: the lowest number above the last add that is not added', () => {
  const added = new Set([1]);
  const has = (it) => added.has(it.n);
  assert.equal(nextAdd(0, items, has).n, 2, 'skips an added one');
  assert.equal(nextAdd(2, items, has).n, 4, 'skips a number the router does not have');
  assert.equal(nextAdd(10, items, has), null);
});
