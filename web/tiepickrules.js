// The Tie line picker's rules (docs/tielines-design.md, The Tie line picker): its keys, the selection and the take it
// sends. Pure, so node tests run it on the shared plan fixture.
//
// The page never computes paths. The server gives the ranked paths of one route (the 409 tie_line_needed body of
// POST /takes, tieroutes.go). A path has one hop for each router; each hop but the last feeds a tie line. The picker
// shows one step for each of those hops. The choices of a step are the tie lines that the paths give at that step. Only
// the paths that agree with the choices of the steps before it count. A choice stands for its best path: the first
// usable one in rank order, else the first one. So a change on one step takes the best path through it, which
// calculates the later steps again.
//
// State: { route, path, step, marks, protect }. path is the index of the chosen path in route.paths; step is the step
// with focus. marks are the changes that the live update found on the chosen path (refresh). protect holds the ids of
// the tie lines that the person wants Protected after the take (docs/tielines-design.md, Protect in a tie take).

// tieOf is the tie line id that hop h of path p feeds, or '' for the last hop.
const tieOf = (p, h) => p.hops[h]?.tie?.id || '';

// A hop that would change a Protected or Reserved tie line is grey: a person cannot choose it.
export const hopHeld = (h) => !!(h.sends && (h.protect || h.reserve));
// A gone path is the chosen path after a new plan left it out (refresh): the picker shows it, but cannot send it.
export const pathUsable = (p) => !!p && !p.gone && !p.protected && !p.reserved && !p.hops.some(hopHeld);

// The number of steps of path p: one for each tie line.
export const stepsOf = (p) => (p ? p.hops.length - 1 : 0);

// best gives the index of the best path whose first ties are prefix, or -1.
function best(paths, prefix) {
  let first = -1;
  for (let i = 0; i < paths.length; i++) {
    const p = paths[i];
    if (stepsOf(p) < prefix.length || prefix.some((id, h) => tieOf(p, h) !== id)) continue;
    if (pathUsable(p)) return i;
    if (first < 0) first = i;
  }
  return first;
}

const prefixOf = (p, n) => p.hops.slice(0, n).map((h) => h.tie?.id || '');

// pickState opens the picker on the best path of route.
export function pickState(route) {
  return { route, path: Math.max(0, best(route.paths, [])), step: 0, marks: noMarks(), protect: [] };
}

// marks: ties are the tie lines whose row changed. end is the destination's hop. hits are the new Affected
// destinations ("router n"). affected is any other change of the Affected destinations or of the cost.
const noMarks = () => ({ ties: [], end: false, hits: [], affected: false });

// pathKey names a path by its tie lines, upstream first. A new plan keeps the selection on the path with the same key.
export const pathKey = (p) =>
  p.hops
    .slice(0, -1)
    .map((h) => h.tie?.id || '')
    .join(' ');

const endKey = (e) => (e ? `${e.router} ${e.n}` : '');
const chainSign = (c) =>
  c ? JSON.stringify([endKey(c.origin), (c.via || []).map((v) => v.id), !!c.mixed, !!c.nothing, c.unknown || '', !!c.loop, (c.levels || []).map(endKey)]) : '';
// The parts of a hop that a person sees and that the take depends on. The last change time is not one: a change of
// the tie line changes what it carries.
const hopSign = (h) => JSON.stringify([h.router, h.tie?.id || '', !!h.sends, h.protect || '', h.protectSince || '', h.reserve || '', chainSign(h.now)]);

// pathChanges compares two plans of one path: null when nothing changed. Else hops gives the index of each hop that
// changed, and affected tells if its Affected destinations or its cost changed.
export function pathChanges(was, now) {
  const hops = [];
  for (let i = 0; i < Math.max(was.hops.length, now.hops.length); i++) {
    if (!was.hops[i] || !now.hops[i] || hopSign(was.hops[i]) !== hopSign(now.hops[i])) hops.push(i);
  }
  const ends = (p) => JSON.stringify([(p.affected || []).map(endKey), p.others || 0, p.unknown || [], !!p.loop]);
  const affected = ends(was) !== ends(now);
  const cost = was.cost !== now.cost || !!was.protected !== !!now.protected || !!was.reserved !== !!now.reserved || !!was.gone !== !!now.gone;
  return hops.length || affected || cost ? { hops, affected, cost } : null;
}

// refresh puts a new plan of the route into the open picker (docs/tielines-design.md, The Tie line picker, rule 7).
// The selection stays on the path with the same tie lines; it never moves by itself. When the new plan has no such
// path, the old one stays as a gone path. changed: the cost, the Protect state or a link of the chosen path changed,
// so the lockout starts again. The marks then name the changed rows.
export function refresh(st, route) {
  const was = chosen(st);
  let paths = route.paths;
  let i = was ? paths.findIndex((p) => pathKey(p) === pathKey(was)) : -1;
  if (was && i < 0) {
    paths = [...paths, { ...was, gone: true }];
    i = paths.length - 1;
  }
  const next = { ...st, route: { ...route, paths }, path: Math.max(i, 0) };
  const ch = was && pathChanges(was, paths[next.path]);
  if (!ch) return { st: next, changed: false };
  const now = paths[next.path];
  const m = st.marks || noMarks();
  const steps = stepsOf(now);
  const ties = [...m.ties];
  let end = m.end;
  for (const h of ch.hops) {
    if (h >= steps) end = true;
    else if (!ties.includes(now.hops[h].tie.id)) ties.push(now.hops[h].tie.id);
  }
  const old = new Set((was.affected || []).map(endKey));
  const hits = [...m.hits, ...(now.affected || []).map(endKey).filter((k) => !old.has(k) && !m.hits.includes(k))];
  const affected = m.affected || ch.affected || (ch.cost && !ch.hops.length);
  return { st: { ...next, marks: { ties, end, hits, affected } }, changed: true };
}

// reopenState opens the picker again after a take got 409 tie_line_changed. It chooses the path through ids, the tie
// lines of that take, when the plan still has it. changed are the tie lines that moved; fresh are the Affected
// destinations that the person did not see.
// protect: the tie lines that the person wanted Protected; the picker keeps them.
export function reopenState(route, ids, changed = [], fresh = [], protect = []) {
  const st = pickState(route);
  const i = route.paths.findIndex((p) => pathKey(p) === ids.join(' '));
  return { ...st, path: i < 0 ? st.path : i, marks: { ties: [...changed], end: false, hits: fresh.map(endKey), affected: false }, protect: [...protect] };
}

// marked: the mark of the open picker for a tie line id, an Affected destination or the destination's hop.
export const tieMarked = (st, id) => !!st.marks?.ties.includes(id);
export const hitMarked = (st, e) => !!st.marks?.hits.includes(endKey(e));

export const chosen = (st) => st.route.paths[st.path] || null;

// choices gives the choices of step s, in rank order: { tie, name, path, hop, held }. hop is the hop of that choice's
// best path at step s.
export function choices(st, s) {
  const paths = st.route.paths;
  const cur = chosen(st);
  if (!cur || s >= stepsOf(cur)) return [];
  const prefix = prefixOf(cur, s);
  const out = [];
  for (const p of paths) {
    if (stepsOf(p) <= s || prefix.some((id, h) => tieOf(p, h) !== id)) continue;
    const id = tieOf(p, s);
    if (out.some((c) => c.tie === id)) continue;
    const i = best(paths, [...prefix, id]);
    const bp = paths[i];
    out.push({ tie: id, name: bp.hops[s].tie?.name || id, path: i, hop: bp.hops[s], held: !pathUsable(bp), gone: !!bp.gone });
  }
  return out;
}

// choose puts tie line id on step s. The later steps follow the best path through it.
export function choose(st, s, id) {
  const cur = chosen(st);
  if (!cur || s >= stepsOf(cur)) return st;
  const c = choices(st, s).find((x) => x.tie === id);
  if (!c) return st;
  return { ...st, path: c.path, step: s };
}

// moveChoice moves to the next choice of the step with focus that a person can choose (by is 1 or -1). It stops at the
// ends.
export function moveChoice(st, by) {
  const cs = choices(st, st.step);
  const at = cs.findIndex((c) => c.tie === tieOf(chosen(st), st.step));
  for (let i = at + by; i >= 0 && i < cs.length; i += by) {
    if (!cs[i].held) return choose(st, st.step, cs[i].tie);
  }
  return st;
}

export function moveStep(st, by) {
  const n = stepsOf(chosen(st));
  const step = Math.min(Math.max(st.step + by, 0), Math.max(n - 1, 0));
  return step === st.step ? st : { ...st, step };
}

// The keys of docs/tielines-design.md. chord is keys.js chord(e).
const KEYS = {
  j: 'down',
  down: 'down',
  k: 'up',
  up: 'up',
  l: 'next',
  tab: 'next',
  h: 'prev',
  'shift+tab': 'prev',
  p: 'protect',
  enter: 'send',
  escape: 'cancel',
};
export const pickKey = (chord) => KEYS[chord] || null;

// keyStep applies one key to the state: { st, act }. act is 'send', 'cancel' or null. canProtect: the person has the
// Protect right, so P protects; else P does nothing.
export function keyStep(st, chord, canProtect = false) {
  switch (pickKey(chord)) {
    case 'protect':
      return { st: canProtect ? toggleProtect(st, st.step) : st, act: null };
    case 'down':
      return { st: moveChoice(st, 1), act: null };
    case 'up':
      return { st: moveChoice(st, -1), act: null };
    case 'next':
      return { st: moveStep(st, 1), act: null };
    case 'prev':
      return { st: moveStep(st, -1), act: null };
    case 'send':
      return { st, act: 'send' };
    case 'cancel':
      return { st, act: 'cancel' };
  }
  return { st, act: null };
}

// protectable: a person can ask for a Protect on the tie line of hop h. A tie line that has a Protect already cannot
// get a second one.
export const protectable = (h) => !!h?.tie && !h.protect;

// protectOn: the person wants the tie line id Protected after the take.
export const protectOn = (st, id) => !!st.protect?.includes(id);

// toggleProtect asks for a Protect on the tie line of step s, or stops asking for it.
export function toggleProtect(st, s) {
  const p = chosen(st);
  if (!p || s >= stepsOf(p) || !protectable(p.hops[s])) return st;
  const id = p.hops[s].tie.id;
  const list = st.protect || [];
  return { ...st, protect: list.includes(id) ? list.filter((x) => x !== id) : [...list, id] };
}

// heldText says why a hop's tie line is held: who Protected it and when, or the panel that Reserved it. clock formats a
// time as HH:MM. '' when it is free.
export function heldText(h, clock) {
  if (h.protect) return `Protected by ${h.protect}${h.protectSince ? ` at ${clock(h.protectSince)}` : ''}`;
  if (h.reserve) return `Reserved by panel ${h.reserve}`;
  return '';
}

// ready: the chosen path can go.
export const ready = (st) => pathUsable(chosen(st));

const ref = (e) => ({ router: e.router, n: e.n });

// expectOf is what the take expects on a tie line, from the hop's chain now (apitake.go, apiExpect). It is one
// source, or null for nothing. A mixed tie line has one entry for each level.
export function expectOf(now) {
  if (!now || now.nothing) return null;
  if (now.mixed) return (now.levels || []).map((e) => (e ? ref(e) : null));
  return now.origin ? ref(now.origin) : null;
}

// takeRoute is the route that POST /takes gets for the chosen path. It has the tie line of each step with what the
// picker showed on it. It also has the Affected destinations that the person saw.
export function takeRoute(st) {
  const { route } = st;
  const p = chosen(st);
  return {
    src: ref(route.src),
    dst: ref(route.dst),
    levels: route.levels,
    ties: p.hops.slice(0, -1).map((h) => ({ id: h.tie.id, expect: expectOf(h.now), ...(protectOn(st, h.tie.id) && protectable(h) ? { protect: true } : {}) })),
    affected: p.affected.map(ref),
  };
}

// ago gives a count-up time: "45 s", "3 min 05 s", "2 h 07 min".
export function ago(ms) {
  const s = Math.max(0, Math.floor(ms / 1000));
  const two = (n) => String(n).padStart(2, '0');
  if (s < 60) return `${s} s`;
  if (s < 3600) return `${Math.floor(s / 60)} min ${two(s % 60)} s`;
  return `${Math.floor(s / 3600)} h ${two(Math.floor(s / 60) % 60)} min`;
}

// changedText is the last change of a hop's tie line. clock formats a time as HH:MM. Before: Sode was down after that
// time, or found the change at a re-read, so it claims no more than "before HH:MM".
export function changedText(hop, now, clock) {
  if (!hop.changed) return 'No change known';
  if (hop.before) return `Changed before ${clock(hop.changed)}`;
  return `Changed ${ago(now - Date.parse(hop.changed))} ago`;
}

export const TICK_MS = 1000;

// countUp runs tick each second until stop: the last change timers of an open picker. timers gives setInterval and
// clearInterval, so a test can see that a close stops it.
export function countUp(tick, timers = globalThis) {
  let id = timers.setInterval(tick, TICK_MS);
  return {
    running: () => id != null,
    stop() {
      if (id == null) return;
      timers.clearInterval(id);
      id = null;
    },
  };
}
