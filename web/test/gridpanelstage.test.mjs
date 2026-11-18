// Staging on a Grid panel (gridpanelstage.js): keyed by destination and level, Take mode and Instant mode.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createStage,
  pressCrosspoint,
  stageRoutes,
  cancelConfirm,
  routesOf,
  stagedCount,
  confirmRoutes,
  dropStaged,
  unstageRoute,
  afterTake,
  stageText,
  STAGE_MAX,
} from '../gridpanelstage.js';

const take = () => createStage({ mode: 'take' });
const press = (st, src, dst, levels = [1, 2]) => pressCrosspoint(st, { src, dst }, levels);

test('a press with nothing staged stages that one route on the levels given', () => {
  const { st, send } = press(take(), 3, 7);
  assert.equal(send, null);
  assert.deepEqual(routesOf(st), [{ src: 3, dst: 7, levels: [1, 2] }]);
  assert.equal(stagedCount(st), 1);
});

test('a second press on a staged crosspoint keeps the stage and asks for the take confirm', () => {
  const first = press(take(), 3, 7).st;
  const r = press(first, 3, 7);
  assert.equal(r.confirm, true);
  assert.equal(r.send, null);
  assert.deepEqual(routesOf(r.st), [{ src: 3, dst: 7, levels: [1, 2] }]);
});

test('a press on one crosspoint of a staged Diagonal or line asks for the take confirm', () => {
  const st = diag(take(), three).st;
  const r = press(st, three[1].src, three[1].dst);
  assert.equal(r.confirm, true);
  assert.equal(stagedCount(r.st), 3);
});

test('another source in a staged row replaces only the levels it carries, so a breakaway is one plan', () => {
  let { st } = press(take(), 3, 7, [1, 2, 3]);
  st = press(st, 4, 7, [2]).st;
  assert.deepEqual(routesOf(st), [
    { src: 3, dst: 7, levels: [1, 3] },
    { src: 4, dst: 7, levels: [2] },
  ]);
  assert.equal(stagedCount(st), 2);
});

test('a source that covers every staged level of a row replaces the route whole', () => {
  let { st } = press(take(), 3, 7, [1, 2]);
  st = press(st, 4, 7, [1, 2]).st;
  assert.deepEqual(routesOf(st), [{ src: 4, dst: 7, levels: [1, 2] }]);
});

test('routes on other destinations stay', () => {
  let { st } = press(take(), 3, 7);
  st = press(st, 4, 8).st;
  assert.deepEqual(routesOf(st), [
    { src: 3, dst: 7, levels: [1, 2] },
    { src: 4, dst: 8, levels: [1, 2] },
  ]);
});

test('a press with no levels stages nothing and says why', () => {
  const { st } = press(take(), 3, 7, []);
  assert.deepEqual(routesOf(st), []);
  assert.equal(st.error.code, 'no-levels');
});

test(`at most ${STAGE_MAX} routes are staged; a press past that stages nothing and says why`, () => {
  assert.equal(STAGE_MAX, 128);
  let st = take();
  for (let d = 0; d < STAGE_MAX; d++) st = press(st, 1, d).st;
  const over = press(st, 1, STAGE_MAX).st;
  assert.equal(stagedCount(over), STAGE_MAX);
  assert.equal(over.error.code, 'cap');
  // A replace in a staged row adds no route, so it still works at the cap.
  const replaced = press(st, 2, 0).st;
  assert.equal(replaced.error, null);
  assert.equal(routesOf(replaced).find((r) => r.dst === 0).src, 2);
});

test('Take mode: the confirm sends every staged route, and a good take empties the stage', () => {
  let { st } = press(take(), 3, 7);
  st = press(st, 4, 8).st;
  assert.equal(confirmRoutes(st).length, 2);
  assert.deepEqual(routesOf(afterTake(st, null)), []);
});

test('Take mode: a failed take keeps the stage so it can be sent again', () => {
  const { st } = press(take(), 3, 7);
  const failed = afterTake(st, 'refused: protected');
  assert.equal(routesOf(failed).length, 1);
  assert.equal(failed.error.message, 'refused: protected');
});

test('nothing staged gives no confirm', () => {
  assert.equal(confirmRoutes(take()), null);
});

test('Instant mode sends a single route at once and holds nothing staged', () => {
  const { st, send } = press(createStage({ mode: 'instant' }), 3, 7);
  assert.deepEqual(send, [{ src: 3, dst: 7, levels: [1, 2] }]);
  assert.equal(stagedCount(st), 0);
  assert.equal(confirmRoutes(st), null);
});

test('each refusal says why in words', () => {
  assert.equal(stageText({ code: 'cap' }), 'At most 128 routes can be staged');
  assert.equal(stageText({ code: 'no-levels' }), "No levels to stage: this panel's levels are not on the router or outside its Rights");
  assert.equal(stageText({ code: 'take', message: 'Sode did not answer' }), 'Sode did not answer');
  assert.equal(stageText(null), '');
});

test('Esc drops every staged route', () => {
  const { st } = press(take(), 3, 7);
  assert.equal(stagedCount(dropStaged(st)), 0);
});

// A Diagonal is a list of { src, dst } pairs (gridpanelpick.js) staged by one press.
const diag = (st, pairs, levels = [1, 2]) => stageRoutes(st, pairs, levels);
const three = [
  { src: 1, dst: 10 },
  { src: 2, dst: 11 },
  { src: 3, dst: 12 },
];

test('a Diagonal stages one route per pair, in its order, on the levels given', () => {
  const r = diag(take(), three);
  assert.equal(r.send, null);
  assert.equal(r.confirm, false);
  assert.deepEqual(routesOf(r.st), [
    { src: 1, dst: 10, levels: [1, 2] },
    { src: 2, dst: 11, levels: [1, 2] },
    { src: 3, dst: 12, levels: [1, 2] },
  ]);
});

test('the same Diagonal pressed again adds nothing, so it asks for the take confirm', () => {
  const again = diag(diag(take(), three).st, three);
  assert.equal(stagedCount(again.st), 3);
  assert.equal(again.confirm, true);
});

test('a Diagonal pressed again with new levels stages those levels and asks for nothing', () => {
  const again = diag(diag(take(), three, [1]).st, three, [1, 2]);
  assert.equal(again.confirm, false);
  assert.deepEqual(
    routesOf(again.st).map((r) => r.levels),
    [
      [1, 2],
      [1, 2],
      [1, 2],
    ],
  );
});

test('a second press with other levels stages them; with fewer levels it adds nothing and confirms', () => {
  const st = press(take(), 3, 7, [1]).st;
  const more = press(st, 3, 7, [2]);
  assert.equal(more.confirm, false);
  assert.deepEqual(routesOf(more.st), [{ src: 3, dst: 7, levels: [1, 2] }]);
  const fewer = press(more.st, 3, 7, [2]);
  assert.equal(fewer.confirm, true);
  assert.deepEqual(routesOf(fewer.st), [{ src: 3, dst: 7, levels: [1, 2] }]);
});

test('a press that puts its source back on a level another source holds is a change, so it stages', () => {
  let st = press(take(), 3, 7, [1, 2]).st;
  st = press(st, 4, 7, [2]).st;
  const r = press(st, 3, 7, [1, 2]);
  assert.equal(r.confirm, false);
  assert.deepEqual(routesOf(r.st), [{ src: 3, dst: 7, levels: [1, 2] }]);
});

test('Instant mode never confirms a press because it adds nothing', () => {
  const r = press(createStage({ mode: 'instant' }), 3, 7);
  assert.equal(r.confirm, false);
  assert.deepEqual(r.send, [{ src: 3, dst: 7, levels: [1, 2] }]);
});

test('unstageRoute drops one staged route on every level and keeps the rest', () => {
  let st = press(take(), 3, 7, [1, 2]).st;
  st = press(st, 4, 7, [3]).st;
  st = press(st, 5, 8).st;
  const out = unstageRoute(st, { src: 3, dst: 7 });
  assert.deepEqual(routesOf(out), [
    { src: 4, dst: 7, levels: [3] },
    { src: 5, dst: 8, levels: [1, 2] },
  ]);
  assert.equal(unstageRoute(st, { src: 9, dst: 9 }), st);
});

test('a Diagonal replaces the staged levels on its destinations and leaves other levels', () => {
  let st = diag(take(), [{ src: 9, dst: 10 }], [1, 2, 3]).st;
  st = diag(st, three, [2]).st;
  assert.deepEqual(routesOf(st), [
    { src: 9, dst: 10, levels: [1, 3] },
    { src: 1, dst: 10, levels: [2] },
    { src: 2, dst: 11, levels: [2] },
    { src: 3, dst: 12, levels: [2] },
  ]);
});

test(`a Diagonal that would take the total past ${STAGE_MAX} stages nothing and says why`, () => {
  let st = take();
  for (let d = 0; d < STAGE_MAX - 2; d++) st = press(st, 1, d).st;
  const r = diag(st, [
    { src: 1, dst: 500 },
    { src: 1, dst: 501 },
    { src: 1, dst: 502 },
  ]);
  assert.equal(stagedCount(r.st), STAGE_MAX - 2);
  assert.equal(r.st.error.code, 'cap');
  assert.equal(stageText(r.st.error), 'At most 128 routes can be staged. This press adds 3 routes to 126.');
  // Two fit exactly.
  assert.equal(
    stagedCount(
      diag(st, [
        { src: 1, dst: 500 },
        { src: 1, dst: 501 },
      ]).st,
    ),
    STAGE_MAX,
  );
});

test('Instant mode: a press of more than one route sends at once, with no confirm', () => {
  const r = diag(createStage({ mode: 'instant' }), three);
  assert.equal(r.confirm, false);
  assert.equal(r.send.length, 3);
  assert.equal(stagedCount(r.st), 0);
});

test('Instant mode: the press after a Diagonal stands alone', () => {
  const { send } = press(diag(createStage({ mode: 'instant' }), three).st, 5, 20);
  assert.deepEqual(send, [{ src: 5, dst: 20, levels: [1, 2] }]);
});

test('Take mode: a cancelled confirm keeps the stage', () => {
  assert.equal(stagedCount(cancelConfirm(diag(take(), three).st)), 3);
});

test('Instant mode: a failed take holds nothing staged', () => {
  const st = afterTake(diag(createStage({ mode: 'instant' }), three).st, 'refused');
  assert.equal(stagedCount(st), 0);
  assert.equal(st.error.message, 'refused');
});

test('only the stored mode takeOnly turns the confirm box off; it is Take mode', () => {
  assert.equal(createStage({ mode: 'takeOnly' }).mode, 'take');
  assert.equal(createStage({ mode: 'takeOnly' }).ask, false);
  for (const mode of [undefined, 'take', 'instant', 'other']) assert.equal(createStage({ mode }).ask, true, String(mode));
});

test('with the box off a second press still asks for the take, and the routes are the staged ones', () => {
  const first = press(createStage({ mode: 'takeOnly' }), 3, 7);
  assert.equal(first.send, null);
  const r = press(first.st, 3, 7);
  assert.equal(r.confirm, true);
  assert.deepEqual(confirmRoutes(r.st), [{ src: 3, dst: 7, levels: [1, 2] }]);
});
