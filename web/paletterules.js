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

// Rows that people name themselves. They and the pages and actions come before settings when the match is close. The
// boost of an action or a page is more than one strength step (STRENGTH), so "theme" finds the theme action before the
// Theme setting.
const CAT_BOOST = { panels: 250, presets: 250, filterSets: 250, pages: 150, actions: 150 };

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
    // The letters in order count only for a short form with no vowel after the first letter (pnl, stg). A real word
    // such as "sing" holds the letters of settings by chance.
    if (subsequence(w, n.replace(/ /g, '')) && !/[aeiouy]/.test(w.slice(1))) best = Math.max(best, 2);
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

// How well a text holds a query, best first. A match must start at the start of a word. Only LOOSE lets the letters
// skip, and a loose match is never the row that Enter picks first (paletteView).
export const STRENGTH = Object.freeze({ exact: 1000, start: 900, word: 800, wordStart: 700, allWords: 600, inside: 300, hint: 200, loose: 100 });

// The words of a text with no space or mark, from each word on: "re-read names" gives rereadnames, readnames, names.
const tails = (words) => words.map((_, i) => words.slice(i).join(''));

// The strength of text for q (in lower case), or 0. The marks in a label do not count, so "reread" finds "Re-read".
function textStrength(text, q) {
  const raw = String(text || '').toLowerCase();
  if (!raw) return 0;
  if (raw === q) return STRENGTH.exact;
  const t = wordsOf(raw);
  const w = wordsOf(q);
  if (w.trim()) {
    if (t === w) return STRENGTH.exact;
    if (raw.startsWith(q) || t.startsWith(w)) return STRENGTH.start;
    if ((t + ' ').includes(w + ' ')) return STRENGTH.word;
    if (t.includes(w)) return STRENGTH.wordStart;
    const qw = w.trim().split(' ');
    const tw = t.trim().split(' ');
    if (qw.join('').length > 2 && tails(tw).some((x) => x.startsWith(qw.join('')))) return STRENGTH.wordStart;
    if (qw.length > 1 && qw.every((x) => tw.some((y) => y.startsWith(x)))) return STRENGTH.allWords;
  } else if (raw.startsWith(q)) return STRENGTH.start;
  return raw.includes(q) ? STRENGTH.inside : 0;
}

// w is in s in order, and the first letter of w starts a word of s: a loose match ("ip" in "Export & Import").
const looseIn = (q, s) => fuzzyScore({ num: '', lname: s }, q) >= 0 && s.split(/[^\p{L}\p{N}]+/u).some((x) => x.startsWith(q[0]));

// A row that deletes or resets something ranks after every other match and is never the row that Enter picks first.
// The catalogue marks the buttons with a danger confirm (palettecatalogue.js); the label marks the rest.
export const isDanger = (row) => !!row?.danger || /^(clear|delete|remove|reset)\b/i.test(String(row?.label || ''));

// How well a row matches q: { strength, loose }, or null for none. The strength already holds the category boost.
// - The label gives the full STRENGTH.
// - The words of an action, a page or a setting count almost as the label (50 less). The words of a panel, preset or
//   filter set are only the category word ("preset"), so they count STRENGTH.word at most less 300.
// - The hint (scope, section) counts STRENGTH.hint, and only at the start of a word.
// - A loose match of the label is the last chance.
function match(row, q) {
  const boost = CAT_BOOST[row.cat] || 0;
  let s = textStrength(row.label, q);
  for (const w of row.words || []) {
    const ws = textStrength(w, q);
    if (ws >= STRENGTH.wordStart) s = Math.max(s, USER_CATS.has(row.cat) ? Math.min(ws, STRENGTH.word) - 300 : ws - 50);
  }
  if (!s && row.hint && wordsOf(row.hint).includes(wordsOf(q))) s = STRENGTH.hint;
  if (s) return { strength: s + boost, loose: false };
  return looseIn(q, row.label.toLowerCase()) ? { strength: STRENGTH.loose + boost, loose: true } : null;
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

// Rows that match q, best first, as [{ row, weak }]. weak: a loose match or a danger row (isDanger). Such a row is never
// the row that Enter picks first, so it comes after every other match. Then every usable row comes before every grey
// row. Then the strength with the category boost (match). Only among equal strengths do COMMON and the use boost count
// (uses: cleanUses, now), then the shorter label, then the catalogue order.
export function rankMatches(rows, q, n = 50, { uses = {}, now = Date.now() } = {}) {
  q = String(q || '')
    .trim()
    .toLowerCase();
  const scored = rows
    .map((row, i) => {
      if (!q) return { row, weak: false, s: 0, use: 0, i };
      const m = match(row, q);
      return m && { row, weak: m.loose || isDanger(row), s: m.strength, use: (COMMON[row.key] || 0) + useBoost(uses[row.key], now), i };
    })
    .filter(Boolean);
  const shorter = (a, b) => (q ? a.row.label.length - b.row.label.length : 0);
  return scored
    .sort((a, b) => a.weak - b.weak || usable(b.row) - usable(a.row) || b.s - a.s || b.use - a.use || shorter(a, b) || a.i - b.i)
    .slice(0, n)
    .map(({ row, weak }) => ({ row, weak }));
}
export const rankRows = (rows, q, n = 50, use = {}) => rankMatches(rows, q, n, use).map((m) => m.row);

// Most preferences are in This browser and again in Everyone's defaults, with the same label. A search lists the
// This browser row only, since Settings shows it first and every user can change it. The section list keeps both.
export function dropTwins(rows) {
  const own = new Set(rows.filter((r) => r.cat === 'settings' && r.target?.section === 'this-browser' && usable(r)).map((r) => r.label));
  return rows.filter((r) => !(r.cat === 'settings' && r.target?.section === 'everyone-defaults' && own.has(r.label)));
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
// - { type: 'row', row, weak }: a catalogue row. weak: Enter does not pick it first (rankMatches).
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
    if (q) return searchView(dropTwins(rows), String(query), use);
    const home = rowByKey(rows, 'page:home');
    const marked = marks.filter((r) => r !== home);
    const picks = cleanPicks(last)
      .map((k) => rowByKey(rows, k))
      .filter((r) => r && r !== home && !marked.includes(r));
    const cats = CATEGORIES.map((c) => catItem(c.id, rows)).filter((c) => c.count);
    return [
      ...(home && !home.here ? rowItems([home]) : []),
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
  else if (q) list = dropTwins(list);
  const ms = rankMatches(list, q, Infinity, use);
  return [...strongItems(ms), ...withHead(weakItems(ms))];
}

const catItem = (id, rows) => ({ type: 'cat', id, label: catLabel(id), count: rows.filter((r) => r.cat === id).length });
const strongItems = (ms) => ms.filter((m) => !m.weak).map(({ row }) => ({ type: 'row', row }));
const weakItems = (ms) => ms.filter((m) => m.weak).map(({ row }) => ({ type: 'row', row, weak: true }));
// The rows after the heading "Other matches", or none. The weak rows are always under it, so the list shows why Enter
// skips them.
const withHead = (items) => (items.length ? [{ type: 'head', label: 'Other matches' }, ...items] : []);

// A search at the top. When the first word names a category (paletteScope), that category comes first.
// - The word alone: the category itself is the first row, so Enter opens it. A row of a different category with the
//   word as its full label comes next (the Settings page for "settings"). Then the rows of the category.
// - A word and more text: the rows of the category that match the rest come first.
// The other matches of the whole query follow under a heading, so a wrong guess hides nothing. When no row of the
// category matches, the list says so and shows every match. A word alone that names two categories (a tie) gives both.
function searchView(rows, query, use) {
  const sc = paletteScope(query, rows);
  const word = /^\s*(\S+)\s*$/.exec(query)?.[1];
  const g = !sc && word ? categoryGuess(word) : { ids: [] };
  const tie = g.ids.length > 1 && !weakGuess(g, word, rows) ? g.ids : [];
  const cats = sc ? [sc.cat] : tie.length > 1 && !paletteNamed(query, rows) ? tie : [];
  if (!cats.length) {
    const ms = rankMatches(rows, query, 50, use);
    return [...strongItems(ms), ...withHead(weakItems(ms))];
  }
  const inCats = rows.filter((r) => cats.includes(r.cat));
  const others = rankMatches(
    rows.filter((r) => !cats.includes(r.cat)),
    query,
    50,
    use,
  );
  if (!sc?.rest) {
    const named = others.filter((m) => !m.weak && wordsOf(m.row.label) === wordsOf(query));
    const rest = others.filter((m) => !named.includes(m));
    return [
      ...cats.map((id) => catItem(id, rows)).filter((c) => c.count),
      ...strongItems(named),
      ...inCats.slice(0, 50).map((row) => ({ type: 'row', row })),
      ...withHead([...strongItems(rest), ...weakItems(rest)]),
    ];
  }
  const first = rankMatches(inCats, sc.rest, 50, use);
  const tail = [...strongItems(others), ...weakItems(first), ...weakItems(others)];
  if (!strongItems(first).length) return [{ type: 'head', label: `No match in ${cats.map(catLabel).join(' or ')}` }, ...tail];
  return [...strongItems(first), ...withHead(tail)];
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

const pickable = (items, i) => !!items[i] && items[i].type !== 'head';

// The next row index for a step, skipping heading lines; -1 when nothing can be picked. It wraps. From no row (-1), a
// step down goes to the first row and a step up to the last.
export function stepIndex(items, from, by) {
  const n = items.length;
  if (!items.some((_, i) => pickable(items, i))) return -1;
  let i = from < 0 ? (by > 0 ? -1 : n) : from;
  for (let k = 0; k < n; k++) {
    i = (((i + by) % n) + n) % n;
    if (pickable(items, i)) return i;
  }
  return -1;
}

// A jump of by rows (a page of them). It stops at the first or the last row and does not wrap.
export function jumpIndex(items, from, by) {
  const all = items.map((_, i) => i).filter((i) => pickable(items, i));
  if (!all.length) return -1;
  if (from < 0) return by > 0 ? all[0] : all[all.length - 1];
  const to = from + by;
  return by > 0 ? (all.find((i) => i >= to) ?? all[all.length - 1]) : (all.findLast((i) => i <= to) ?? all[0]);
}

// The new row index for a key in the search box, or null when the key does not move the cursor. Tab and Shift+Tab
// step, so the focus stays in the box. PageDown and PageUp jump page rows. Ctrl+Home and Ctrl+End go to the first and
// the last row. Plain Home and End stay text keys of the box.
export function keyMove({ key, ctrlKey, metaKey, altKey, shiftKey }, items, cur, page = 10) {
  if (metaKey || altKey) return null;
  if (key === 'Tab' && !ctrlKey) return stepIndex(items, cur, shiftKey ? -1 : 1);
  if (shiftKey) return null;
  if (!ctrlKey && (key === 'PageDown' || key === 'PageUp')) return jumpIndex(items, cur, key === 'PageDown' ? page : -page);
  if (ctrlKey && (key === 'Home' || key === 'End')) return jumpIndex(items, -1, key === 'Home' ? 1 : -1);
  return null;
}

// Backspace in an empty search box: 'up' goes back one level, 'none' does nothing. At the top only Esc closes the
// Palette, so one Backspace too many does not close it.
export const emptyBackspace = (path) => (path.length ? 'up' : 'none');

// Where the cursor starts, or -1 for no row: then Enter does nothing. It is never a weak row (rankMatches), and never a
// grey row while a different row can be picked. With no query (empty), only a category or a Settings section can be
// first. So Enter on an empty Palette never fires a bookmarked preset.
export function firstIndex(items, { empty = false } = {}) {
  if (empty) {
    const i = items.findIndex((it) => it.type !== 'head');
    return items[i]?.type === 'cat' ? i : -1;
  }
  const i = items.findIndex((it) => it.type === 'cat' || (it.type === 'row' && !it.weak && usable(it.row)));
  return i >= 0 ? i : items.findIndex((it) => it.type === 'row' && !it.weak);
}
