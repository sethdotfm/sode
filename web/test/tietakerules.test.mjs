import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  tieIndexes,
  openAt,
  nextQuestion,
  questionIds,
  questionHead,
  finalAt,
  planRequest,
  takeRequest,
  keptItems,
  changedAt,
  reopenPicks,
  failedText,
  reasonText,
  takeRan,
} from '../tietakerules.js';

const end = (router, n) => ({ router, n });
// A take from A.1 to A.3 (plain), B.4 and B.5 (tie routes), as a patch with three destinations.
const items = [{ dst: 'A.3' }, { dst: 'B.4' }, { dst: 'B.5' }];
const mixed = [
  { src: end('a', 1), dst: end('a', 3), levels: [1], tie: false, item: items[0] },
  { src: end('a', 1), dst: end('b', 4), levels: [1], tie: true, item: items[1] },
  { src: end('a', 1), dst: end('b', 5), levels: [1], tie: true, item: items[2] },
];
const pick = (i, id = 't1', name = 'TIE 1', seq = 1) => ({
  take: { ...mixed[i], tie: undefined, item: undefined, ties: [{ id, expect: null }], affected: [] },
  names: [name],
  ids: [id],
  seq,
});
const strip = ({ src, dst, levels }) => ({ src, dst, levels });

// path gives a plan path through the routers and tie lines: path(['a', 'b', 'c'], ['t1', 't2']).
const path = (routers, ties) => ({
  cost: 0,
  affected: [],
  hops: routers.map((router, i) => ({ router, ...(ties[i] ? { tie: { id: ties[i], name: ties[i] } } : {}) })),
});
const plan = (...paths) => ({ paths, reasons: [] });

test('the open tie routes come in plan order; a route with a path or cleared is not open', () => {
  assert.deepEqual(tieIndexes(mixed), [1, 2]);
  assert.deepEqual(openAt(mixed, []), [1, 2]);
  assert.deepEqual(openAt(mixed, [undefined, pick(1)]), [2]);
  assert.deepEqual(openAt(mixed, [undefined, { ids: ['t1'] }, { cleared: true }]), [1]);
});

test('one source to three destinations on router B is one question that goes to the destinations', () => {
  const routes = [4, 5, 6].map((n) => ({ src: end('a', 1), dst: end('b', n), levels: [1], tie: true }));
  const plans = routes.map(() => plan(path(['a', 'b'], ['t1']), path(['a', 'b'], ['t2'])));
  const q = nextQuestion(routes, plans, []);
  assert.deepEqual(q.members, [0, 1, 2]);
  assert.equal(q.at, 0);
  assert.equal(q.from, 0);
  assert.equal(q.to, null);
  assert.equal(finalAt(routes, [], q), true);
  assert.deepEqual(
    questionIds(q, plans[0].paths[1], routes).map((x) => [x.k, x.ids, x.full]),
    [
      [0, ['t2'], true],
      [1, ['t2'], true],
      [2, ['t2'], true],
    ],
  );
  assert.equal(
    questionHead(q, plans[0].paths[0], ['B.4', 'B.5', 'B.6'], (u) => u.toUpperCase()),
    'A → B, for B.4 B.5 B.6',
  );
});

test('destinations on B and C ask the hop A to B for all, then the hop B to C for the C destinations', () => {
  const routes = [
    { src: end('a', 1), dst: end('b', 4), levels: [1], tie: true },
    { src: end('a', 1), dst: end('c', 4), levels: [1], tie: true },
    { src: end('a', 1), dst: end('b', 5), levels: [1], tie: true },
    { src: end('a', 1), dst: end('c', 5), levels: [1], tie: true },
  ];
  const toB = plan(path(['a', 'b'], ['ab']));
  const toC = plan(path(['a', 'b', 'c'], ['ab', 'bc1']), path(['a', 'b', 'c'], ['ab', 'bc2']));
  const plans = [toB, toC, toB, toC];
  const q1 = nextQuestion(routes, plans, []);
  assert.deepEqual(q1.members, [0, 1, 2, 3]);
  assert.equal(q1.to, 0);
  assert.equal(q1.only, 'b');
  assert.equal(finalAt(routes, [], q1), false);
  const prefix = (u) => u.toUpperCase();
  assert.equal(questionHead(q1, toB.paths[0], ['B.4', 'C.4', 'B.5', 'C.5'], prefix), 'A → B, for B.4 C.4 B.5 C.5');
  const got = questionIds(q1, toB.paths[0], routes);
  assert.deepEqual(
    got.map((x) => [x.k, x.full]),
    [
      [0, true],
      [1, false],
      [2, true],
      [3, false],
    ],
  );
  // The B destinations got their path; the C destinations keep the first hop.
  const picks = [pick(1, 'ab'), { ids: ['ab'] }, pick(1, 'ab', 'ab', 2), { ids: ['ab'] }];
  const q2 = nextQuestion(routes, plans, picks);
  assert.deepEqual(q2.members, [1, 3]);
  assert.equal(q2.from, 1);
  assert.equal(q2.to, null);
  assert.equal(finalAt(routes, picks, q2), true);
  assert.equal(questionHead(q2, toC.paths[1], ['C.4', 'C.5'], prefix), 'B → C, for C.4 C.5');
  assert.deepEqual(questionIds(q2, toC.paths[1], routes)[0].ids, ['ab', 'bc2']);
});

test('a route that needs a hop that the others do not have gets its own question', () => {
  const routes = [
    { src: end('a', 1), dst: end('c', 4), levels: [1], tie: true },
    { src: end('a', 1), dst: end('b', 4), levels: [1], tie: true },
  ];
  // The best path to C goes straight from A to C.
  const plans = [plan(path(['a', 'c'], ['ac'])), plan(path(['a', 'b'], ['ab']))];
  assert.deepEqual(nextQuestion(routes, plans, []).members, [0]);
  // Another source, other levels or another stored tie line is another question too.
  const two = [
    { src: end('a', 1), dst: end('b', 4), levels: [1], tie: true },
    { src: end('a', 2), dst: end('b', 5), levels: [1], tie: true },
    { src: end('a', 1), dst: end('b', 6), levels: [2], tie: true },
    { src: end('a', 1), dst: end('b', 7), levels: [1], tie: true, stored: ['t2'] },
    { src: end('a', 1), dst: end('b', 8), levels: [1], tie: true },
  ];
  const ab = plan(path(['a', 'b'], ['t1']), path(['a', 'b'], ['t2']));
  assert.deepEqual(
    nextQuestion(
      two,
      two.map(() => ab),
      [],
    ).members,
    [0, 4],
  );
});

test('a split question asks each destination alone', () => {
  const routes = [4, 5].map((n) => ({ src: end('a', 1), dst: end('b', n), levels: [1], tie: true }));
  const plans = routes.map(() => plan(path(['a', 'b'], ['t1'])));
  const q = nextQuestion(routes, plans, []);
  const alone = nextQuestion(routes, plans, [], [q.key]);
  assert.deepEqual(alone.members, [0]);
  assert.equal(finalAt(routes, [], alone), false);
  assert.deepEqual(nextQuestion(routes, plans, [pick(1)], [q.key]).members, [1]);
});

test('a route with no path is a question of its own; no open route is no question', () => {
  const routes = [4, 5].map((n) => ({ src: end('a', 1), dst: end('b', n), levels: [1], tie: true }));
  assert.deepEqual(nextQuestion(routes, [plan(), plan(path(['a', 'b'], ['t1']))], []).members, [0]);
  assert.equal(nextQuestion(routes, [], [pick(1), { cleared: true }]), null);
});

test('with a plain route the confirm comes after the questions', () => {
  const q = { members: [1, 2], to: null };
  assert.equal(finalAt(mixed, [], q), false);
  assert.equal(finalAt(mixed.slice(1), [], { members: [0, 1], to: null }), true);
  assert.equal(finalAt(mixed.slice(1), [], { members: [0], to: null }), false);
});

test('a plan sends the plain routes, the routes with a path in their order, then the open routes through their choice', () => {
  assert.deepEqual(planRequest(mixed, []), { routes: [strip(mixed[0]), strip(mixed[1]), strip(mixed[2])], at: [0, 1, 2] });
  const p2 = pick(2);
  assert.deepEqual(planRequest(mixed, [undefined, { ids: ['t1'] }, p2]), {
    routes: [strip(mixed[0]), p2.take, { ...strip(mixed[1]), through: ['t1'] }],
    at: [0, 2, 1],
  });
  // A cleared route stays out.
  assert.deepEqual(planRequest(mixed, [undefined, { cleared: true }]).at, [0, 2]);
});

test('the take sends each route that stays in the order of the plan, and at maps it back', () => {
  const p1 = pick(1, 't1', 'TIE 1', 2);
  const p2 = pick(2, 't1', 'TIE 1', 1);
  const t = takeRequest(mixed, [undefined, p1, p2]);
  assert.deepEqual(t.routes, [strip(mixed[0]), p2.take, p1.take]);
  assert.deepEqual(t.at, [0, 2, 1]);
  assert.deepEqual(takeRequest(mixed, [undefined, { cleared: true }, p2]).at, [0, 2]);
});

test('the confirm keeps the plain items and marks the tie items via their tie lines', () => {
  const out = keptItems(items, mixed, [undefined, pick(1, 't1', 'TIE 1'), { cleared: true }]);
  assert.equal(out.length, 2);
  assert.equal(out[0], items[0]);
  assert.equal(out[1].dst, 'B.4');
  assert.equal(out[1].via, 'TIE 1');
});

test('a 409 tie_line_changed opens the first route that changed; every choice goes but Clear this route', () => {
  assert.equal(changedAt({ routes: [{ route: 2 }, { route: 1 }] }, [0, 1, 2]), 1);
  assert.equal(changedAt({ routes: [{ route: 0 }, { route: 1, changed: [{ id: 't1' }] }] }, [0, 1, 2]), 1);
  assert.equal(changedAt({ routes: [{ route: 0 }, { route: 1, newAffected: [{ n: 3 }] }] }, [2, 1]), 1);
  assert.equal(changedAt({ routes: [{ route: 1 }] }, [0, 2]), 2);
  assert.equal(changedAt({ routes: [] }, [0]), -1);
  const cleared = { cleared: true };
  assert.deepEqual(reopenPicks([undefined, pick(1), cleared, { ids: ['t1'] }]), [undefined, undefined, cleared, undefined]);
});

test('a failed route names its hop, else its error', () => {
  assert.equal(failedText({ hops: [{ tieName: 'TIE 1', result: 'failed' }] }), 'the hop to TIE 1 failed');
  assert.equal(failedText({ error: 'timeout' }), 'timeout');
  assert.equal(failedText({}), 'failed');
});

test('each no-path reason has a name', () => {
  const prefix = (uid) => uid.toUpperCase();
  assert.equal(reasonText({ why: 'no_tie_line', from: 'a', to: 'b' }, prefix), 'No tie line from router A to router B');
  assert.equal(reasonText({ why: 'router_down', router: 'b' }, prefix), 'Router B is down');
  assert.equal(reasonText({ why: 'protected', tie: 't1', tieName: 'TIE 1', by: 'Sode' }, prefix), 'TIE 1 is protected by Sode');
  assert.equal(reasonText({ why: 'taken', tie: 't1', tieName: 'TIE 1' }, prefix), 'TIE 1 carries another source in this take');
  assert.equal(reasonText({ why: 'taken', tie: 't1' }, prefix), 't1 carries another source in this take');
});

test('a take counts as sent only when the server ran it', () => {
  assert.equal(takeRan({ status: 200, body: { ok: false, results: [{ result: 'failed' }] } }), true);
  assert.equal(takeRan({ status: 409, body: { error: { code: 'tie_line_changed' }, routes: [] } }), false);
  assert.equal(takeRan({ status: 0, err: 'Not sent' }), false);
});
