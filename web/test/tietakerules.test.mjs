import { test } from 'node:test';
import assert from 'node:assert/strict';
import { tieIndexes, nextTie, finalAt, planRequest, takeRequest, keptItems, changedAt, resetFrom, failedText, reasonText, takeRan } from '../tietakerules.js';

const end = (router, n) => ({ router, n });
// A take from A.1 to A.3 (plain), B.4 and B.5 (tie routes), as a patch with three destinations.
const items = [{ dst: 'A.3' }, { dst: 'B.4' }, { dst: 'B.5' }];
const mixed = [
  { src: end('a', 1), dst: end('a', 3), levels: [1], tie: false, item: items[0] },
  { src: end('a', 1), dst: end('b', 4), levels: [1], tie: true, item: items[1] },
  { src: end('a', 1), dst: end('b', 5), levels: [1], tie: true, item: items[2] },
];
const pick = (i, id = 't1', name = 'TIE 1') => ({
  take: { ...mixed[i], tie: undefined, item: undefined, ties: [{ id, expect: null }], affected: [] },
  names: [name],
});
const strip = ({ src, dst, levels }) => ({ src, dst, levels });

test('the tie routes come in plan order; the next one is the first with no pick', () => {
  assert.deepEqual(tieIndexes(mixed), [1, 2]);
  assert.equal(nextTie(mixed, [], 0), 1);
  assert.equal(nextTie(mixed, [undefined, pick(1)], 0), 2);
  assert.equal(nextTie(mixed, [undefined, pick(1), { cleared: true }], 0), -1);
});

test('with a plain route the confirm comes after the pickers; with tie routes only the last picker sends', () => {
  assert.equal(finalAt(mixed, 2), false);
  const ties = mixed.slice(1);
  assert.equal(finalAt(ties, 0), false);
  assert.equal(finalAt(ties, 1), true);
  assert.equal(finalAt([mixed[1]], 0), true);
});

test('the plan of a tie route sends the plain routes, the routes with a pick and itself with no ties', () => {
  assert.deepEqual(planRequest(mixed, [], 1), [strip(mixed[0]), strip(mixed[1])]);
  const p1 = pick(1);
  assert.deepEqual(planRequest(mixed, [undefined, p1], 2), [strip(mixed[0]), p1.take, strip(mixed[2])]);
  // A cleared route stays out.
  assert.deepEqual(planRequest(mixed, [undefined, { cleared: true }], 2), [strip(mixed[0]), strip(mixed[2])]);
});

test('the take sends each route that stays, and at maps it back', () => {
  const p2 = pick(2);
  const t = takeRequest(mixed, [undefined, { cleared: true }, p2]);
  assert.deepEqual(t.routes, [strip(mixed[0]), p2.take]);
  assert.deepEqual(t.at, [0, 2]);
});

test('the confirm keeps the plain items and marks the tie items via their tie lines', () => {
  const out = keptItems(items, mixed, [undefined, pick(1, 't1', 'TIE 1'), { cleared: true }]);
  assert.equal(out.length, 2);
  assert.equal(out[0], items[0]);
  assert.equal(out[1].dst, 'B.4');
  assert.equal(out[1].via, 'TIE 1');
});

test('a 409 tie_line_changed opens the first route that it names; its pick and the later ones go', () => {
  assert.equal(changedAt({ routes: [{ route: 2 }, { route: 1 }] }, [0, 1, 2]), 1);
  assert.equal(changedAt({ routes: [{ route: 1 }] }, [0, 2]), 2);
  assert.equal(changedAt({ routes: [] }, [0]), -1);
  const cleared = { cleared: true };
  const picks = [undefined, pick(1), cleared, pick(3)];
  assert.deepEqual(resetFrom(picks, 1), [undefined, undefined, cleared, undefined]);
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
