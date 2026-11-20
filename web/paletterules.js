// Palette rules: pure, so Node tests load them. The Palette character, last picks, the lists the Palette shows and
// how a query ranks the rows. The rows come from palettecatalogue.js.
import { fuzzyScore } from './fuzzy.js';
import { rowByKey } from './palettecatalogue.js';

// The Palette character is a pref, so its values are a fixed list (prefs.js). Keep in step with PaletteChars in
// internal/config/palette.go (TestPrefTable). Each one passes paletteCharProblem.
export const PALETTE_CHARS = '>~!/;:=#@$%^&*|?<_()[]{}§°¬£€¤';
export const PALETTE_DEFAULT = '>';
// The value for no Palette character. It is not one character, so it never clashes with one.
export const PALETTE_OFF = 'off';

// The patch boxes give these a meaning. + adds a destination and the Levels box reads , and -. A router prefix is a
// letter and a dot (A.23).
const BOX_CHARS = '+,-.';

// '' when ch can be the Palette character, else why not.
export function paletteCharProblem(ch) {
  const all = [...String(ch ?? '')];
  if (all.length !== 1) return 'Use exactly one character.';
  const c = all[0];
  if (/[\p{L}\p{N}\p{M}]/u.test(c)) return 'Use a character that is not a letter or a digit: names and numbers start with them.';
  if (/[\s\p{C}]/u.test(c)) return 'Use a character that you can see.';
  if (BOX_CHARS.includes(c)) return `The patch boxes use ${c}. Use a different character.`;
  return '';
}

// The select's choices: each character as itself, then Off.
export const PALETTE_VALUES = Object.freeze({
  ...Object.fromEntries([...PALETTE_CHARS].map((c) => [c, c])),
  [PALETTE_OFF]: 'No character (no Palette from the boxes)',
});

// The character a pref value gives, or '' for none.
export const paletteCharOf = (v) => (typeof v === 'string' && v !== PALETTE_OFF && !paletteCharProblem(v) ? v : '');

// A key typed into a box opens the Palette when it is the Palette character and the box is empty. The box keeps the
// character when it is not the first one, so "A>B" is plain text.
export const opensPalette = ({ value, key, ctrlKey, metaKey, altKey }, ch) => !!ch && !value && key === ch && !ctrlKey && !metaKey && !altKey;

// Router names that start with ch, for the Settings warning. Each list is [{ name }].
export function namesStartingWith(ch, ...lists) {
  if (!ch) return [];
  const out = [];
  for (const list of lists) for (const it of list || []) if (String(it.name || '').startsWith(ch) && !out.includes(it.name)) out.push(it.name);
  return out;
}

// The warning line under the Palette character, or ''. Settings warns and saves all the same.
export function startsNamesText(ch, names, max = 5) {
  if (!ch || !names.length) return '';
  const shown = names.slice(0, max).join(', ');
  const more = names.length > max ? ` and ${names.length - max} more` : '';
  return `${names.length === 1 ? '1 router name starts' : `${names.length} router names start`} with ${ch}: ${shown}${more}. To find them, type ${ch}${ch} first.`;
}

// Last picks: newest first, at most five. A signed-in user keeps them in Me (Profile.palettePicks), a Guest in This
// browser. They have no Everyone scope.
export const PICKS_MAX = 5;
export const cleanPicks = (v) => (Array.isArray(v) ? [...new Set(v.filter((k) => typeof k === 'string' && k))].slice(0, PICKS_MAX) : []);
export const addPick = (keys, key) => [key, ...cleanPicks(keys).filter((k) => k !== key)].slice(0, PICKS_MAX);

// The categories, in the order the empty Palette lists them. A row's cat is one of these ids. names: the words a
// person types first to search in the category (paletteScope). short: fixed short words, so that a single letter has
// one meaning (p is Panels).
export const CATEGORIES = Object.freeze([
  { id: 'actions', label: 'Actions', names: ['action', 'actions'], short: ['a', 'act'] },
  { id: 'presets', label: 'Presets', names: ['preset', 'presets'], short: ['pre', 'ps'] },
  { id: 'pages', label: 'Pages', names: ['page', 'pages'], short: ['pg'] },
  { id: 'panels', label: 'Panels', names: ['panel', 'panels'], short: ['p'] },
  { id: 'filterSets', label: 'Filter sets', names: ['filter set', 'filter sets', 'filter', 'filters'], short: ['f', 'fs'] },
  { id: 'settings', label: 'Settings', names: ['setting', 'settings'], short: ['s', 'set', 'cfg'] },
]);
const catLabel = (id) => CATEGORIES.find((c) => c.id === id)?.label || id;

// Rows that people name themselves. They and the pages and actions come before settings when the match is close.
const CAT_BOOST = { panels: 50, presets: 50, filterSets: 50, pages: 30, actions: 30 };

// One edit (Damerau: a swap of two letters next to each other counts as one).
function oneEdit(a, b) {
  if (a === b) return true;
  if (Math.abs(a.length - b.length) > 1) return false;
  let i = 0;
  while (i < a.length && i < b.length && a[i] === b[i]) i++;
  if (a.length === b.length) return a.slice(i + 1) === b.slice(i + 1) || (a[i] === b[i + 1] && a[i + 1] === b[i] && a.slice(i + 2) === b.slice(i + 2));
  return a.length > b.length ? a.slice(i + 1) === b.slice(i) : a.slice(i) === b.slice(i + 1);
}

// w is in s in order, and the first letters agree ("pnl" in "panel").
function subsequence(w, s) {
  if (w[0] !== s[0]) return false;
  let j = 0;
  for (const c of s) if (c === w[j]) j++;
  return j >= w.length;
}

// How well a typed word names a category: 3 a prefix, 2 the letters in order, 1 one typo (4 letters or more), 0 none.
function nameStrength(w, names) {
  let best = 0;
  for (const n of names) {
    if (n.startsWith(w)) return 3;
    if (subsequence(w, n.replace(/ /g, ''))) best = Math.max(best, 2);
    else if (w.length >= 4 && oneEdit(w, n.replace(/ /g, ''))) best = Math.max(best, 1);
  }
  return best;
}

// The categories a typed word names, with how well. ids: [] none, one id a clear guess, more ids a tie. strength: 4 a
// short word from the table, 3 a prefix, 2 the letters in order, 1 one typo. A single letter names a category only
// through the table.
function categoryGuess(word) {
  const w = String(word || '').toLowerCase();
  if (!w) return { ids: [], strength: 0 };
  const fixed = CATEGORIES.find((c) => c.short.includes(w));
  if (fixed) return { ids: [fixed.id], strength: 4 };
  if (w.length < 2) return { ids: [], strength: 0 };
  let best = 0;
  let out = [];
  for (const c of CATEGORIES) {
    const s = nameStrength(w, c.names);
    if (s > best) [best, out] = [s, [c.id]];
    else if (s && s === best) out.push(c.id);
  }
  return { ids: out, strength: best };
}
export const guessCategories = (word) => categoryGuess(word).ids;

const USER_CATS = new Set(['panels', 'presets', 'filterSets']);

// A panel, preset or filter set outside cat whose name starts with the whole query. One letter alone never counts.
function paletteNamed(query, rows, cat = '') {
  const q = String(query).trim().toLowerCase().replace(/\s+/g, ' ');
  return q.length > 1 && rows.some((r) => r.cat !== cat && USER_CATS.has(r.cat) && r.label.toLowerCase().replace(/\s+/g, ' ').startsWith(q));
}

// The label as words, each after one space: ' new panel mode' for "New-panel mode".
const wordsOf = (text) =>
  ' ' +
  String(text)
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, ' ')
    .trim();

// A usable row whose label has a word that starts with w ("sig" and Sign in).
const labelStarts = (w, rows) => {
  const q = wordsOf(w);
  return q.length > 1 && rows.some((r) => usable(r) && !r.alias && wordsOf(r.label).includes(q));
};

// A guess by the letters in order or a typo (strength 2 or less) is weak. A row label that starts with the word wins
// over it, so "sig" is Sign in, not "settings". A prefix of a category name and a short word from the table stay.
const weakGuess = (g, word, rows) => g.strength < 3 && labelStarts(word, rows);

// The category that the first word of a query names, or null. The fields: cat, word, rest (the text after the word).
// There is no scope for a tie or for a weak guess. A name of a panel, preset or filter set in a different category can
// start with the whole query. Then there is no scope either (a preset "P 1" for "p 1"). A single letter alone always
// gives its category.
export function paletteScope(query, rows = []) {
  const m = /^\s*(\S+)(?:\s+([\s\S]*))?$/.exec(String(query || ''));
  if (!m) return null;
  const g = categoryGuess(m[1]);
  if (g.ids.length !== 1 || weakGuess(g, m[1], rows)) return null;
  const cat = g.ids[0];
  if (paletteNamed(query, rows, cat)) return null;
  return { cat, word: m[1], rest: String(m[2] || '').trim() };
}

// How well a row matches q: { tier, s }, or null for none. tier 2: the label, or a word of it, starts with q. tier 1:
// another label match. tier 0: the hint (scope, section) or the words only. A higher tier always wins, so a row that
// is obviously not the target never comes first, however often it is used. On the label, the letters-in-order match
// must start at the start of a word. The hint and the words count only when they hold the query as it is, so a long
// hint does not match by chance.
const SUBSEQ_TOP = 500;
function match(row, q) {
  const lname = row.label.toLowerCase();
  let s = fuzzyScore({ num: '', lname }, q);
  if (s >= 0 && s < SUBSEQ_TOP && !lname.split(/[^\p{L}\p{N}]+/u).some((w) => w.startsWith(q[0]))) s = -1;
  const boost = CAT_BOOST[row.cat] || 0;
  if (s >= 0) return { tier: wordsOf(lname).includes(wordsOf(q)) ? 2 : 1, s: 2000 + s + boost };
  const extra = [row.hint, ...(row.words || [])].filter(Boolean).join(' ').toLowerCase();
  const e = extra ? fuzzyScore({ num: '', lname: extra }, q) : -1;
  return e >= SUBSEQ_TOP ? { tier: 0, s: (e + boost) / 10 } : null;
}

// Commonly used first. The rows most people want get a small fixed boost, so they come first among equal matches.
// A row's own use adds up to USE_TOP points: more for more picks (log scale), less as the last pick gets older. It only
// moves a row inside its match tier (match).
export const COMMON = Object.freeze({ 'action:signIn': 20, 'action:signOut': 20, 'action:lock': 20, 'page:settings': 20, 'page:home': 20 });
export const USE_TOP = 120;
const USE_HALF_DAYS = 14;
export const USES_MAX = 50;
export function useBoost(use, now = Date.now()) {
  if (!use || !(use.n > 0)) return 0;
  const days = Math.max(0, (now - (use.at || 0)) / 86_400_000);
  return Math.min(USE_TOP, 40 * Math.log2(1 + use.n)) * 0.5 ** (days / USE_HALF_DAYS);
}
// Uses: { [row key]: { n, at } }, the pick count and the time of the last pick. The USES_MAX newest stay.
export function cleanUses(v) {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return {};
  const ok = Object.entries(v).filter(([k, u]) => k && u && Number.isFinite(u.n) && u.n > 0 && Number.isFinite(u.at));
  ok.sort((x, y) => y[1].at - x[1].at);
  return Object.fromEntries(ok.slice(0, USES_MAX).map(([k, u]) => [k, { n: Math.floor(u.n), at: u.at }]));
}
export function addUse(uses, key, now = Date.now()) {
  const u = cleanUses(uses);
  return cleanUses({ ...u, [key]: { n: (u[key]?.n || 0) + 1, at: now } });
}

// A row outside the Rights or Role is grey: it only explains why it does nothing. The Palette lists one only when it
// is a bookmark with Show in Palette.
const usable = (row) => row?.access?.ok !== false;

// Rows that match q, best first; equal scores keep the catalogue order. Every usable row comes before every grey row,
// so Enter on the first match does something. uses (cleanUses) and now give the use boost.
export function rankRows(rows, q, n = 50, { uses = {}, now = Date.now() } = {}) {
  q = String(q || '')
    .trim()
    .toLowerCase();
  const scored = rows
    .map((r, i) => {
      if (!q) return [0, 0, i, r];
      const m = match(r, q);
      return m ? [m.tier, m.s + (COMMON[r.key] || 0) + useBoost(uses[r.key], now), i, r] : null;
    })
    .filter(Boolean);
  return scored
    .sort((a, b) => usable(b[3]) - usable(a[3]) || b[0] - a[0] || b[1] - a[1] || a[2] - b[2])
    .slice(0, n)
    .map(([, , , r]) => r);
}

// The Settings sections in the order of their first setting, as [{ id, label, count }].
export function settingsSections(rows) {
  const out = [];
  for (const r of rows) {
    if (r.cat !== 'settings') continue;
    const sec = out.find((s) => s.id === r.target.section);
    if (sec) sec.count++;
    else out.push({ id: r.target.section, label: r.target.sectionTitle, count: 1 });
  }
  return out;
}

// What the Palette lists. path is [] (the top), [category] or ['settings', section]. The items:
// - { type: 'row', row }: a catalogue row.
// - { type: 'cat', id, label, count }: a category (top) or a Settings section (path ['settings']).
// - { type: 'head', label }: a heading line, never picked.
// The empty top lists Home (on every page but Home), the bookmarks with Show in Palette (marks, as rows), the last
// picks, then the categories. A query at the top searches every row (searchView). Grey rows show only as marks.
export function paletteView(rows, { path = [], query = '', last = [], marks = [], uses = {}, now = Date.now() } = {}) {
  const use = { uses, now };
  const q = String(query || '').trim();
  // An alias (a Settings button that an action row does) shows only as a bookmark, so a search finds one Sign out.
  rows = rows.filter((r) => (usable(r) && !r.alias) || marks.includes(r));
  const rowItems = (list) => list.map((row) => ({ type: 'row', row }));
  if (!path.length) {
    if (q) return searchView(rows, String(query), use);
    const home = rowByKey(rows, 'page:home');
    const marked = marks.filter((r) => r !== home);
    const picks = cleanPicks(last)
      .map((k) => rowByKey(rows, k))
      .filter((r) => r && r !== home && !marked.includes(r));
    const cats = CATEGORIES.map((c) => ({ type: 'cat', id: c.id, label: c.label, count: rows.filter((r) => r.cat === c.id).length })).filter((c) => c.count);
    return [
      ...(home ? rowItems([home]) : []),
      ...(marked.length ? [{ type: 'head', label: 'Bookmarks' }, ...rowItems(marked)] : []),
      ...(picks.length ? [{ type: 'head', label: 'Last picks' }, ...rowItems(picks)] : []),
      { type: 'head', label: 'Categories' },
      ...cats,
    ];
  }
  const [cat, section] = path;
  let list = rows.filter((r) => r.cat === cat);
  if (cat === 'settings' && !section && !q) return settingsSections(list).map((s) => ({ type: 'cat', ...s }));
  if (section) list = list.filter((r) => r.target.section === section);
  return rowItems(rankRows(list, q, Infinity, use));
}

// A search at the top. When the first word names a category (paletteScope), its rows come first. The word alone lists
// all of them, else the rows that match the rest come first. The other matches of the whole query follow under a
// heading, so a wrong guess hides nothing. When no row of the category matches, the list says so and shows every
// match. A word alone that names two categories (a tie) lists the rows of both first.
function searchView(rows, query, use) {
  const rowItems = (list) => list.map((row) => ({ type: 'row', row }));
  const sc = paletteScope(query, rows);
  const word = /^\s*(\S+)\s*$/.exec(query)?.[1];
  const g = !sc && word ? categoryGuess(word) : { ids: [] };
  const tie = g.ids.length > 1 && !weakGuess(g, word, rows) ? g.ids : [];
  const cats = sc ? [sc.cat] : tie.length > 1 && !paletteNamed(query, rows) ? tie : [];
  if (!cats.length) return rowItems(rankRows(rows, query, 50, use));
  const inCats = rows.filter((r) => cats.includes(r.cat));
  const first = sc?.rest ? rankRows(inCats, sc.rest, 50, use) : inCats.slice(0, 50);
  const others = rankRows(
    rows.filter((r) => !cats.includes(r.cat)),
    query,
    50,
    use,
  );
  if (!first.length) return [{ type: 'head', label: `No match in ${cats.map(catLabel).join(' or ')}` }, ...rowItems(others)];
  return [...rowItems(first), ...(others.length ? [{ type: 'head', label: 'Other matches' }, ...rowItems(others)] : [])];
}

// The line above the list: where the Palette is, or the category that a search shows first.
export function pathText(path, rows, query = '') {
  if (!path.length) {
    const sc = String(query).trim() ? paletteScope(query, rows) : null;
    return sc ? `${catLabel(sc.cat)} first` : '';
  }
  const [cat, section] = path;
  const sec = section && settingsSections(rows).find((s) => s.id === section);
  return [catLabel(cat), sec?.label].filter(Boolean).join(' › ');
}

// The next row index for a step, skipping heading lines; -1 when nothing can be picked.
export function stepIndex(items, from, by) {
  const ok = (i) => items[i] && items[i].type !== 'head';
  const n = items.length;
  if (!items.some((_, i) => ok(i))) return -1;
  let i = from;
  for (let k = 0; k < n; k++) {
    i = (((i + by) % n) + n) % n;
    if (ok(i)) return i;
  }
  return -1;
}
// Where the cursor starts: the first row that does something, never a grey one while another can be picked.
export function firstIndex(items) {
  const i = items.findIndex((it) => it.type === 'cat' || (it.type === 'row' && usable(it.row)));
  return i >= 0 ? i : stepIndex(items, -1, 1);
}
