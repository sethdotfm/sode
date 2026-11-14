// A panel's state model: view, stage, arm and Take. No DOM, no clock and no network, so the
// rules test directly (test/panelmodel.test.mjs). Every function returns a new state.
//
// env is what the router says right now: { levels: [UI levels], tally: Map(dst -> Map(level -> src)),
// protectedDsts: Set(dst) }. A cell is { kind, n, levels? } with n the router number and levels a UI
// level subset. Settings: { mode: 'take' | 'instant', multi: bool, levels: [UI levels] | null, guard: 'none' | 'hold' | 'double' }.
// A Preset cell is { kind: 'preset', scope, name }; env.presets maps presetKey(scope, name) to the
// preset's items [{ src, dsts, levels }], where levels [] means every level.
// settings.fixedDests (Fixed destination) makes the list the panel's destinations for the life of the page: the
// first one is the view, so Current Source and the Source states read it, and Clear keeps it.
// page is the current Page's id (panelnav.js). The plan is panel-wide, so a Page change leaves the view and staged alone.
import { presetKey, rightsAllow, fixedDestsOf } from './panelrules.js';
import { navTarget } from './panelnav.js';

export const RESULT_MS = 1500;
export const ERROR_MS = 4000;
export const GUARD_DOUBLE_MS = 1500;
export const GUARD_HOLD_MS = 800;

const fixedView = (fixed) => (fixed.length ? { dst: fixed[0], levels: null } : null);

export function createState(settings = {}) {
  const fixed = fixedDestsOf({ settings });
  return {
    mode: settings.mode || 'take',
    multi: settings.multi !== false,
    levels: settings.levels || null,
    resultText: !!settings.resultText,
    guard: settings.guard === 'hold' || settings.guard === 'double' ? settings.guard : 'none',
    guardAt: 0, // first press of a double-press guard
    holdAt: 0, // start of a hold, 0 when none is running
    fixed: fixed.length ? fixed : null,
    view: fixedView(fixed), // { dst, levels }: levels is the destination cell's own set, null for all
    staged: [], // [{ dst, src, levels }] in the order staged
    busy: false,
    results: {}, // dst -> { result, at }
    error: null,
    page: '',
  };
}

export const showPage = (st, page) => ({ ...st, page });

// A Go to Page, Next Page or Previous Page press. One that goes nowhere (greyed) returns st itself.
export function pressPage(st, cell, pages, wrap) {
  const to = navTarget(pages, st.page, cell, wrap);
  return to === null ? st : { ...st, page: to, guardAt: 0, holdAt: 0 };
}

const inSet = (levels, set) => (set ? levels.filter((l) => set.includes(l)) : levels);

// A take sends the levels the panel, the Source cell and the Destination cell all allow.
function shared(st, env, srcLevels, dstLevels) {
  return inSet(inSet(inSet(inSet(env.levels, env.rights?.levels), st.levels), srcLevels), dstLevels);
}

const quiet = (env) => env.rights?.visibility === 'hidden';

// The first route piece the Rights refuse, as an error, or null. Hidden Visibility makes it quiet.
function outsideRights(env, src, dst) {
  const what = !rightsAllow(env.rights, 'dsts', dst) ? 'destination' : !rightsAllow(env.rights, 'sources', src) ? 'source' : '';
  return what ? { code: 'outside-rights', what, src, dst, quiet: quiet(env) } : null;
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

// The router already has the route on every level it sends; a level with no tally yet counts as different.
const routed = (env, x) => x.levels.every((l) => env.tally.get(x.dst)?.get(l) === x.src);

const fail = (st, error, now) => ({ ...st, error: { ...error, at: now } });

// The view and the staged routes have no clock: they stay until a press, a take or Clear ends them.
export function pressDestination(st, cell) {
  const own = st.staged.some((x) => x.dst === cell.n);
  if (st.view?.dst === cell.n && own) return unstage(st, cell.n);
  return { ...st, view: { dst: cell.n, levels: cell.levels || null }, error: null };
}

// A refusal on any one fixed destination refuses the whole press. One that already has the source on every level
// the route sends is left out.
function pressSourceFixed(st, cell, env, now) {
  const routes = [];
  for (const dst of st.fixed) {
    if (env.protectedDsts.has(dst)) return fail(st, { code: 'protected', dst }, now);
    const denied = outsideRights(env, cell.n, dst);
    if (denied) return fail(st, denied, now);
    const levels = shared(st, env, cell.levels, null);
    if (!levels.length) return fail(st, { code: 'no-common-levels', src: cell.n, dst }, now);
    const route = { dst, src: cell.n, levels };
    if (!routed(env, route)) routes.push(route);
  }
  return { ...st, staged: routes, error: null };
}

export function pressSource(st, cell, env, now) {
  if (st.fixed) return pressSourceFixed(st, cell, env, now);
  if (!st.view) return fail(st, { code: 'no-destination' }, now);
  const dst = st.view.dst;
  if (env.protectedDsts.has(dst)) return fail(st, { code: 'protected', dst }, now);
  const denied = outsideRights(env, cell.n, dst);
  if (denied) return fail(st, denied, now);
  const levels = shared(st, env, cell.levels, st.view.levels);
  if (!levels.length) return fail(st, { code: 'no-common-levels', src: cell.n, dst }, now);
  // Instant mode takes on the press, so only a failed route can still be staged: a new press replaces it.
  const others = st.multi && st.mode === 'take' ? st.staged.filter((x) => x.dst !== dst) : [];
  const staged = cell.n === currentSource(env, st) ? others : [...others, { dst, src: cell.n, levels }];
  return { ...st, staged, error: null };
}

// With fixed destinations a Preset keeps only its routes to them.
const presetItems = (st, items) =>
  st.fixed ? items.map((it) => ({ ...it, dsts: it.dsts.filter((d) => st.fixed.includes(d)) })).filter((it) => it.dsts.length) : items;

// A Preset stages every route it holds or none: one refused route refuses the whole preset.
export function pressPreset(st, cell, env, now) {
  const all = env.presets?.get(presetKey(cell.scope, cell.name));
  if (!all?.length) return fail(st, { code: 'no-preset', name: cell.name }, now);
  if (!rightsAllow(env.rights, 'presets', cell.name)) return fail(st, { code: 'outside-rights', what: 'preset', name: cell.name, quiet: quiet(env) }, now);
  const items = presetItems(st, all);
  if (!items.length) return fail(st, { code: 'preset-no-fixed', name: cell.name }, now);
  const dsts = new Set(items.flatMap((it) => it.dsts));
  if (!st.fixed && !st.multi && dsts.size > 1) return fail(st, { code: 'preset-multi', name: cell.name }, now);
  const routes = [];
  for (const it of items) {
    for (const dst of it.dsts) {
      if (env.protectedDsts.has(dst)) return fail(st, { code: 'protected', dst }, now);
      const denied = outsideRights(env, it.src, dst);
      if (denied) return fail(st, denied, now);
      const levels = shared(st, env, inSet(env.levels, it.levels?.length ? it.levels : null), cell.levels);
      if (!levels.length) return fail(st, { code: 'no-common-levels', src: it.src, dst }, now);
      routes.push({ dst, src: it.src, levels });
    }
  }
  const last = new Map(routes.map((x) => [x.dst, x]));
  // The fixed set is staged together, so a press replaces the whole plan, as a Source press does.
  if (st.fixed) return { ...st, staged: [...last.values()], error: null };
  const taken = new Set(routes.map((x) => x.dst));
  const others = st.multi && st.mode === 'take' ? st.staged.filter((x) => !taken.has(x.dst)) : [];
  return { ...st, staged: [...others, ...last.values()], view: { dst: routes[0].dst, levels: null }, error: null };
}

export const unstage = (st, dst) => ({ ...st, staged: st.staged.filter((x) => x.dst !== dst) });

export const clear = (st) => ({ ...st, view: fixedView(st.fixed || []), staged: [], error: null, guardAt: 0, holdAt: 0 });

const valid = (st, env, x) => !env.protectedDsts.has(x.dst) && x.levels.length > 0;

const ready = (st, env) => !st.busy && st.staged.length > 0 && st.staged.every((x) => valid(st, env, x));

export const armed = (st, env) => st.mode === 'take' && ready(st, env);

export const plan = (st) => st.staged.map((x) => ({ src: x.src, dst: x.dst, levels: x.levels }));

// The routes to send and the busy state, or null when a press must do nothing.
export function startTake(st, env) {
  if (!armed(st, env)) return null;
  return { routes: plan(st), state: { ...st, busy: true, error: null } };
}

// Instant mode: the press that staged the route is the Confirm, so the page calls this right after it. A refused press
// is no Confirm: a route a failed take left staged must not go with it.
export function startInstant(st, env) {
  if (st.mode !== 'instant' || st.error || !ready(st, env)) return null;
  return { routes: plan(st), state: { ...st, busy: true, error: null } };
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

// The Undo cell. next is GET /panels/<id>/undo's `undo`: null when this instance has nothing left to undo, undefined
// before an answer or when the key lacks the take scope.
export const undoReady = (st, next) => !st.busy && !!next?.allowed;

export const startUndo = (st, next) => (undoReady(st, next) ? { state: { ...st, busy: true, error: null } } : null);

// As finishTake, but the plan stays: an undo is not what was staged. The status says what was undone, or where it failed.
export function finishUndo(st, reply, now) {
  const results = { ...st.results };
  for (const r of reply.results || []) results[r.dst] = { result: r.result, at: now };
  const failed = (reply.results || []).filter((r) => r.result === 'failed').map((r) => r.dst);
  const routes = (reply.results || []).map((r) => ({ src: r.src, dst: r.dst }));
  const error = failed.length ? { code: 'undo-failed', dsts: failed, at: now } : { code: 'undone', routes, at: now };
  return { ...st, busy: false, results, error };
}

// The router moved: a stage it now satisfies has nothing left to take.
export function applyTally(st, env) {
  const staged = st.staged.filter((x) => !routed(env, x));
  return staged.length === st.staged.length ? st : { ...st, staged };
}

export function sourceState(st, n, env) {
  if (!st.view) return 'idle';
  if (st.fixed && st.staged.some((x) => x.src === n)) return 'pending';
  if (currentSource(env, st) === n) return 'current';
  return st.staged.some((x) => x.dst === st.view.dst && x.src === n) ? 'pending' : 'idle';
}

// pending while the plan holds every route of the preset, so the cell says it is what is staged.
export function presetState(st, cell, env) {
  const all = env.presets?.get(presetKey(cell.scope, cell.name));
  const items = all?.length ? presetItems(st, all) : [];
  if (!items.length) return 'idle';
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

const RESULT_WORD = { connected: 'Connected', already: 'Already', failed: 'Failed' };

// What a destination shows after a take: the icon's result and word, and the text only when the panel turned it on.
export function resultShown(st, dst, now) {
  const result = resultOf(st, dst, now);
  return result ? { result, word: RESULT_WORD[result], text: st.resultText ? RESULT_WORD[result] : '' } : null;
}

export const errorOf = (st, now) => (st.error && now - st.error.at <= ERROR_MS ? st.error : null);

// A live update builds a fresh state; the destination being viewed survives it when the new copy still has that cell.
// A fixed list in the new copy is its own view.
export function carryView(next, prev, cells) {
  if (!prev.view || next.fixed) return next;
  const cell = cells.find((c) => c.kind === 'destination' && c.n === prev.view.dst);
  return cell ? { ...next, view: { dst: cell.n, levels: cell.levels || null } } : next;
}
