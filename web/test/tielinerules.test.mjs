import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { tieLineProblem, listProblem, fillName, upsert, without, wireNumber, otherRouter } from '../tielinerules.js';

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
  assert.equal(listProblem([tie('TIE 1', 1, 1), tie('TIE 2', 1, 1)], uids), 'another tie line has the same two ends ("TIE 2")');
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
