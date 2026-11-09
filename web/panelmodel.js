// A panel's state model: view, stage, arm and Take. No DOM, no clock and no network, so the
// rules test directly (test/panelmodel.test.mjs). Every function returns a new state.
//
// env is what the router says right now: { levels: [UI levels], tally: Map(dst -> Map(level -> src)),
// protectedDsts: Set(dst) }. A cell is { kind, n, levels? } with n the router number and levels a UI
// level subset. Settings: { mode: 'take' | 'instant', multi: bool, levels: [UI levels] | null, guard: 'none' | 'hold' | 'double' }.
// A Preset cell is { kind: 'preset', scope, name }; env.presets maps presetKey(scope, name) to the
// preset's items [{ src, dsts, levels }], where levels [] means every level.
import { presetKey } from './panelrules.js';

export const IDLE_MS = 30000;
export const RESULT_MS = 1500;
export const ERROR_MS = 4000;
export const GUARD_DOUBLE_MS = 1500;
export const GUARD_HOLD_MS = 800;

export function createState(settings = {}) {
  return {
    mode: settings.mode || 'take',
    multi: settings.multi !== false,
    levels: settings.levels || null,
    guard: settings.guard === 'hold' || settings.guard === 'double' ? settings.guard : 'none',
    guardAt: 0, // first press of a double-press guard
    holdAt: 0, // start of a hold, 0 when none is running
    view: null, // { dst, levels }: levels is the destination cell's own set, null for all
    staged: [], // [{ dst, src, levels }] in the order staged
    busy: false,
    results: {}, // dst -> { result, at }
    error: null,
    touchedAt: 0,
  };
}

const inSet = (levels, set) => (set ? levels.filter((l) => set.includes(l)) : levels);

// A take sends the levels the panel, the Source cell and the Destination cell all allow.
function shared(st, env, srcLevels, dstLevels) {
  return inSet(inSet(inSet(env.levels, st.levels), srcLevels), dstLevels);
}

// The one source on every routed level the panel and the cell send, or -1 when there is none or the levels differ.
export function currentOfDestination(st, env, cell) {
  const row = env.tally.get(cell.n);
  const srcs = inSet(inSet(env.levels, st.levels), cell.levels)
    .map((l) => row?.get(l) ?? -1)
    .filter((s) => s >= 0);
  return srcs.length && srcs.every((s) => s === srcs[0]) ? srcs[0] : -1;
}

export const currentSource = (env, st) => (st.view ? currentOfDestination(st, env, { n: st.view.dst, levels: st.view.levels }) : -1);

const touch = (st, now) => ({ ...st, touchedAt: now });
const fail = (st, error, now) => ({ ...touch(st, now), error: { ...error, at: now } });

export function pressDestination(st, cell, env, now) {
  const own = st.staged.some((x) => x.dst === cell.n);
  if (st.view?.dst === cell.n && own) return unstage(touch(st, now), cell.n);
  return { ...touch(st, now), view: { dst: cell.n, levels: cell.levels || null }, error: null };
}

export function pressSource(st, cell, env, now) {
  if (!st.view) return fail(st, { code: 'no-destination' }, now);
  const dst = st.view.dst;
  if (env.protectedDsts.has(dst)) return fail(st, { code: 'protected', dst }, now);
  const levels = shared(st, env, cell.levels, st.view.levels);
  if (!levels.length) return fail(st, { code: 'no-common-levels', src: cell.n, dst }, now);
  // Instant mode takes on the press, so only a failed route can still be staged: a new press replaces it.
  const others = st.multi && st.mode === 'take' ? st.staged.filter((x) => x.dst !== dst) : [];
  const staged = cell.n === currentSource(env, st) ? others : [...others, { dst, src: cell.n, levels }];
  return { ...touch(st, now), staged, error: null };
}

// A Preset stages every route it holds or none: one refused route refuses the whole preset.
export function pressPreset(st, cell, env, now) {
  const items = env.presets?.get(presetKey(cell.scope, cell.name));
  if (!items?.length) return fail(st, { code: 'no-preset', name: cell.name }, now);
  const dsts = new Set(items.flatMap((it) => it.dsts));
  if (!st.multi && dsts.size > 1) return fail(st, { code: 'preset-multi', name: cell.name }, now);
  const routes = [];
  for (const it of items) {
    for (const dst of it.dsts) {
      if (env.protectedDsts.has(dst)) return fail(st, { code: 'protected', dst }, now);
      const levels = shared(st, env, inSet(env.levels, it.levels?.length ? it.levels : null), cell.levels);
      if (!levels.length) return fail(st, { code: 'no-common-levels', src: it.src, dst }, now);
      routes.push({ dst, src: it.src, levels });
    }
  }
  const taken = new Set(routes.map((x) => x.dst));
  const others = st.multi && st.mode === 'take' ? st.staged.filter((x) => !taken.has(x.dst)) : [];
  const last = new Map(routes.map((x) => [x.dst, x]));
  return { ...touch(st, now), staged: [...others, ...last.values()], view: { dst: routes[0].dst, levels: null }, error: null };
}

export const unstage = (st, dst) => ({ ...st, staged: st.staged.filter((x) => x.dst !== dst) });

export const clear = (st) => ({ ...st, view: null, staged: [], error: null, guardAt: 0, holdAt: 0 });
export const pressClear = (st, now) => touch(clear(st), now);

const valid = (st, env, x) => !env.protectedDsts.has(x.dst) && x.levels.length > 0;

const ready = (st, env) => !st.busy && st.staged.length > 0 && st.staged.every((x) => valid(st, env, x));

export const armed = (st, env) => st.mode === 'take' && ready(st, env);

export const plan = (st) => st.staged.map((x) => ({ src: x.src, dst: x.dst, levels: x.levels }));

// The routes to send and the busy state, or null when a press must do nothing.
export function startTake(st, env, now) {
  if (!armed(st, env)) return null;
  return { routes: plan(st), state: { ...touch(st, now), busy: true, error: null } };
}

// Instant mode: the press that staged the route is the Confirm, so the page calls this right after it.
export function startInstant(st, env, now) {
  if (st.mode !== 'instant' || !ready(st, env)) return null;
  return { routes: plan(st), state: { ...touch(st, now), busy: true, error: null } };
}

// Take guards. none: one press goes. double: a second press within GUARD_DOUBLE_MS goes.
// hold: the page calls guardHeld after GUARD_HOLD_MS and guardCancel on release or leave.
export function guardDown(st, now) {
  if (st.guard === 'double') {
    if (st.guardAt && now - st.guardAt <= GUARD_DOUBLE_MS) return { state: { ...st, guardAt: 0 }, go: true };
    return { state: { ...st, guardAt: now }, go: false };
  }
  if (st.guard === 'hold') return { state: { ...st, holdAt: now }, go: false };
  return { state: st, go: true };
}

export function guardHeld(st, now) {
  const go = st.holdAt > 0 && now - st.holdAt >= GUARD_HOLD_MS;
  return { state: go ? { ...st, holdAt: 0 } : st, go };
}

export const guardCancel = (st) => (st.holdAt ? { ...st, holdAt: 0 } : st);

export const guardWaiting = (st, now) => st.guard === 'double' && st.guardAt > 0 && now - st.guardAt <= GUARD_DOUBLE_MS;

// reply is the /api/v1/takes answer. Connected and Already routes leave the plan; a Failed one stays staged.
export function finishTake(st, reply, now) {
  const results = { ...st.results };
  const gone = new Set();
  for (const r of reply.results || []) {
    results[r.dst] = { result: r.result, at: now };
    if (r.result !== 'failed') gone.add(r.dst);
  }
  return { ...st, busy: false, results, staged: st.staged.filter((x) => !gone.has(x.dst)) };
}

export const failTake = (st, reason, message, now) => ({ ...st, busy: false, error: { code: 'refused', reason, message, at: now } });

// The router moved: a stage it now satisfies has nothing left to take.
export function applyTally(st, env) {
  const probe = (x) => {
    const row = env.tally.get(x.dst);
    return x.levels.every((l) => row?.get(l) === x.src);
  };
  const staged = st.staged.filter((x) => !probe(x));
  return staged.length === st.staged.length ? st : { ...st, staged };
}

export function idleClear(st, now) {
  if (st.busy || (!st.view && !st.staged.length) || now - st.touchedAt <= IDLE_MS) return st;
  return clear(st);
}

export function sourceState(st, n, env) {
  if (!st.view) return 'idle';
  if (currentSource(env, st) === n) return 'current';
  return st.staged.some((x) => x.dst === st.view.dst && x.src === n) ? 'pending' : 'idle';
}

// pending while the plan holds every route of the preset, so the cell says it is what is staged.
export function presetState(st, cell, env) {
  const items = env.presets?.get(presetKey(cell.scope, cell.name));
  if (!items?.length) return 'idle';
  return items.every((it) => it.dsts.every((d) => st.staged.some((x) => x.dst === d && x.src === it.src))) ? 'pending' : 'idle';
}

export function destinationState(st, n) {
  if (st.staged.some((x) => x.dst === n)) return 'staged';
  return st.view?.dst === n ? 'view' : 'idle';
}

export function resultOf(st, dst, now) {
  const r = st.results[dst];
  return r && now - r.at <= RESULT_MS ? r.result : null;
}

export const errorOf = (st, now) => (st.error && now - st.error.at <= ERROR_MS ? st.error : null);
