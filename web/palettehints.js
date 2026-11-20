// Palette lines that are not rows: the count of matches the user cannot use, the hint for a number and the key hint.
// Pure, so Node tests load them. palette.js draws them.
import { CATEGORIES, paletteScope, rankRows } from './paletterules.js';

const NOUN = { actions: 'action', presets: 'preset', pages: 'page', panels: 'panel', filterSets: 'filter set', settings: 'setting' };
const counted = (n, id) => `${n} ${NOUN[id] || id}${n === 1 ? '' : 's'}`;
const words = (q) =>
  String(q || '')
    .toLowerCase()
    .split(/[^\p{L}\p{N}]+/u)
    .filter(Boolean);

// The query names panels: their category, or a word such as "panel" or "pan".
const namesPanels = (query, sc) => sc?.cat === 'panels' || words(query).some((w) => (w.length >= 3 && 'panels'.startsWith(w)) || w.startsWith('panel'));

// The matches that the Palette hides, per category id. Grey rows (outside the Rights or Role) do not show, except a
// bookmark (marks). panels is the count of panels that the server did not list (palettePanels hidden). Their names do
// not come to the browser, so they count only when the query names panels.
export function hiddenCounts(rows, query, { marks = [], panels = 0 } = {}) {
  const q = String(query || '').trim();
  if (!q) return {};
  const grey = rows.filter((r) => r.access?.ok === false && !r.alias && !marks.includes(r));
  const sc = paletteScope(q, rows);
  let found;
  if (sc) {
    const inCat = grey.filter((r) => r.cat === sc.cat);
    found = [
      ...(sc.rest ? rankRows(inCat, sc.rest, Infinity) : inCat),
      ...rankRows(
        grey.filter((r) => r.cat !== sc.cat),
        q,
        Infinity,
      ),
    ];
  } else found = rankRows(grey, q, Infinity);
  const out = {};
  for (const r of found) out[r.cat] = (out[r.cat] || 0) + 1;
  if (panels > 0 && namesPanels(q, sc)) out.panels = (out.panels || 0) + panels;
  return out;
}

// The line under the matches when some are hidden, or null. signedIn: the browser's own sign-in has a user. act is
// what Enter does: 'signIn' for a Guest, 'stepIn' for a user who can step in, else '' (the line only tells).
export function hiddenLine(counts, { signedIn = false, canSignIn = false, canStepIn = false } = {}) {
  const parts = CATEGORIES.filter((c) => counts[c.id] > 0).map((c) => counted(counts[c.id], c.id));
  if (!parts.length) return null;
  const list = parts.length > 1 ? `${parts.slice(0, -1).join(', ')} and ${parts.at(-1)}` : parts[0];
  const one = parts.length === 1 && Object.values(counts).reduce((a, b) => a + b, 0) === 1;
  if (!signedIn) {
    const act = canSignIn ? 'signIn' : '';
    return { act, label: `${list} ${one ? 'needs' : 'need'} sign-in.${act ? ' Enter: Sign in' : ''}` };
  }
  const act = canStepIn ? 'stepIn' : '';
  return { act, label: `${list} ${one ? 'is' : 'are'} outside your Rights or Role.${act ? ' Enter: Step in' : ''}` };
}

// A query of digits only is a router number: the Palette does not patch, so the hint tells where to type it.
export function numberHint(query, page) {
  if (!/^\d+$/.test(String(query || '').trim())) return '';
  return page === 'main' ? 'To patch by number, press Esc and type the number in Source.' : 'To patch by number, open Home and type the number in Source.';
}

// What Shift+Enter does on a row, or '': a preset fills the Patch boxes, a filter set (not All) adds to the stack.
export function shiftEnterDoes(row) {
  if (row?.kind === 'preset') return 'fill the Patch boxes';
  if (row?.kind === 'filterSet' && row.target?.scope !== 'all') return 'stack';
  return '';
}

// The key hint for the item under the cursor: only the keys that do something there. item is a paletteView item, a
// hidden line ({ type: 'hidden', act }) or undefined. pick: the Bookmarks editor's Add. why: a grey row's reason shows.
export function footKeys(item, { query = '', path = [], pick = false, why = false } = {}) {
  const keys = [['↑↓', 'move']];
  if (why) return [['Esc', 'back']];
  if (item?.type === 'cat') keys.push(['Enter', 'open']);
  else if (item?.type === 'hidden' && item.act) keys.push(['Enter', item.act === 'signIn' ? 'sign in' : 'step in']);
  else if (item?.type === 'row') {
    const row = item.row;
    if (pick) keys.push(['Enter', 'add']);
    else if (row.access?.ok === false) keys.push(['Enter', 'why']);
    else {
      keys.push(['Enter', row.kind === 'action' || row.kind === 'settingsButton' ? 'do' : 'open']);
      const shift = shiftEnterDoes(row);
      if (shift) keys.push(['⇧Enter', shift]);
      if (row.kind !== 'url') keys.push(['Alt+B', 'bookmark']);
    }
  }
  keys.push(['Esc', query ? 'clear' : path.length ? 'back' : 'close']);
  return keys;
}
