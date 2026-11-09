// Preset, Clear and Current Destination cells, Instant mode and the Take guards (panelmodel.js, panelrules.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createState,
  pressDestination,
  pressSource,
  pressPreset,
  pressClear,
  startInstant,
  startTake,
  armed,
  presetState,
  currentSource,
  errorOf,
  guardDown,
  guardHeld,
  guardCancel,
  guardWaiting,
  GUARD_DOUBLE_MS,
  GUARD_HOLD_MS,
} from '../panelmodel.js';
import { cellsIn, envFrom, namesFrom, errorText, presetKey } from '../panelrules.js';

const tally = (rows) => new Map(Object.entries(rows).map(([d, ls]) => [Number(d), new Map(Object.entries(ls).map(([l, s]) => [Number(l), s]))]));
const env = (over = {}) => ({
  levels: [1, 2],
  tally: tally({ 0: { 1: 5, 2: 5 }, 1: { 1: 6, 2: 7 } }),
  protectedDsts: new Set(),
  presets: new Map([
    [presetKey('everyone', 'Studio A'), [{ src: 4, dsts: [0, 1], levels: [] }]],
    [presetKey('everyone', 'Video'), [{ src: 3, dsts: [0], levels: [1] }]],
    [
      presetKey('me', 'Two'),
      [
        { src: 3, dsts: [0], levels: [] },
        { src: 8, dsts: [1], levels: [] },
      ],
    ],
  ]),
  ...over,
});
const P = (name, scope = 'everyone', levels) => ({ kind: 'preset', scope, name, levels });
const D = (n) => ({ kind: 'destination', n });
const S = (n) => ({ kind: 'source', n });
const T0 = 1000;

test('a Preset cell stages its routes for every destination it names and views the first one', () => {
  const st = pressPreset(createState(), P('Studio A'), env(), T0);
  assert.deepEqual(st.staged, [
    { dst: 0, src: 4, levels: [1, 2] },
    { dst: 1, src: 4, levels: [1, 2] },
  ]);
  assert.equal(st.view.dst, 0);
  assert.equal(armed(st, env()), true);
});

test('a Preset item with its own levels stages only those, within the panel levels', () => {
  assert.deepEqual(pressPreset(createState(), P('Video'), env(), T0).staged, [{ dst: 0, src: 3, levels: [1] }]);
  assert.equal(pressPreset(createState({ levels: [2] }), P('Video'), env(), T0).error.code, 'no-common-levels');
});

test('a Preset keeps what other destinations already have staged, and replaces its own', () => {
  const e = env();
  let st = pressSource(pressDestination(createState(), D(1), e, T0), S(9), e, T0);
  st = pressPreset(st, P('Video'), e, T0);
  assert.deepEqual(
    st.staged.map((x) => [x.dst, x.src]),
    [
      [1, 9],
      [0, 3],
    ],
  );
  st = pressPreset(st, P('Studio A'), e, T0);
  assert.deepEqual(
    st.staged.map((x) => [x.dst, x.src]),
    [
      [0, 4],
      [1, 4],
    ],
  );
});

test('a Preset aimed at a Protected destination is refused whole, with nothing staged', () => {
  const e = env({ protectedDsts: new Set([1]) });
  const st = pressPreset(createState(), P('Studio A'), e, T0);
  assert.equal(st.staged.length, 0);
  assert.deepEqual(errorOf(st, T0), { code: 'protected', dst: 1, at: T0 });
});

test('a Preset the router or the caller does not have is refused', () => {
  const st = pressPreset(createState(), P('Gone'), env(), T0);
  assert.equal(st.staged.length, 0);
  assert.equal(errorOf(st, T0).code, 'no-preset');
});

test('with multi-destination off a Preset may name one destination only, and replaces the plan', () => {
  const one = createState({ multi: false });
  assert.equal(pressPreset(one, P('Studio A'), env(), T0).error.code, 'preset-multi');
  assert.equal(pressPreset(one, P('Two', 'me'), env(), T0).error.code, 'preset-multi');
  const e = env();
  const staged = pressSource(pressDestination(one, D(1), e, T0), S(9), e, T0);
  const ok = pressPreset(staged, P('Video'), e, T0);
  assert.deepEqual(
    ok.staged.map((x) => x.dst),
    [0],
  );
});

test('Preset cells read pending while the plan holds all of their routes', () => {
  const e = env();
  const st = pressPreset(createState(), P('Studio A'), e, T0);
  assert.equal(presetState(st, P('Studio A'), e), 'pending');
  assert.equal(presetState(st, P('Video'), e), 'idle');
  assert.equal(presetState(createState(), P('Studio A'), e), 'idle');
});

test('a Preset views its destination, so Current Destination shows what that destination has now', () => {
  const e = env();
  const st = pressPreset(createState(), P('Video'), e, T0);
  assert.equal(currentSource(e, st), 5);
});

test('Clear drops the view, the plan and the error', () => {
  const e = env();
  const st = pressClear(pressPreset(createState(), P('Studio A'), e, T0), T0);
  assert.equal(st.view, null);
  assert.equal(st.staged.length, 0);
  assert.equal(st.error, null);
});

test('Instant mode: pressing a Source takes at once, and a Preset does too', () => {
  const e = env();
  const base = createState({ mode: 'instant' });
  const a = startInstant(pressSource(pressDestination(base, D(0), e, T0), S(4), e, T0), e, T0);
  assert.deepEqual(a.routes, [{ src: 4, dst: 0, levels: [1, 2] }]);
  assert.equal(a.state.busy, true);
  const b = startInstant(pressPreset(base, P('Studio A'), e, T0), e, T0);
  assert.equal(b.routes.length, 2);
});

test('Instant mode holds one route at a time, so a second Source replaces the first', () => {
  const e = env();
  let st = pressDestination(createState({ mode: 'instant' }), D(0), e, T0);
  st = pressSource(st, S(4), e, T0);
  st = pressDestination(st, D(1), e, T0);
  st = pressSource(st, S(9), e, T0);
  assert.deepEqual(
    st.staged.map((x) => [x.dst, x.src]),
    [[1, 9]],
  );
});

test('startInstant sends nothing in Take mode, on an empty plan, or while busy', () => {
  const e = env();
  const staged = pressSource(pressDestination(createState(), D(0), e, T0), S(4), e, T0);
  assert.equal(startInstant(staged, e, T0), null);
  assert.equal(startInstant(createState({ mode: 'instant' }), e, T0), null);
  const inst = pressSource(pressDestination(createState({ mode: 'instant' }), D(0), e, T0), S(4), e, T0);
  assert.equal(startInstant({ ...inst, busy: true }, e, T0), null);
});

test('startTake still does nothing in Instant mode', () => {
  const e = env();
  const inst = pressSource(pressDestination(createState({ mode: 'instant' }), D(0), e, T0), S(4), e, T0);
  assert.equal(startTake(inst, e, T0), null);
});

test('no guard: one press goes', () => {
  assert.equal(guardDown(createState(), T0).go, true);
});

test('double-press guard: the first press waits, a second within the window goes, a late one waits again', () => {
  const st = createState({ guard: 'double' });
  const first = guardDown(st, T0);
  assert.equal(first.go, false);
  assert.equal(guardWaiting(first.state, T0 + 10), true);
  assert.equal(guardWaiting(first.state, T0 + GUARD_DOUBLE_MS + 1), false);
  assert.equal(guardDown(first.state, T0 + GUARD_DOUBLE_MS).go, true);
  assert.equal(guardDown(first.state, T0 + GUARD_DOUBLE_MS + 1).go, false);
  assert.equal(guardWaiting(guardDown(first.state, T0 + GUARD_DOUBLE_MS).state, T0 + GUARD_DOUBLE_MS), false);
});

test('hold guard: pressing starts the hold, holding long enough goes, letting go early cancels', () => {
  const st = createState({ guard: 'hold' });
  const down = guardDown(st, T0);
  assert.equal(down.go, false);
  assert.equal(guardHeld(down.state, T0 + GUARD_HOLD_MS - 1).go, false);
  assert.equal(guardHeld(down.state, T0 + GUARD_HOLD_MS).go, true);
  assert.equal(guardHeld(guardCancel(down.state), T0 + GUARD_HOLD_MS).go, false);
  assert.equal(guardHeld(st, T0 + GUARD_HOLD_MS).go, false);
});

test('an unknown guard setting means no guard', () => {
  assert.equal(createState({ guard: 'sideways' }).guard, 'none');
  assert.equal(guardDown(createState({ guard: 'sideways' }), T0).go, true);
});

test('the new cell kinds are drawn when whole: Preset needs a scope and a name, Clear and Current need nothing', () => {
  const panel = {
    rows: 1,
    cols: 8,
    cells: [
      { r: 1, c: 1, kind: 'preset', scope: 'everyone', name: 'Studio A' },
      { r: 1, c: 2, kind: 'preset', scope: 'me', name: 'Mine' },
      { r: 1, c: 3, kind: 'preset', scope: 'everyone' },
      { r: 1, c: 4, kind: 'preset', scope: 'this browser', name: 'X' },
      { r: 1, c: 5, kind: 'clear' },
      { r: 1, c: 6, cs: 2, kind: 'current' },
    ],
  };
  assert.deepEqual(
    cellsIn(panel).map((c) => [c.kind, c.c, c.cs]),
    [
      ['preset', 1, 1],
      ['preset', 2, 1],
      ['clear', 5, 1],
      ['current', 6, 2],
    ],
  );
});

test('names: Clear and Current Destination have default captions, a Preset shows its name, a caption wins', () => {
  const n = namesFrom({ sources: [], destinations: [] });
  assert.equal(n.of({ kind: 'clear' }), 'Clear');
  assert.equal(n.of({ kind: 'current' }), 'Currently');
  assert.equal(n.of({ kind: 'preset', name: 'Studio A' }), 'Studio A');
  assert.equal(n.of({ kind: 'preset', name: 'Studio A', caption: 'A' }), 'A');
  assert.equal(n.of({ kind: 'clear', caption: 'Reset' }), 'Reset');
});

test('envFrom keys the presets by scope and name', () => {
  const e = envFrom({ levels: [], routes: [] }, { protected: [] }, { presets: [{ scope: 'me', name: 'Two', items: [{ src: 1, dsts: [2], levels: [] }] }] });
  assert.deepEqual(e.presets.get(presetKey('me', 'Two')), [{ src: 1, dsts: [2], levels: [] }]);
  assert.equal(envFrom({}, {}).presets.size, 0);
});

test('error text for the new refusals', () => {
  const n = namesFrom({});
  assert.equal(errorText({ code: 'no-preset' }, n), 'That preset is not available to this panel');
  assert.equal(errorText({ code: 'preset-multi' }, n), 'This panel stages one destination at a time, and that preset has more');
});
