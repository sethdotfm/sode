// Fixed destination (settings.fixedDests): a panel that always patches to the same destinations (panelmodel.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createState,
  pressSource,
  pressPreset,
  presetState,
  sourceState,
  currentSource,
  clear,
  carryView,
  plan,
  armed,
  errorOf,
  startTake,
  startInstant,
  finishTake,
  resultOf,
} from '../panelmodel.js';
import { presetKey, errorText, namesFrom, drawnCells } from '../panelrules.js';

const tally = (rows) => new Map(Object.entries(rows).map(([d, ls]) => [Number(d), new Map(Object.entries(ls).map(([l, s]) => [Number(l), s]))]));
// Destination 0 has source 5, 1 has 6, 2 has 5.
const env = (over = {}) => ({
  levels: [1, 2],
  tally: tally({ 0: { 1: 5, 2: 5 }, 1: { 1: 6, 2: 6 }, 2: { 1: 5, 2: 5 } }),
  protectedDsts: new Set(),
  presets: new Map(),
  ...over,
});
const S = (n, levels) => ({ kind: 'source', n, levels });
const T0 = 1000;
const fixed = (dsts, over = {}) => createState({ fixedDests: dsts, ...over });

test('a Source press stages a route to the one fixed destination with nothing viewed first', () => {
  const st = pressSource(fixed([1]), S(4), env(), T0);
  assert.equal(errorOf(st, T0), null);
  assert.deepEqual(plan(st), [{ src: 4, dst: 1, levels: [1, 2] }]);
  assert.equal(armed(st, env()), true);
});

test('a Source press stages one route to each of several fixed destinations, and one Take sends them all', () => {
  const st = pressSource(fixed([0, 1]), S(4), env(), T0);
  assert.deepEqual(plan(st), [
    { src: 4, dst: 0, levels: [1, 2] },
    { src: 4, dst: 1, levels: [1, 2] },
  ]);
  const started = startTake(st, env());
  assert.equal(started.routes.length, 2);
});

test('a fixed destination that already has the source is left out', () => {
  const st = pressSource(fixed([0, 1, 2]), S(5), env(), T0);
  assert.deepEqual(plan(st), [{ src: 5, dst: 1, levels: [1, 2] }]);
});

// Destination 3 has source 5 on Level 1 and 6 on Level 2; destination 4 has source 5 on Level 1 and nothing known on Level 2.
const split = (over = {}) => env({ tally: tally({ 3: { 1: 5, 2: 6 }, 4: { 1: 5 } }), ...over });

test('a panel limited to Level 1 leaves out a fixed destination that has the source on Level 1 only', () => {
  const st = pressSource(fixed([3], { levels: [1] }), S(5), split(), T0);
  assert.deepEqual(plan(st), []);
});

test('a Source cell or Rights limited to Level 1 leave out a fixed destination that has the source on Level 1 only', () => {
  assert.deepEqual(plan(pressSource(fixed([3]), S(5, [1]), split(), T0)), []);
  assert.deepEqual(plan(pressSource(fixed([3]), S(5), split({ rights: { levels: [1] } }), T0)), []);
});

test('a fixed destination is staged when the route would change any level it sends, an unknown one included', () => {
  assert.deepEqual(plan(pressSource(fixed([3, 4]), S(5), split(), T0)), [
    { src: 5, dst: 3, levels: [1, 2] },
    { src: 5, dst: 4, levels: [1, 2] },
  ]);
});

test('a Source every fixed destination already has drops the staged routes and arms nothing', () => {
  let st = pressSource(fixed([0, 2]), S(4), env(), T0);
  assert.equal(plan(st).length, 2);
  st = pressSource(st, S(5), env(), T0);
  assert.deepEqual(plan(st), []);
  assert.equal(errorOf(st, T0), null);
  assert.equal(armed(st, env()), false);
});

test('a second Source replaces the staged routes to the fixed destinations', () => {
  const st = pressSource(pressSource(fixed([0, 1]), S(4), env(), T0), S(3), env(), T0);
  assert.deepEqual(
    plan(st).map((x) => [x.dst, x.src]),
    [
      [0, 3],
      [1, 3],
    ],
  );
});

test('a Protected fixed destination refuses the whole Source press, and earlier staged routes stay', () => {
  const before = pressSource(fixed([0, 1]), S(4), env(), T0);
  const st = pressSource(before, S(3), env({ protectedDsts: new Set([1]) }), T0);
  assert.deepEqual(errorOf(st, T0), { code: 'protected', dst: 1, at: T0 });
  assert.deepEqual(st.staged, before.staged);
  assert.equal(pressSource(fixed([0, 1]), S(3), env({ protectedDsts: new Set([1]) }), T0).staged.length, 0);
});

test('a fixed destination outside the Rights refuses the whole Source press', () => {
  const st = pressSource(fixed([0, 1]), S(4), env({ rights: { dsts: [0] } }), T0);
  assert.equal(errorOf(st, T0).code, 'outside-rights');
  assert.equal(errorOf(st, T0).dst, 1);
  assert.equal(st.staged.length, 0);
});

test('a source with no level in common with the panel refuses the whole Source press', () => {
  const st = pressSource(fixed([0, 1], { levels: [1] }), S(4, [2]), env(), T0);
  assert.equal(errorOf(st, T0).code, 'no-common-levels');
  assert.equal(st.staged.length, 0);
});

const presets = new Map([
  [presetKey('everyone', 'Both'), [{ src: 4, dsts: [0, 1], levels: [] }]],
  [
    presetKey('everyone', 'Wide'),
    [
      { src: 3, dsts: [0], levels: [] },
      { src: 8, dsts: [7], levels: [] },
    ],
  ],
  [presetKey('everyone', 'Elsewhere'), [{ src: 3, dsts: [7, 9], levels: [] }]],
]);
const P = (name) => ({ kind: 'preset', scope: 'everyone', name });

test('a Preset whose routes all land on fixed destinations stages them all', () => {
  const st = pressPreset(fixed([0, 1]), P('Both'), env({ presets }), T0);
  assert.deepEqual(plan(st), [
    { src: 4, dst: 0, levels: [1, 2] },
    { src: 4, dst: 1, levels: [1, 2] },
  ]);
  assert.equal(st.view.dst, 0);
});

test('a Preset stages only its routes to fixed destinations and ignores the rest without an error', () => {
  const st = pressPreset(fixed([1, 0]), P('Wide'), env({ presets }), T0);
  assert.equal(errorOf(st, T0), null);
  assert.deepEqual(plan(st), [{ src: 3, dst: 0, levels: [1, 2] }]);
  assert.equal(st.view.dst, 1);
  assert.equal(presetState(st, P('Wide'), env({ presets })), 'pending');
});

test('a Preset with no route to a fixed destination is refused, naming the preset', () => {
  const st = pressPreset(fixed([0, 1]), P('Elsewhere'), env({ presets }), T0);
  assert.deepEqual(errorOf(st, T0), { code: 'preset-no-fixed', name: 'Elsewhere', at: T0 });
  assert.equal(st.staged.length, 0);
  assert.equal(errorText(errorOf(st, T0), namesFrom({})), "Elsewhere has no route to this panel's destinations");
});

test('a refused route among the kept ones refuses the whole Preset', () => {
  assert.equal(pressPreset(fixed([0, 1]), P('Both'), env({ presets, protectedDsts: new Set([1]) }), T0).error.code, 'protected');
  assert.equal(pressPreset(fixed([0, 1]), P('Both'), env({ presets, rights: { dsts: [0] } }), T0).error.code, 'outside-rights');
  assert.equal(pressPreset(fixed([0, 1]), P('Both'), env({ presets, rights: { dsts: [0] } }), T0).staged.length, 0);
});

test('Stage several Destinations has no effect: a Preset to two fixed destinations stages both either way', () => {
  const on = pressPreset(fixed([0, 1], { multi: true }), P('Both'), env({ presets }), T0);
  const off = pressPreset(fixed([0, 1], { multi: false }), P('Both'), env({ presets }), T0);
  assert.deepEqual(plan(off), plan(on));
  assert.equal(plan(off).length, 2);
  assert.deepEqual(plan(pressSource(fixed([0, 1], { multi: false }), S(4), env(), T0)).length, 2);
});

test('Instant mode patches every fixed destination on the Source or Preset press', () => {
  const st = pressSource(fixed([0, 1], { mode: 'instant' }), S(4), env(), T0);
  assert.equal(armed(st, env()), false);
  assert.deepEqual(
    startInstant(st, env()).routes.map((x) => x.dst),
    [0, 1],
  );
  const pre = pressPreset(fixed([0, 1], { mode: 'instant' }), P('Wide'), env({ presets }), T0);
  assert.deepEqual(startInstant(pre, env({ presets })).routes, [{ src: 3, dst: 0, levels: [1, 2] }]);
});

test('Clear drops the staged routes and keeps the fixed destinations', () => {
  const st = clear(pressSource(fixed([1, 0]), S(4), env(), T0));
  assert.equal(st.staged.length, 0);
  assert.equal(st.view.dst, 1);
  assert.equal(currentSource(env(), st), 6);
  assert.deepEqual(
    plan(pressSource(st, S(3), env(), T0)).map((x) => x.dst),
    [1, 0],
  );
});

test('the Current Source and the Source states read the first fixed destination', () => {
  const st = fixed([1, 0]);
  assert.equal(currentSource(env(), st), 6);
  assert.equal(sourceState(st, 6, env()), 'current');
  assert.equal(sourceState(st, 5, env()), 'idle');
  assert.equal(sourceState(pressSource(st, S(5), env(), T0), 5, env()), 'pending');
  // The first one has it already, the second is staged for it: there is still something to take.
  assert.equal(sourceState(pressSource(fixed([0, 1]), S(5), env(), T0), 5, env()), 'pending');
});

test('a Result shows on each fixed destination after a take', () => {
  const { state } = startTake(pressSource(fixed([0, 1]), S(4), env(), T0), env());
  const done = finishTake(
    state,
    {
      results: [
        { src: 4, dst: 0, result: 'connected' },
        { src: 4, dst: 1, result: 'failed' },
      ],
    },
    T0,
  );
  assert.equal(resultOf(done, 0, T0), 'connected');
  assert.equal(resultOf(done, 1, T0), 'failed');
  assert.deepEqual(
    done.staged.map((x) => x.dst),
    [1],
  );
});

test('a live update takes the new fixed list, even when an old Destination cell matches the old view', () => {
  const prev = fixed([1]);
  const next = carryView(fixed([0, 2]), prev, [{ kind: 'destination', n: 1 }]);
  assert.equal(next.view.dst, 0);
  assert.deepEqual(next.fixed, [0, 2]);
});

test('a panel without the setting still needs a viewed destination, and Clear drops the view', () => {
  for (const settings of [{}, { fixedDests: [] }, { fixedDests: null }]) {
    const st = createState(settings);
    assert.equal(st.view, null);
    assert.equal(errorOf(pressSource(st, S(4), env(), T0), T0).code, 'no-destination');
  }
  const viewed = { ...createState(), view: { dst: 0, levels: null } };
  assert.equal(clear(viewed).view, null);
  assert.equal(pressPreset(createState({ multi: false }), P('Both'), env({ presets }), T0).error.code, 'preset-multi');
  assert.equal(pressPreset(createState(), P('Elsewhere'), env({ presets }), T0).view.dst, 7);
});

test('Destination cells are not drawn while the panel has fixed destinations, and come back without them', () => {
  const cells = [
    { r: 1, c: 1, kind: 'destination', n: 0 },
    { r: 1, c: 2, kind: 'source', n: 4 },
    { r: 1, c: 3, kind: 'take' },
  ];
  const kinds = (settings) => drawnCells({ rows: 1, cols: 3, cells, settings }).map((c) => c.kind);
  assert.deepEqual(kinds({ fixedDests: [1] }), ['source', 'take']);
  assert.deepEqual(kinds({ fixedDests: [] }), ['destination', 'source', 'take']);
  assert.deepEqual(kinds(undefined), ['destination', 'source', 'take']);
});

test('a stored list the server would refuse turns the setting off, so the panel still works', async () => {
  const { fixedDestsOf } = await import('../panelrules.js');
  const of = (fixedDests) => fixedDestsOf({ settings: { fixedDests } });
  assert.deepEqual(of([12, 0, 65535]), [12, 0, 65535]);
  for (const bad of [[1.5], ['3'], [-1], [4, 4], [65536], [null], Array.from({ length: 17 }, (_, i) => i), 7]) {
    assert.deepEqual(of(bad), [], JSON.stringify(bad));
  }
  assert.deepEqual(of(Array.from({ length: 16 }, (_, i) => i)).length, 16);
  assert.deepEqual(createState({ fixedDests: [2, 2] }).fixed, null);
});
