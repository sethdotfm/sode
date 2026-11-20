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

// The categories, in the order the empty Palette lists them. A row's cat is one of these ids.
export const CATEGORIES = Object.freeze([
  { id: 'actions', label: 'Actions' },
  { id: 'presets', label: 'Presets' },
  { id: 'pages', label: 'Pages' },
  { id: 'panels', label: 'Panels' },
  { id: 'filterSets', label: 'Filter sets' },
  { id: 'settings', label: 'Settings' },
]);
const catLabel = (id) => CATEGORIES.find((c) => c.id === id)?.label || id;

// Higher is a better match, -1 none. The label counts most, then the hint (scope, section) and the words.
function score(row, q) {
  const s = fuzzyScore({ num: '', lname: row.label.toLowerCase() }, q);
  const extra = [row.hint, ...(row.words || [])].filter(Boolean).join(' ').toLowerCase();
  const e = extra ? fuzzyScore({ num: '', lname: extra }, q) : -1;
  return Math.max(s, e >= 0 ? e - 400 : -1);
}

// Rows that match q, best first; equal scores keep the catalogue order.
export function rankRows(rows, q, n = 50) {
  q = String(q || '')
    .trim()
    .toLowerCase();
  if (!q) return rows.slice(0, n);
  return rows
    .map((r, i) => [score(r, q), i, r])
    .filter(([s]) => s >= 0)
    .sort((a, b) => b[0] - a[0] || a[1] - b[1])
    .slice(0, n)
    .map(([, , r]) => r);
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
// picks, then the categories. A query at the top searches every row.
export function paletteView(rows, { path = [], query = '', last = [], marks = [] } = {}) {
  const q = String(query || '').trim();
  const rowItems = (list) => list.map((row) => ({ type: 'row', row }));
  if (!path.length) {
    if (q) return rowItems(rankRows(rows, q));
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
  return rowItems(rankRows(list, q, Infinity));
}

// The line above the list: where the Palette is.
export function pathText(path, rows) {
  if (!path.length) return '';
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
export const firstIndex = (items) => stepIndex(items, -1, 1);
