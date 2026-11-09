// The panel's state model (panelmodel.js): view, stage, arm and take, with no DOM.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createState,
  pressDestination,
  pressSource,
  unstage,
  clear,
  armed,
  plan,
  startTake,
  finishTake,
  failTake,
  applyTally,
  idleClear,
  sourceState,
  destinationState,
  currentSource,
  resultOf,
  errorOf,
  IDLE_MS,
  RESULT_MS,
  ERROR_MS,
} from '../panelmodel.js';

const tally = (rows) => new Map(Object.entries(rows).map(([d, ls]) => [Number(d), new Map(Object.entries(ls).map(([l, s]) => [Number(l), s]))]));
const env = (over = {}) => ({ levels: [1, 2], tally: tally({ 0: { 1: 5, 2: 5 }, 1: { 1: 6, 2: 7 } }), protectedDsts: new Set(), ...over });
const D = (n, levels) => ({ kind: 'destination', n, levels });
const S = (n, levels) => ({ kind: 'source', n, levels });
const T0 = 1000;

test('pressing a destination views it and its current source reads current', () => {
  const e = env();
  const st = pressDestination(createState(), D(0), e, T0);
  assert.equal(st.view.dst, 0);
  assert.equal(currentSource(e, st), 5);
  assert.equal(sourceState(st, 5, e), 'current');
  assert.equal(sourceState(st, 4, e), 'idle');
  assert.equal(destinationState(st, 0), 'view');
  assert.equal(destinationState(st, 1), 'idle');
});

test('with nothing viewed every source is idle and there is no current source', () => {
  const st = createState();
  assert.equal(sourceState(st, 5, env()), 'idle');
  assert.equal(currentSource(env(), st), -1);
});

test('a destination with different sources per level has no single current source', () => {
  const e = env();
  const st = pressDestination(createState(), D(1), e, T0);
  assert.equal(currentSource(e, st), -1);
  assert.equal(sourceState(st, 6, e), 'idle');
});

test('pressing a source with no destination viewed asks for a destination and stages nothing', () => {
  const st = pressSource(createState(), S(4), env(), T0);
  assert.equal(st.staged.length, 0);
  assert.equal(errorOf(st, T0).code, 'no-destination');
});

test('pressing a source stages it for the viewed destination, and the panel is armed', () => {
  const e = env();
  let st = pressDestination(createState(), D(0), e, T0);
  assert.equal(armed(st, e), false);
  st = pressSource(st, S(4), e, T0);
  assert.deepEqual(st.staged, [{ dst: 0, src: 4, levels: [1, 2] }]);
  assert.equal(sourceState(st, 4, e), 'pending');
  assert.equal(sourceState(st, 5, e), 'current');
  assert.equal(destinationState(st, 0), 'staged');
  assert.equal(armed(st, e), true);
});

test('restaging another source for the same destination replaces it', () => {
  const e = env();
  let st = pressDestination(createState(), D(0), e, T0);
  st = pressSource(pressSource(st, S(4), e, T0), S(3), e, T0);
  assert.deepEqual(st.staged.map((x) => x.src), [3]);
});

test('pressing the source a destination already has stages nothing and drops an earlier stage', () => {
  const e = env();
  let st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  st = pressSource(st, S(5), e, T0);
  assert.equal(st.staged.length, 0);
  assert.equal(armed(st, e), false);
});

test('several destinations stage together and one Take plan holds every route', () => {
  const e = env();
  let st = pressDestination(createState(), D(0), e, T0);
  st = pressSource(st, S(4), e, T0);
  st = pressDestination(st, D(1), e, T0);
  st = pressSource(st, S(3), e, T0);
  assert.deepEqual(plan(st, e), [
    { src: 4, dst: 0, levels: [1, 2] },
    { src: 3, dst: 1, levels: [1, 2] },
  ]);
});

test('pressing a staged destination again unstages only that one', () => {
  const e = env();
  let st = pressDestination(createState(), D(0), e, T0);
  st = pressSource(st, S(4), e, T0);
  st = pressDestination(st, D(1), e, T0);
  st = pressSource(st, S(3), e, T0);
  st = pressDestination(st, D(1), e, T0);
  assert.deepEqual(st.staged.map((x) => x.dst), [0]);
  assert.equal(st.view.dst, 1);
});

test('pressing a staged destination that is not viewed views it first', () => {
  const e = env();
  let st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  st = pressDestination(st, D(1), e, T0);
  st = pressDestination(st, D(0), e, T0);
  assert.equal(st.view.dst, 0);
  assert.equal(st.staged.length, 1);
});

test('unstage drops one destination and leaves the rest', () => {
  const e = env();
  let st = pressDestination(createState(), D(0), e, T0);
  st = pressDestination(pressSource(st, S(4), e, T0), D(1), e, T0);
  st = pressSource(st, S(3), e, T0);
  st = unstage(st, 0);
  assert.deepEqual(st.staged.map((x) => x.dst), [1]);
});

test('clear drops the plan, the view and any error', () => {
  const e = env();
  let st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  st = clear(st);
  assert.equal(st.view, null);
  assert.equal(st.staged.length, 0);
  assert.equal(errorOf(pressSource(clear(st), S(4), e, T0), T0).code, 'no-destination');
  assert.equal(errorOf(clear(pressSource(createState(), S(4), e, T0)), T0), null);
});

test('a Protected destination can be viewed but nothing is staged for it', () => {
  const e = env({ protectedDsts: new Set([0]) });
  let st = pressDestination(createState(), D(0), e, T0);
  assert.equal(st.view.dst, 0);
  st = pressSource(st, S(4), e, T0);
  assert.equal(st.staged.length, 0);
  assert.deepEqual(errorOf(st, T0), { code: 'protected', dst: 0, at: T0 });
  assert.equal(armed(st, e), false);
});

test('a Source and Destination with no common level are refused before arming', () => {
  const e = env();
  let st = pressDestination(createState(), D(0, [1]), e, T0);
  st = pressSource(st, S(4, [2]), e, T0);
  assert.equal(st.staged.length, 0);
  assert.deepEqual(errorOf(st, T0), { code: 'no-common-levels', src: 4, dst: 0, at: T0 });
});

test('a take sends the levels the cells and the panel share', () => {
  const e = env({ levels: [1, 2, 3] });
  let st = pressDestination(createState({ levels: [1, 2] }), D(0, [2, 3]), e, T0);
  st = pressSource(st, S(4, [1, 2, 3]), e, T0);
  assert.deepEqual(plan(st, e), [{ src: 4, dst: 0, levels: [2] }]);
});

test('with no levels set anywhere a take sends every router level', () => {
  const e = env({ levels: [1, 2, 3] });
  const st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  assert.deepEqual(plan(st, e)[0].levels, [1, 2, 3]);
});

test('a level with nothing routed does not hide the source the other levels share', () => {
  const e = env({ tally: tally({ 0: { 1: 5 }, 1: { 1: 6, 2: -1 } }) });
  const st = pressDestination(createState(), D(0), e, T0);
  assert.equal(currentSource(e, st), 5);
  assert.equal(currentSource(e, pressDestination(createState(), D(1), e, T0)), 6);
  assert.equal(currentSource(e, pressDestination(createState(), D(9), e, T0)), -1);
});

test('the current source follows only the levels the panel sends', () => {
  const e = env();
  const st = pressDestination(createState({ levels: [1] }), D(1), e, T0);
  assert.equal(currentSource(e, st), 6);
});

test('a tally change moves the current source and drops a stage the router now satisfies', () => {
  const e = env();
  let st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  const e2 = env({ tally: tally({ 0: { 1: 4, 2: 4 } }) });
  assert.equal(sourceState(st, 5, e2), 'idle');
  assert.equal(sourceState(st, 4, e2), 'current');
  st = applyTally(st, e2);
  assert.equal(st.staged.length, 0);
  assert.equal(armed(st, e2), false);
});

test('a tally change that does not satisfy a stage keeps it', () => {
  const e = env();
  const st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  assert.equal(applyTally(st, env({ tally: tally({ 0: { 1: 9, 2: 9 } }) })).staged.length, 1);
});

test('a destination that becomes Protected disarms the Take', () => {
  const e = env();
  const st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  assert.equal(armed(st, env({ protectedDsts: new Set([0]) })), false);
});

test('instant mode never arms a Take cell', () => {
  const e = env();
  const st = pressSource(pressDestination(createState({ mode: 'instant' }), D(0), e, T0), S(4), e, T0);
  assert.equal(armed(st, e), false);
});

test('Take sends the plan once, marks the panel busy, and refuses a second press', () => {
  const e = env();
  const st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  const first = startTake(st, e, T0);
  assert.deepEqual(first.routes, [{ src: 4, dst: 0, levels: [1, 2] }]);
  assert.equal(first.state.busy, true);
  assert.equal(startTake(first.state, e, T0), null);
  assert.equal(armed(first.state, e), false);
});

test('Take on an unarmed panel sends nothing', () => {
  assert.equal(startTake(createState(), env(), T0), null);
});

test('a connected result clears the stage and shows a result icon for a short while', () => {
  const e = env();
  const st = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  const { state } = startTake(st, e, T0);
  const done = finishTake(state, { ok: true, results: [{ src: 4, dst: 0, result: 'connected' }] }, T0 + 50);
  assert.equal(done.busy, false);
  assert.equal(done.staged.length, 0);
  assert.equal(resultOf(done, 0, T0 + 60), 'connected');
  assert.equal(resultOf(done, 0, T0 + 50 + RESULT_MS + 1), null);
  assert.equal(resultOf(done, 1, T0 + 60), null);
});

test('an already result clears the stage too', () => {
  const e = env();
  const { state } = startTake(pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0), e, T0);
  const done = finishTake(state, { ok: true, results: [{ src: 4, dst: 0, result: 'already' }] }, T0);
  assert.equal(resultOf(done, 0, T0), 'already');
  assert.equal(done.staged.length, 0);
});

test('a failed route stays staged so Take can be pressed again', () => {
  const e = env();
  let st = pressDestination(createState(), D(0), e, T0);
  st = pressDestination(pressSource(st, S(4), e, T0), D(1), e, T0);
  st = pressSource(st, S(3), e, T0);
  const { state } = startTake(st, e, T0);
  const done = finishTake(
    state,
    {
      ok: false,
      results: [
        { src: 4, dst: 0, result: 'connected' },
        { src: 3, dst: 1, result: 'failed' },
      ],
    },
    T0,
  );
  assert.deepEqual(done.staged.map((x) => x.dst), [1]);
  assert.equal(resultOf(done, 1, T0), 'failed');
  assert.equal(armed(done, e), true);
});

test('a request that failed keeps the stage, ends busy and reports the reason', () => {
  const e = env();
  const { state } = startTake(pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0), e, T0);
  const done = failTake(state, 'outside_rights', 'destination 1 is outside this key rights', T0);
  assert.equal(done.busy, false);
  assert.equal(done.staged.length, 1);
  assert.deepEqual(errorOf(done, T0), { code: 'refused', reason: 'outside_rights', message: 'destination 1 is outside this key rights', at: T0 });
});

test('an error shows for a few seconds', () => {
  const st = pressSource(createState(), S(4), env(), T0);
  assert.ok(errorOf(st, T0 + ERROR_MS - 1));
  assert.equal(errorOf(st, T0 + ERROR_MS + 1), null);
});

test('a pending selection clears after 30 idle seconds and a press resets the clock', () => {
  const e = env();
  let st = pressDestination(createState(), D(0), e, T0);
  assert.equal(idleClear(st, T0 + IDLE_MS - 1).view.dst, 0);
  assert.equal(idleClear(st, T0 + IDLE_MS + 1).view, null);
  st = pressSource(st, S(4), e, T0 + 20000);
  assert.equal(idleClear(st, T0 + IDLE_MS + 1).staged.length, 1);
  assert.equal(idleClear(st, T0 + 20000 + IDLE_MS + 1).staged.length, 0);
});

test('idle clear leaves a panel with nothing pending as it is, and never clears mid-take', () => {
  const e = env();
  const idle = createState();
  assert.equal(idleClear(idle, T0 + 99 * IDLE_MS), idle);
  const { state } = startTake(pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0), e, T0);
  assert.equal(idleClear(state, T0 + 99 * IDLE_MS).staged.length, 1);
});

test('the model never mutates a state it was given', () => {
  const e = env();
  const a = pressDestination(createState(), D(0), e, T0);
  const frozen = JSON.stringify(a);
  pressSource(a, S(4), e, T0);
  clear(a);
  assert.equal(JSON.stringify(a), frozen);
});
