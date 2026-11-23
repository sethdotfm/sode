import test from 'node:test';
import assert from 'node:assert/strict';
import { filterFn, entryAllows, routerTermsProblem, filterProblem } from '../filter.js';

test('a set with a router matches number and range terms on that router only', () => {
  const f = filterFn('5, 10-12, CAM', { router: 'r1' });
  assert.equal(f(5, 'MON', 'r1'), true);
  assert.equal(f(11, 'MON', 'r1'), true);
  assert.equal(f(5, 'MON', 'r2'), false);
  assert.equal(f(11, 'MON', 'r2'), false);
  // Name terms ignore the router.
  assert.equal(f(99, 'CAM 3', 'r2'), true);
  // An excluded number is excluded on its router only.
  const not = filterFn('!5', { router: 'r1' });
  assert.equal(not(5, 'X', 'r1'), false);
  assert.equal(not(5, 'X', 'r2'), true);
});

test('a set with no router: number terms match on any router', () => {
  const f = filterFn('5');
  assert.equal(f(5, 'X', 'r1'), true);
  assert.equal(f(5, 'X', 'r2'), true);
  assert.equal(f(5, 'X'), true);
});

test('entryAllows passes the entry router to both ends', () => {
  const set = { sode: true, ext: true, hidden: true };
  const ends = { srcOk: filterFn('1', { router: 'r1' }), dstOk: filterFn('2', { router: 'r1' }) };
  const x = { ext: false, sn: 1, sname: 'A', dn: 2, dname: 'B' };
  assert.equal(entryAllows(set, ends, { ...x, router: 'r1' }), true);
  assert.equal(entryAllows(set, ends, { ...x, router: 'r2' }), false);
});

test('with a second router, a set with no router allows only name, exact and regex terms', () => {
  assert.equal(routerTermsProblem('CAM, "MON 1", /^X/', '', 2), '');
  assert.match(routerTermsProblem('CAM, 5', '', 2), /router/);
  assert.match(routerTermsProblem('1-4', '', 3), /router/);
  assert.equal(routerTermsProblem('5', 'r1', 2), '');
  assert.equal(routerTermsProblem('5', '', 1), '');
  // The parser reports text that does not parse; this check skips it.
  assert.equal(routerTermsProblem('"open', '', 2), '');
});

test('the filter editor refuses number terms on a filter with no router once there is a second router', () => {
  assert.equal(filterProblem('CAM, 5', '', 1), null);
  assert.equal(filterProblem('CAM, 5', '', 2), 'Number terms need a router.');
  assert.equal(filterProblem('CAM, 5', 'r1', 2), null);
  assert.equal(filterProblem('', '', 2), null);
  // A parse error comes first and says where it is.
  assert.match(filterProblem('5, "open', '', 2), /at “"open”/);
});
