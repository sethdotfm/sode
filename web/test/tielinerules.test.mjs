import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  tieLineProblem,
  listProblem,
  fillName,
  upsert,
  without,
  wireNumber,
  otherRouter,
  movedLines,
  removeQuestion,
  ALL_ROUTERS,
  findMatch,
  matches,
  pairGroups,
  pairHeading,
  countLine,
  routerCounts,
  carries,
  endOfItem,
} from '../tielinerules.js';

const shared = JSON.parse(readFileSync(new URL('./tieline_cases.json', import.meta.url), 'utf8'));

test('tieLineProblem says what the server says (tieline_cases.json)', () => {
  for (const c of shared.cases) assert.equal(tieLineProblem(c.t, shared.routers), c.problem, JSON.stringify(c.t));
});

const tie = (name, f, t, id = '') => ({ id, name, from: { router: 'a1', n: f }, to: { router: 'b2', n: t } });

test('listProblem: a name for each, unique ignoring case, and two ends used once', () => {
  const uids = ['a1', 'b2'];
  assert.equal(listProblem([tie('TIE 1', 1, 1), tie('TIE 2', 2, 2)], uids), '');
  assert.equal(listProblem([tie(' ', 1, 1)], uids), 'every tie line needs a name');
  assert.equal(listProblem([tie('TIE 1', 1, 1), tie('tie 1', 2, 2)], uids), 'two tie lines are named "tie 1"');
  assert.equal(listProblem([tie('TIE 1', 1, 1), tie('TIE 2', 1, 1)], uids), 'a1.1 already feeds b2.1 as TIE 1');
  assert.match(listProblem([tie('X'.repeat(33), 1, 1)], uids), /at most 32 characters/);
  assert.equal(listProblem([{ ...tie('TIE 1', 1, 1), to: { router: 'a1', n: 1 } }], uids), 'a tie line goes from one router to a different router');
});

test('the name fills from the downstream source name and can be edited', () => {
  let s = { name: '', auto: '' };
  s = fillName(s, 'FROM ST1');
  assert.deepEqual(s, { name: 'FROM ST1', auto: 'FROM ST1' });
  // Another source while the name is still the filled one: Sode fills the new name.
  s = fillName(s, ' CAM 4 ');
  assert.deepEqual(s, { name: 'CAM 4', auto: 'CAM 4' });
  // A typed name stays when the source changes.
  s = fillName({ ...s, name: 'TIE 3' }, 'CAM 5');
  assert.deepEqual(s, { name: 'TIE 3', auto: 'CAM 5' });
  // Sode fills a name that a person made blank.
  assert.deepEqual(fillName({ name: '  ', auto: 'CAM 5' }, 'CAM 6'), { name: 'CAM 6', auto: 'CAM 6' });
  // A source with no name leaves a blank name.
  assert.deepEqual(fillName({ name: 'CAM 6', auto: 'CAM 6' }, ''), { name: '', auto: '' });
});

test('upsert adds a new tie line and edits one in place by its id', () => {
  const list = [tie('A', 1, 1, 'x'), tie('B', 2, 2, 'y')];
  assert.deepEqual(
    upsert(list, tie('C', 3, 3)).map((t) => t.name),
    ['A', 'B', 'C'],
  );
  assert.deepEqual(
    upsert(list, tie('B2', 2, 2, 'y')).map((t) => t.name),
    ['A', 'B2'],
  );
  assert.deepEqual(
    without(list, 'x').map((t) => t.id),
    ['y'],
  );
});

test('wireNumber takes off the router number offset', () => {
  assert.equal(wireNumber('1', 1), 0);
  assert.equal(wireNumber(' 40 ', 0), 40);
  assert.ok(Number.isNaN(wireNumber('4a', 1)));
  assert.ok(Number.isNaN(wireNumber('', 1)));
  assert.ok(Number.isNaN(wireNumber('-2', 0)));
});

test('otherRouter offers the first router that is not the upstream one', () => {
  assert.equal(otherRouter(['a1', 'b2', 'c3'], 'a1'), 'b2');
  assert.equal(otherRouter(['a1', 'b2'], 'b2'), 'a1');
  assert.equal(otherRouter(['a1'], 'a1'), '');
});

test('movedLines: each Name moved end, with the stored name and the name now', () => {
  const shown = (r) => `${r.router}.${r.n + 1}`;
  assert.deepEqual(movedLines({ moved: [] }, shown), []);
  assert.deepEqual(movedLines({}, shown), []);
  const t = {
    moved: [
      { kind: 'destination', ref: { router: 'A', n: 1, name: 'TO STAGE 2' }, now: '' },
      { kind: 'source', ref: { router: 'B', n: 0, name: 'FROM STAGE 1' }, now: 'FROM STAGE ONE' },
    ],
  };
  assert.deepEqual(movedLines(t, shown), ['A.2 was TO STAGE 2, now no name', 'B.1 was FROM STAGE 1, now FROM STAGE ONE']);
});

test('removeQuestion names the tie line, its ends and the shared presets that store it', () => {
  const t = { id: 't1', name: 'TIE AB 1' };
  const presets = [
    { name: 'Studio feed', items: [{ ties: ['t1'] }] },
    { name: 'Other', items: [{ ties: ['t2'] }, {}] },
  ];
  assert.equal(
    removeQuestion(t, 'A.21 MV 8 feeds B.31 SRC 031', presets),
    "Remove tie line TIE AB 1? A.21 MV 8 feeds B.31 SRC 031. A preset that stores this tie line becomes Broken. Everyone's presets that store it: Studio feed.",
  );
  assert.equal(removeQuestion(t, 'A.21 feeds B.31'), 'Remove tie line TIE AB 1? A.21 feeds B.31. A preset that stores this tie line becomes Broken.');
});

test('listProblem: a cable in the list already comes before the name, with the ends as shown (R-20)', () => {
  const uids = ['a1', 'b2'];
  const shown = (e) => `${e.router === 'a1' ? 'A' : 'B'}.${e.n + 1}`;
  // The same ends and the same name: the cable is the problem, not the name.
  const list = [tie('CG3 Fill', 4, 6, 'x'), tie('CG3 Fill', 4, 6)];
  assert.equal(listProblem(list, uids, { shown }), 'A.5 already feeds B.7 as CG3 Fill');
  // An edit that moves a tie line onto the ends of a later one names the later one.
  const edited = [tie('TIE 9', 4, 6, 'x'), tie('CG3 Fill', 4, 6, 'y')];
  assert.equal(listProblem(edited, uids, { shown, changed: 'x' }), 'A.5 already feeds B.7 as CG3 Fill');
  // The same destination to a different source is no cable problem.
  assert.equal(listProblem([tie('TIE 1', 4, 6), tie('TIE 2', 4, 7)], uids, { shown }), '');
});

const end = (router, n, num, name = '') => ({ router, n, num, name });
const row = (id, name, from, to) => ({ id, name, from, to });
const routers = [
  { uid: 'a1', prefix: 'A', title: 'Stage 1' },
  { uid: 'b2', prefix: 'B', title: 'Stage 2' },
  { uid: 'c3', prefix: 'C', title: 'Truck' },
];
const rows = [
  row('1', 'TIE BA 1', end('b2', 3, 'B.4', 'TO A'), end('a1', 9, 'A.10', 'FROM B')),
  row('2', 'TIE AB 2', end('a1', 5, 'A.6', 'MV 2'), end('b2', 7, 'B.8', 'CAM 8')),
  row('3', 'TIE AB 1', end('a1', 4, 'A.5', 'MV 1'), end('b2', 6, 'B.7', 'CG3 Fill')),
  row('4', 'TIE AC 1', end('a1', 1, 'A.2', 'PGM'), end('c3', 0, 'C.1', 'IN 1')),
];
const all = { q: '', router: ALL_ROUTERS };

test('findMatch: the Tie line name, an end, or a number as shown', () => {
  const r = rows[2];
  assert.ok(findMatch(r, ''));
  assert.ok(findMatch(r, 'tie ab'));
  assert.ok(findMatch(r, 'cg3'));
  assert.ok(findMatch(r, 'A.5'));
  assert.ok(findMatch(r, 'a.5 mv'));
  assert.ok(findMatch(r, '7'));
  assert.ok(!findMatch(r, '6'), 'a bare number matches the whole number of an end');
  assert.ok(!findMatch(r, 'truck'));
});

test('matches: the router filter takes a tie line with either end on the router', () => {
  assert.deepEqual(
    rows.filter((r) => matches(r, { q: '', router: 'b2' })).map((r) => r.id),
    ['1', '2', '3'],
  );
  assert.deepEqual(
    rows.filter((r) => matches(r, { q: 'mv', router: 'a1' })).map((r) => r.id),
    ['2', '3'],
  );
});

test('pairGroups: one group for each router pair in router order, rows by destination number', () => {
  const g = pairGroups(rows, routers, all);
  assert.deepEqual(
    g.map((x) => [pairHeading(x), x.rows.map((r) => r.id)]),
    [
      ['A to B, 2', ['3', '2']],
      ['A to C, 1', ['4']],
      ['B to A, 1', ['1']],
    ],
  );
  assert.equal(g[0].from.title, 'Stage 1');
  const f = pairGroups(rows, routers, { q: 'MV 1', router: ALL_ROUTERS });
  assert.deepEqual(f.map(pairHeading), ['A to B, 1 of 2']);
  assert.deepEqual(pairGroups(rows, routers, { q: 'nothing', router: ALL_ROUTERS }), []);
});

test('countLine and routerCounts', () => {
  assert.equal(countLine([], all), 'No tie lines.');
  assert.equal(countLine(rows, all), '4 tie lines between 3 routers.');
  assert.equal(countLine(rows.slice(0, 1), all), '1 tie line between 2 routers.');
  assert.equal(countLine(rows, { q: '', router: 'c3' }), 'Shows 1 of 4.');
  assert.deepEqual(routerCounts(rows, routers, all), { [ALL_ROUTERS]: 4, a1: 4, b2: 3, c3: 1 });
  assert.deepEqual(routerCounts(rows, routers, { q: 'mv', router: 'c3' }), { [ALL_ROUTERS]: 2, a1: 2, b2: 2, c3: 0 });
});

test('carries: one source on every level, nothing, mixed or not known', () => {
  assert.deepEqual(carries(undefined), { kind: 'unknown' });
  assert.deepEqual(carries(new Map()), { kind: 'unknown' });
  assert.deepEqual(
    carries(
      new Map([
        [0, 4],
        [1, 4],
      ]),
    ),
    { kind: 'one', src: 4 },
  );
  assert.deepEqual(carries(new Map([[0, -1]])), { kind: 'nothing' });
  assert.deepEqual(
    carries(
      new Map([
        [0, 4],
        [1, 5],
      ]),
    ),
    { kind: 'mixed' },
  );
});

test('endOfItem: a match of a find box as a stored end', () => {
  assert.deepEqual(endOfItem({ n: 4, name: 'MV 1' }, 'a1'), { router: 'a1', n: 4 });
  assert.deepEqual(endOfItem({ n: 6, router: 'b2' }, 'a1'), { router: 'b2', n: 6 });
  assert.equal(endOfItem(undefined, 'a1'), null);
});
