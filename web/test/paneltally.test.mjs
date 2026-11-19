import test from 'node:test';
import assert from 'node:assert/strict';
import { withRoutes, framesApply, sameRoutes } from '../paneltally.js';

test('a frame changes, adds and keeps routes, and leaves the read answer as it was', () => {
  const t = Object.freeze({
    levels: [{ level: 1 }],
    routes: Object.freeze([Object.freeze({ dst: 0, level: 1, src: 4 }), Object.freeze({ dst: 1, level: 1, src: 5 })]),
  });
  const got = withRoutes(t, [
    [1, 1, 9],
    [2, 2, -1],
  ]);
  assert.deepEqual(got.routes, [
    { dst: 0, level: 1, src: 4 },
    { dst: 1, level: 1, src: 9 },
    { dst: 2, level: 2, src: -1 },
  ]);
  assert.equal(got.levels, t.levels);
  assert.equal(t.routes[1].src, 5);
  assert.equal(t.routes.length, 2);
});

test('the last row of a frame for a route wins', () => {
  const got = withRoutes({ routes: [] }, [
    [3, 1, 1],
    [3, 1, 2],
  ]);
  assert.deepEqual(got.routes, [{ dst: 3, level: 1, src: 2 }]);
});

test('only a Hidden caller reads again in place of a frame', () => {
  assert.equal(framesApply({ visibility: 'hidden' }), false);
  assert.equal(framesApply({ visibility: 'readonly' }), true);
  assert.equal(framesApply(undefined), true);
});

test('the slow check compares routes in any order', () => {
  const a = {
    routes: [
      { dst: 1, level: 1, src: 2 },
      { dst: 0, level: 1, src: 3 },
    ],
  };
  assert.equal(
    sameRoutes(a, {
      routes: [
        { dst: 0, level: 1, src: 3 },
        { dst: 1, level: 1, src: 2 },
      ],
    }),
    true,
  );
  assert.equal(
    sameRoutes(a, {
      routes: [
        { dst: 0, level: 1, src: 3 },
        { dst: 1, level: 1, src: 4 },
      ],
    }),
    false,
  );
  assert.equal(sameRoutes(a, { routes: [{ dst: 0, level: 1, src: 3 }] }), false);
});

import { newFeed, wantsRead, readStarts, readCame, readEnds, framesCame, drawRows, helloCame, socketClosed, rightsCame, CHECK_MS } from '../paneltally.js';

const tally = (...rows) => ({ routes: rows.map(([dst, level, src]) => ({ dst, level, src })) });
// A feed after a hello and one good read at time 0.
const steady = () => readEnds(readCame(readStarts(helloCame(newFeed(true, {})), true), tally([0, 1, 4]), 1, 0));

test('feed: a steady feed reads only for the check', () => {
  const f = steady();
  assert.equal(f.stale, false);
  assert.equal(wantsRead(f, CHECK_MS - 1), false);
  assert.equal(wantsRead(f, CHECK_MS), true);
  assert.equal(wantsRead(newFeed(true, {}), 0), true); // nothing shown yet
  assert.equal(wantsRead(socketClosed(f), 1), true);
});

test('feed: frames during a read go into its answer', () => {
  let f = readStarts(steady(), true);
  const r = framesCame(f, [[0, 1, 7]]);
  assert.equal(r.act, 'queue');
  f = readEnds(readCame(r.feed, tally([0, 1, 4]), f.hellos, 5));
  assert.deepEqual(f.shown.routes, [{ dst: 0, level: 1, src: 7 }]);
  assert.equal(f.late, null);
  assert.equal(f.trusted, true);
});

test('feed: a check that finds the kept tally wrong stops trusting frames until a hello', () => {
  let f = readCame(readStarts(steady(), true), tally([0, 1, 9]), 1, CHECK_MS);
  assert.equal(f.trusted, false);
  assert.equal(wantsRead(f, CHECK_MS + 1), true);
  assert.equal(framesCame(f, [[0, 1, 2]]).act, '');
  f = helloCame(f);
  assert.equal(f.trusted, true);
  assert.equal(f.stale, true);
});

test('feed: rows waiting for their draw are part of what the check compares', () => {
  const r = framesCame(steady(), [[0, 1, 6]]);
  const f = readCame(readStarts(r.feed, true), tally([0, 1, 6]), 1, CHECK_MS);
  assert.equal(f.trusted, true);
});

test('feed: a hello during a read keeps the feed stale', () => {
  let f = readStarts(helloCame(steady()), true);
  const at = f.hellos;
  f = helloCame(f);
  f = readCame(f, tally([0, 1, 4]), at, 1);
  assert.equal(f.stale, true);
  f = readCame(readStarts(f, true), tally([0, 1, 4]), f.hellos, 2);
  assert.equal(f.stale, false);
});

test('feed: a Hidden caller reads on every frame, and widened Rights make the feed stale', () => {
  const hidden = { visibility: 'hidden' };
  let f = rightsCame(steady(), hidden);
  assert.equal(f.stale, true);
  f = readCame(readStarts(f, true), tally([0, 1, 4]), f.hellos, 1);
  assert.equal(wantsRead(f, 2), true);
  assert.equal(framesCame(f, [[0, 1, 5]]).act, 'poll');
  assert.equal(rightsCame(f, hidden).stale, false);
  assert.equal(rightsCame(f, {}).stale, true);
});

test('feed: with nothing shown a frame asks for a read, unless one is out', () => {
  const f = helloCame(newFeed(true, {}));
  assert.equal(framesCame(f, [[0, 1, 1]]).act, 'poll');
  assert.equal(framesCame(readStarts(f, true), [[0, 1, 1]]).act, '');
});

test('feed: drawing the rows puts them into shown', () => {
  const r = framesCame(steady(), [[1, 1, 3]]);
  const d = drawRows(r.feed);
  assert.equal(d.changed, true);
  assert.deepEqual(d.feed.rows, []);
  assert.equal(d.feed.shown.routes.length, 2);
  assert.equal(drawRows(d.feed).changed, false);
});
