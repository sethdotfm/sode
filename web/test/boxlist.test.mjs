import test from 'node:test';
import assert from 'node:assert/strict';
import { parseList, namedAs, listMatches, missingNote, nextAdd, nextAfter, nameStem, LIST_MAX } from '../boxlist.js';

const parts = (text) => parseList(text)?.parts.map(({ tail, ...p }) => (tail ? { ...p, tail } : p)) ?? null;

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
  for (const t of [
    '',
    '11',
    'Rec 1',
    'Rec',
    '11-',
    '11,',
    ',11',
    '-4',
    'Rec 1-Cam 4',
    '1-Rec 4',
    'Rec 1-2-3',
    'DSM-1',
    'Cam 1, Rec',
    '1-4 PIP 1',
    '1-4 PIP 1-2',
    'MV 1-4 PIP 1-2 X 1-2',
    'MV 1-4 PIP 1-Cam 2',
    'MV 1-4 PIP 1-2-3',
    'Rec 1-2 DSM-1',
  ]) {
    assert.equal(parseList(t), null, JSON.stringify(t));
  }
});

test('parseList reads a tail after a range of names', () => {
  assert.deepEqual(parts('MV 1-4 PIP 1'), [{ stem: 'MV', from: 1, to: 4, tail: 'PIP 1' }]);
  assert.deepEqual(parts('MV1-4PIP1'), [{ stem: 'MV', from: 1, to: 4, tail: 'PIP1' }]);
  assert.deepEqual(parts('MV 1-MV 4 PIP 1'), [{ stem: 'MV', from: 1, to: 4, tail: 'PIP 1' }]);
  assert.deepEqual(parts('MV 1-2 PIP 1, Rec 3'), [
    { stem: 'MV', from: 1, to: 2, tail: 'PIP 1' },
    { stem: 'Rec', from: 3, to: 3 },
  ]);
  assert.equal(parseList('MV 1-4 PIP 1').count, 4);
});

test('parseList reads a second range in the tail', () => {
  const mv = { stem: 'MV', from: 1, to: 4, sub: { stem: 'PIP', from: 1, to: 2, tail: '' } };
  assert.deepEqual(parts('MV 1-4 PIP 1-2'), [mv]);
  assert.deepEqual(parts('MV1-4PIP1-2'), [mv]);
  assert.deepEqual(parts('MV 1-MV 4 PIP 1-PIP 2'), [mv]);
  assert.deepEqual(parts('MV 4-1 PIP 2-1'), [mv], 'ranges written high to low');
  assert.deepEqual(parts('MV 1-4 PIP 1-2 L'), [{ ...mv, sub: { ...mv.sub, tail: 'L' } }]);
  assert.deepEqual(parts('MV 1-4 PIP 1-2, Rec 3'), [mv, { stem: 'Rec', from: 3, to: 3 }]);
  assert.equal(parseList('MV 1-4 PIP 1-2').count, 8);
  assert.equal(parseList('MV 1-4 PIP 1-2, 7').count, 9);
});

test('parseList marks a list with too many items', () => {
  assert.equal(parseList('MV 1-10 PIP 1-10').tooMany, false);
  assert.equal(parseList('MV 1-10 PIP 1-11').tooMany, true, 'the product counts');
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

test('listMatches: a tail filters the range, with spaces between words and numbers optional', () => {
  const mv = [
    { n: 20, num: '21', name: 'MV 1 PIP 1' },
    { n: 21, num: '22', name: 'MV 1 PIP 2' },
    { n: 22, num: '23', name: 'MV2 PIP1' },
    { n: 23, num: '24', name: 'MV 2 PIP 2' },
    { n: 24, num: '25', name: 'mv  3 pip 1' },
    { n: 25, num: '26', name: 'MV 11 PIP 1' },
  ];
  const m = listMatches(parseList('MV 1-4 PIP 1'), mv);
  assert.deepEqual(
    m.found.map((it) => it.n),
    [20, 22, 24],
  );
  assert.deepEqual(m.missing, ['MV 4 PIP 1']);
  assert.deepEqual(
    listMatches(parseList('MV1-2 PIP2'), mv).found.map((it) => it.n),
    [21, 23],
  );
  assert.deepEqual(
    listMatches(parseList('MV 1-1 PIP 1'), mv).found.map((it) => it.n),
    [20],
    'MV 11 PIP 1 is not MV 1 PIP 1',
  );
});

test('listMatches: a second range in the tail runs inside the first range', () => {
  const mv = [];
  for (let k = 1; k <= 3; k++) for (let j = 1; j <= 2; j++) mv.push({ n: mv.length, num: String(mv.length + 1), name: `MV ${k} PIP ${j}` });
  mv.push({ n: 20, num: '21', name: 'mv4pip1' });
  mv.push({ n: 21, num: '22', name: 'MV 1 PIP 11' });
  const m = listMatches(parseList('MV 1-4 PIP 1-2'), mv);
  assert.deepEqual(
    m.found.map((it) => it.name),
    ['MV 1 PIP 1', 'MV 1 PIP 2', 'MV 2 PIP 1', 'MV 2 PIP 2', 'MV 3 PIP 1', 'MV 3 PIP 2', 'mv4pip1'],
  );
  assert.deepEqual(m.missing, ['MV 4 PIP 2']);
  assert.deepEqual(
    listMatches(parseList('MV 2-1 PIP 02-01'), mv).found.map((it) => it.name),
    ['MV 1 PIP 1', 'MV 1 PIP 2', 'MV 2 PIP 1', 'MV 2 PIP 2'],
    'the list order stays, with no leading zeros',
  );
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
  assert.equal(nextAdd(items[0], items, has).n, 2, 'skips an added one');
  assert.equal(nextAdd(items[2], items, has).n, 4, 'skips a number the router does not have');
  assert.equal(nextAdd(items[5], items, has), null);
});

test('nextAdd uses Next match of the name when given the name search', () => {
  const list = [
    { n: 0, name: 'Cam 1' },
    { n: 1, name: 'Rec 1' },
    { n: 2, name: 'Cam 2' },
    { n: 3, name: 'Rec 2' },
    { n: 4, name: 'Rec 3' },
    { n: 5, name: 'Bars' },
  ];
  const find = (q) => list.filter((it) => it.name.toLowerCase().includes(q.toLowerCase()));
  const added = new Set([3]);
  const has = (it) => added.has(it.n);
  assert.equal(nextAdd(list[1], list, has).n, 2, 'by number');
  assert.equal(nextAdd(list[1], list, has, find).n, 4, 'by name, skipping an added one');
  assert.equal(nextAdd(list[4], list, has, find).n, 5, 'no higher name match: the next number');
  assert.equal(nextAfter(list[5], list, { find }), null);
  assert.equal(nameStem('Rec 12'), 'Rec');
});
