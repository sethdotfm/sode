// The Grid panel cursor (gridpanelcursor.js): where it starts, how keys move it, and how it follows new axes.
import test from 'node:test';
import assert from 'node:assert/strict';
import { CORNER, MOVES, moveCursor, cursorAt, followCursor, pointOf, barOf, rangeCursor, rangeKind } from '../gridpanelcursor.js';

// 10 destination rows by 20 source columns, 4 rows and 6 columns to a screen.
const dims = { rows: 10, cols: 20, page: { rows: 4, cols: 6 } };
const mv = (c, m, d = dims) => moveCursor(c, m, d);

test('the cursor starts at the corner, on neither axis', () => {
  assert.deepEqual(CORNER, { i: -1, j: -1 });
});

test('one-step moves go onto the bars and the crosspoints, and stop at each edge', () => {
  assert.deepEqual(mv(CORNER, 'down'), { i: 0, j: -1 });
  assert.deepEqual(mv(CORNER, 'right'), { i: -1, j: 0 });
  assert.deepEqual(mv(CORNER, 'up'), CORNER);
  assert.deepEqual(mv(CORNER, 'left'), CORNER);
  assert.deepEqual(mv({ i: 9, j: 19 }, 'down'), { i: 9, j: 19 });
  assert.deepEqual(mv({ i: 9, j: 19 }, 'right'), { i: 9, j: 19 });
  assert.deepEqual(mv({ i: 3, j: 4 }, 'up'), { i: 2, j: 4 });
  assert.deepEqual(mv({ i: 3, j: 4 }, 'left'), { i: 3, j: 3 });
});

test('a screen move goes one screen, stops at the last item, and stops at the first crosspoint before the bar', () => {
  assert.deepEqual(mv({ i: 1, j: 1 }, 'pageDown'), { i: 5, j: 1 });
  assert.deepEqual(mv({ i: 8, j: 1 }, 'pageDown'), { i: 9, j: 1 });
  assert.deepEqual(mv({ i: 5, j: 1 }, 'pageUp'), { i: 1, j: 1 });
  assert.deepEqual(mv({ i: 2, j: 1 }, 'pageUp'), { i: 0, j: 1 });
  assert.deepEqual(mv({ i: 2, j: 1 }, 'pageRight'), { i: 2, j: 7 });
  assert.deepEqual(mv({ i: 2, j: 3 }, 'pageLeft'), { i: 2, j: 0 });
  // From a bar, a screen move goes onto the crosspoints.
  assert.deepEqual(mv(CORNER, 'pageDown'), { i: 3, j: -1 });
  // Already on the bar, a screen move back stays there.
  assert.deepEqual(mv(CORNER, 'pageUp'), CORNER);
});

test('Home goes to the corner; Ctrl/Cmd+Home and End go to the row start and end; End goes to the column end', () => {
  assert.deepEqual(mv({ i: 4, j: 7 }, 'corner'), CORNER);
  assert.deepEqual(mv({ i: 4, j: 7 }, 'rowStart'), { i: 4, j: 0 });
  assert.deepEqual(mv({ i: 4, j: 7 }, 'rowEnd'), { i: 4, j: 19 });
  assert.deepEqual(mv({ i: 4, j: 7 }, 'colEnd'), { i: 9, j: 7 });
  assert.deepEqual(mv(CORNER, 'rowEnd'), { i: -1, j: 19 });
});

test('on an empty axis the cursor stays on its bar', () => {
  const none = { rows: 0, cols: 0, page: { rows: 4, cols: 6 } };
  for (const m of MOVES) assert.deepEqual(mv(CORNER, m, none), CORNER, m);
});

test('a page of no whole items still moves one', () => {
  assert.deepEqual(mv({ i: 1, j: 1 }, 'pageDown', { ...dims, page: { rows: 0, cols: 0 } }), { i: 2, j: 1 });
});

test('cursorAt names the router source and destination under the cursor, undefined on a bar', () => {
  const axes = { srcs: [7, 3, 9], dsts: [40, 41] };
  assert.deepEqual(cursorAt({ i: 1, j: 2 }, axes), { src: 9, dst: 41 });
  assert.deepEqual(cursorAt({ i: -1, j: 0 }, axes), { src: 7, dst: undefined });
  assert.deepEqual(cursorAt(CORNER, axes), { src: undefined, dst: undefined });
});

test('pointOf is the crosspoint under the cursor, or null on a bar', () => {
  const axes = { srcs: [7, 3, 9], dsts: [40, 41] };
  assert.deepEqual(pointOf({ i: 0, j: 1 }, axes), { src: 3, dst: 40 });
  assert.equal(pointOf({ i: -1, j: 1 }, axes), null);
  assert.equal(pointOf({ i: 0, j: -1 }, axes), null);
});

test('with new axes the cursor stays on the same source and destination', () => {
  const was = { srcs: [7, 3, 9], dsts: [40, 41, 42] };
  const now = { srcs: [1, 9, 3, 7], dsts: [42, 40] };
  assert.deepEqual(followCursor({ i: 2, j: 2 }, was, now), { i: 0, j: 1 });
  assert.deepEqual(followCursor({ i: -1, j: 0 }, was, now), { i: -1, j: 3 });
  assert.deepEqual(followCursor(CORNER, was, now), CORNER);
});

test('with an item gone the cursor keeps its index on that axis, inside the new length', () => {
  const was = { srcs: [7, 3, 9], dsts: [40, 41, 42] };
  assert.deepEqual(followCursor({ i: 2, j: 1 }, was, { srcs: [7, 9], dsts: [40, 41] }), { i: 1, j: 1 });
  assert.deepEqual(followCursor({ i: 2, j: 2 }, was, { srcs: [], dsts: [] }), CORNER);
});

test('barOf names the bar the cursor is on', () => {
  assert.equal(barOf({ i: -1, j: 3 }), 'srcs');
  assert.equal(barOf({ i: 2, j: -1 }), 'dsts');
  assert.equal(barOf(CORNER), 'corner');
  assert.equal(barOf({ i: 2, j: 3 }), null);
});

test('a range move stays on its bar and never reaches the corner', () => {
  assert.deepEqual(rangeCursor({ i: -1, j: 3 }, 'right', dims), { i: -1, j: 4 });
  assert.deepEqual(rangeCursor({ i: -1, j: 0 }, 'left', dims), { i: -1, j: 0 });
  assert.deepEqual(rangeCursor({ i: -1, j: 2 }, 'pageLeft', dims), { i: -1, j: 0 });
  assert.deepEqual(rangeCursor({ i: -1, j: 18 }, 'pageRight', dims), { i: -1, j: 19 });
  assert.deepEqual(rangeCursor({ i: 0, j: -1 }, 'up', dims), { i: 0, j: -1 });
  assert.deepEqual(rangeCursor({ i: 4, j: -1 }, 'pageDown', dims), { i: 8, j: -1 });
});

test('a Shift move on a crosspoint stays on the crosspoints and picks along the move', () => {
  const dims = { rows: 10, cols: 20, page: { rows: 4, cols: 5 } };
  assert.deepEqual(rangeCursor({ i: 3, j: 3 }, 'right', dims), { i: 3, j: 4 });
  assert.deepEqual(rangeCursor({ i: 0, j: 3 }, 'up', dims), { i: 0, j: 3 });
  assert.equal(rangeKind({ i: 3, j: 3 }, 'left'), 'srcs');
  assert.equal(rangeKind({ i: 3, j: 3 }, 'pageDown'), 'dsts');
  assert.equal(rangeKind({ i: -1, j: 3 }, 'down'), 'srcs');
});
