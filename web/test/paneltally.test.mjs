// The tally a panel page keeps from the WebSocket's frames.
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
