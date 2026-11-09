// A panel's state model: view, stage, arm and Take. No DOM, no clock and no network, so the
// rules test directly (test/panelmodel.test.mjs). Every function returns a new state.
//
// env is what the router says right now: { levels: [UI levels], tally: Map(dst -> Map(level -> src)),
// protectedDsts: Set(dst) }. A cell is { kind, n, levels? } with n the router number and levels a UI
// level subset. Settings: { mode: 'take' | 'instant', multi: bool, levels: [UI levels] | null }.

export const IDLE_MS = 30000;
export const RESULT_MS = 1500;
export const ERROR_MS = 4000;

export function createState(settings = {}) {
  return {
    mode: settings.mode || 'take',
    multi: settings.multi !== false,
    levels: settings.levels || null,
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
  const others = st.multi ? st.staged.filter((x) => x.dst !== dst) : [];
  const staged = cell.n === currentSource(env, st) ? others : [...others, { dst, src: cell.n, levels }];
  return { ...touch(st, now), staged, error: null };
}

export const unstage = (st, dst) => ({ ...st, staged: st.staged.filter((x) => x.dst !== dst) });

export const clear = (st) => ({ ...st, view: null, staged: [], error: null });

const valid = (st, env, x) => !env.protectedDsts.has(x.dst) && x.levels.length > 0;

export const armed = (st, env) => st.mode === 'take' && !st.busy && st.staged.length > 0 && st.staged.every((x) => valid(st, env, x));

export const plan = (st) => st.staged.map((x) => ({ src: x.src, dst: x.dst, levels: x.levels }));

// The routes to send and the busy state, or null when a press must do nothing.
export function startTake(st, env, now) {
  if (!armed(st, env)) return null;
  return { routes: plan(st), state: { ...touch(st, now), busy: true, error: null } };
}

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

export function destinationState(st, n) {
  if (st.staged.some((x) => x.dst === n)) return 'staged';
  return st.view?.dst === n ? 'view' : 'idle';
}

export function resultOf(st, dst, now) {
  const r = st.results[dst];
  return r && now - r.at <= RESULT_MS ? r.result : null;
}

export const errorOf = (st, now) => (st.error && now - st.error.at <= ERROR_MS ? st.error : null);
