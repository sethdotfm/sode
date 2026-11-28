// Settings > Routers > Tie lines: pure rules (Node-tested). A tie line is {id, name, from: {router, n}, to: {router, n}}:
// from is a destination, to is a source on another router. JS twin of config.TieLineProblem (internal/config/tielines.go);
// TestTieLineCases and tielinerules.test.mjs run the shared cases in web/test/tieline_cases.json.

export const MAX_TIE_LINES = 200;
export const MAX_TIE_NAME = 32;

// Why Sode refuses t on its own, or ''. uids: the UID of each router.
export function tieLineProblem(t, uids) {
  for (const [end, side] of [
    [t.from, 'destination'],
    [t.to, 'source'],
  ]) {
    if (!end?.router) return `a tie line needs the router of its ${side}`;
    if (!uids.includes(end.router)) return `a tie line names a router that Sode does not have (${end.router})`;
    if (!Number.isInteger(end.n) || end.n < 0 || end.n > 0xffff) return `a tie line needs a ${side} number from the router`;
  }
  if (t.from.router === t.to.router) return 'a tie line goes from one router to a different router';
  return '';
}

const fold = (s) =>
  String(s || '')
    .trim()
    .toLowerCase();

// The problem of the whole list after a save, or ''. The server checks it again. The check for a cable that is in the
// list already comes before the name checks (R-20). Thus a second add of the same ends names the cable, not the name.
// shown(end) gives an end as the page shows it ("A.5"). changed: the id of the tie line that the save adds or edits;
// the words name the other one. A new tie line has no id and is last in the list.
export function listProblem(list, uids, { shown = (e) => `${e.router}.${e.n}`, changed = '' } = {}) {
  if (list.length > MAX_TIE_LINES) return `at most ${MAX_TIE_LINES} tie lines`;
  for (const t of list) {
    const why = tieLineProblem(t, uids);
    if (why) return why;
  }
  const ends = new Map();
  for (const t of list) {
    const key = cableKey(t);
    const was = ends.get(key);
    if (!was) {
      ends.set(key, t);
      continue;
    }
    const other = changed && was.id === changed ? t : was;
    return `${shown(t.from)} already feeds ${shown(t.to)} as ${String(other.name || '').trim() || 'a tie line with no name'}`;
  }
  const names = new Set();
  for (const t of list) {
    const name = String(t.name || '').trim();
    if (!name) return 'every tie line needs a name';
    if ([...name].length > MAX_TIE_NAME) return `tie line name "${name}" must be at most ${MAX_TIE_NAME} characters`;
    if (names.has(fold(name))) return `another tie line has the name "${name}"`;
    names.add(fold(name));
  }
  return '';
}

const cableKey = (t) => `${t.from.router}/${t.from.n}/${t.to.router}/${t.to.n}`;

// Sode fills the name from the downstream source name until a person types a different name. state: {name, auto}, where
// auto is the name that Sode filled last. Sode also fills a blank name.
export function fillName(state, srcName) {
  const auto = String(srcName || '').trim();
  const name = !state.name.trim() || state.name === state.auto ? auto : state.name;
  return { name, auto };
}

// Adds t, or puts it in place of the tie line with its id.
export function upsert(list, t) {
  const i = t.id ? list.findIndex((x) => x.id === t.id) : -1;
  if (i < 0) return [...list, t];
  return list.map((x, j) => (j === i ? t : x));
}

export const without = (list, id) => list.filter((x) => x.id !== id);

// A typed number as shown is the router's number plus its offset. NaN when it is not a whole number.
export function wireNumber(text, offset) {
  const s = String(text ?? '').trim();
  if (!/^\d+$/.test(s)) return NaN;
  return Number(s) - offset;
}

// The other router to offer as the downstream end: the first one that is not from.
export const otherRouter = (uids, from) => uids.find((u) => u !== from) || '';

// The Name moved text of t, one line for each marked end, or [] (docs/tielines-design.md, Tie lines rule 8). A mark
// is {kind, ref: the end as stored, now: the name at its number now}. shown(ref) gives the end as "B.2".
export function movedLines(t, shown) {
  const marks = Array.isArray(t?.moved) ? t.moved : [];
  return marks.map((m) => `${shown(m.ref || {})} was ${m.ref?.name || 'no name'}, now ${m.now || 'no name'}`);
}

// removeQuestion: the Confirm's words for Remove of tie line t, whose ends read as ends ("A.5 MV 1 feeds B.7 CAM 1").
// presets: Everyone's presets on the page. The ones whose items store t's id are named. A user's own presets are not on
// the page, so the Confirm always says what happens to a preset that stores it (docs/tielines-design.md, Presets).
export function removeQuestion(t, ends, presets = []) {
  const uses = presets.filter((p) => (p?.items || []).some((it) => (it?.ties || []).includes(t.id))).map((p) => p.name);
  const named = uses.length ? ` Everyone's presets that store it: ${uses.join(', ')}.` : '';
  return `Remove tie line ${t.name}? ${ends}. A preset that stores this tie line becomes Broken.${named}`;
}

// The table (settingstielines.js). A row is { id, name, from, to, moved }, where each end is
// { router, n, num: "A.5" as shown, name: the router name now }. A router is { uid, prefix, title } in config order.

// With more routers than this, the router filter is a select, not a row of buttons (as in the Protects list).
export const ROUTER_BUTTONS_MAX = 6;
export const ALL_ROUTERS = '*';

const bare = (num) => (num.includes('.') ? num.slice(num.indexOf('.') + 1) : num);

// The find box accepts part of the Tie line name, part of an end ("A.5 MV 1"), or a number as shown (A.5, 5). Case
// does not count.
export function findMatch(row, q) {
  const s = String(q || '')
    .trim()
    .toLowerCase();
  if (!s) return true;
  if (fold(row.name).includes(s)) return true;
  return [row.from, row.to].some((e) => `${e.num} ${e.name || ''}`.toLowerCase().includes(s) || s === bare(e.num));
}

// f: { q, router }. router is a router UID or ALL_ROUTERS; a tie line matches a router at either end.
export const matches = (row, f) => (f.router === ALL_ROUTERS || row.from.router === f.router || row.to.router === f.router) && findMatch(row, f.q);

export const filtered = (f) => !!String(f.q || '').trim() || f.router !== ALL_ROUTERS;

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

// The groups to draw: one for each router pair, in router order (the From router, then the To router). Rows sort by
// the number of the destination. A pair with no row that matches f is left out.
export function pairGroups(rows, routers, f) {
  const at = new Map(routers.map((r, i) => [r.uid, i]));
  const by = new Map();
  for (const r of rows) {
    const key = `${r.from.router}>${r.to.router}`;
    if (!by.has(key)) by.set(key, { key, from: r.from.router, to: r.to.router, all: [] });
    by.get(key).all.push(r);
  }
  const rank = (uid) => at.get(uid) ?? routers.length;
  return [...by.values()]
    .sort((a, b) => rank(a.from) - rank(b.from) || rank(a.to) - rank(b.to))
    .map((g) => ({
      key: g.key,
      from: routers.find((r) => r.uid === g.from) || { uid: g.from, prefix: '?', title: '' },
      to: routers.find((r) => r.uid === g.to) || { uid: g.to, prefix: '?', title: '' },
      total: g.all.length,
      rows: g.all.filter((r) => matches(r, f)).sort((a, b) => a.from.n - b.from.n || a.to.n - b.to.n),
    }))
    .filter((g) => g.rows.length);
}

// A group heading: "A to B, 4". With fewer rows shown than the pair has: "A to B, 2 of 4".
export const pairHeading = (g) => `${g.from.prefix} to ${g.to.prefix}, ${g.rows.length < g.total ? `${g.rows.length} of ${g.total}` : g.total}`;

// "6 tie lines between 3 routers." With a filter on: "Shows 2 of 6."
export function countLine(rows, f) {
  if (!rows.length) return 'No tie lines.';
  if (filtered(f)) return `Shows ${rows.filter((r) => matches(r, f)).length} of ${rows.length}.`;
  const used = new Set(rows.flatMap((r) => [r.from.router, r.to.router]));
  return `${plural(rows.length, 'tie line', 'tie lines')} between ${plural(used.size, 'router', 'routers')}.`;
}

// The count on each router button, in the find of f.
export function routerCounts(rows, routers, f) {
  const out = { [ALL_ROUTERS]: rows.filter((r) => matches(r, { ...f, router: ALL_ROUTERS })).length };
  for (const rt of routers) out[rt.uid] = rows.filter((r) => matches(r, { ...f, router: rt.uid })).length;
  return out;
}

// What the upstream destination of a tie line carries now. levels: its tally (level → source, a negative source is
// nothing), or undefined when Sode has none. { kind: 'unknown' | 'nothing' | 'one' | 'mixed', src }.
export function carries(levels) {
  const srcs = levels ? [...new Set(levels.values())] : [];
  if (!srcs.length) return { kind: 'unknown' };
  if (srcs.length > 1) return { kind: 'mixed' };
  return srcs[0] < 0 ? { kind: 'nothing' } : { kind: 'one', src: srcs[0] };
}

// A match of a find box (core.js boxSearch) as a tie line end. An item of the first router has no router; firstUid
// is the first router's own UID, which a stored tie line uses.
export const endOfItem = (it, firstUid) => (it ? { router: it.router || firstUid, n: it.n } : null);
