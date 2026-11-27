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

// The problem of the whole list after a save, or ''. The server checks it again.
export function listProblem(list, uids) {
  if (list.length > MAX_TIE_LINES) return `at most ${MAX_TIE_LINES} tie lines`;
  const names = new Set();
  const ends = new Set();
  for (const t of list) {
    const why = tieLineProblem(t, uids);
    if (why) return why;
    const name = String(t.name || '').trim();
    if (!name) return 'every tie line needs a name';
    if ([...name].length > MAX_TIE_NAME) return `tie line name "${name}" must be at most ${MAX_TIE_NAME} characters`;
    if (names.has(fold(name))) return `two tie lines are named "${name}"`;
    names.add(fold(name));
    const key = `${t.from.router}/${t.from.n}/${t.to.router}/${t.to.n}`;
    if (ends.has(key)) return `another tie line has the same two ends ("${name}")`;
    ends.add(key);
  }
  return '';
}

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
