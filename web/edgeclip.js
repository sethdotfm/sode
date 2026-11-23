// The Grid panel editor's clipboard block of list rows. Cmd or Ctrl+C writes it from a Sources or Destinations list,
// and Cmd or Ctrl+V reads it into one. Pure (test/edgeclip.test.mjs); editorrowclip.js moves it.
//
// A block is { sode: ROWS_MARK, version, panel, axis, rows }. axis is the list it came from ('sources' or
// 'destinations'), and each row is { n, pinned?, art?, artB?, hideText? } in list order. It is not the Buttons cell
// block (gridclip.js), so neither paste takes the other's block. panel is the panel copied from ('' when unsaved),
// since an uploaded image belongs to that panel's artwork store.
import { addItems, pinsFirst } from './editorgridrules.js';
import { allowed } from './rightsrules.js';
import { artOf, ART_B_CHOICES } from './panellook.js';
import { plural } from './gridmodel.js';

export const ROWS_MARK = 'sode-panel-rows';
export const ROWS_VERSION = 1;
export const AXES = ['sources', 'destinations'];
const WHAT = { sources: 'source', destinations: 'destination' };
// A router has at most 65536 of each, so a longer block is not one Sode wrote.
const ROWS_MAX = 0x10000;
const TEXT_MAX = 8_000_000;
const NUMBER_MAX = 0xffff;

// Keep in step with ITEM_ART in editorgridrules.js: the keys a Grid item stores besides its pin.
const ROW_FIELDS = {
  pinned: (v) => typeof v === 'boolean',
  art: (v) => !!artOf({ art: v }),
  artB: (v) => ART_B_CHOICES.includes(v),
  hideText: (v) => typeof v === 'boolean',
};

// ns are router numbers in list order. null when there is nothing to copy.
export function copyRows(edge, axis, ns, panel) {
  if (!ns.length) return null;
  const items = edge?.items || {};
  const rows = ns.map((n) => {
    const row = { n };
    for (const k of Object.keys(ROW_FIELDS)) if (items[n]?.[k] !== undefined) row[k] = items[n][k];
    return row;
  });
  return { sode: ROWS_MARK, version: ROWS_VERSION, panel: panel || '', axis, rows };
}

export const rowsText = (block) => JSON.stringify(block);

// A copy of the row with only known fields, or null when a known field has the wrong shape.
function cleanRow(r) {
  if (!r || typeof r !== 'object' || Array.isArray(r)) return null;
  if (!Number.isInteger(r.n) || r.n < 0 || r.n > NUMBER_MAX) return null;
  const out = { n: r.n };
  for (const [k, ok] of Object.entries(ROW_FIELDS)) {
    if (r[k] === undefined) continue;
    if (!ok(r[k])) return null;
    out[k] = r[k];
  }
  return out;
}

// The block in text, or null for anything else. A clipboard is text anyone can write, so one bad row refuses the
// whole block, and a paste is all or nothing.
export function readRows(text) {
  if (typeof text !== 'string' || !text || text.length > TEXT_MAX) return null;
  let b;
  try {
    b = JSON.parse(text);
  } catch {
    return null;
  }
  if (!b || typeof b !== 'object' || Array.isArray(b) || b.sode !== ROWS_MARK || b.version !== ROWS_VERSION) return null;
  if (typeof b.panel !== 'string' || !AXES.includes(b.axis) || !Array.isArray(b.rows) || !b.rows.length || b.rows.length > ROWS_MAX) return null;
  const rows = [];
  const seen = new Set();
  for (const raw of b.rows) {
    const r = cleanRow(raw);
    if (!r || seen.has(r.n)) return null;
    seen.add(r.n);
    rows.push(r);
  }
  return { sode: b.sode, version: b.version, panel: b.panel, axis: b.axis, rows };
}

// A paste of block into the edge of list axis, before anything changes. o: { axis, rights, kind (the Rights key),
// panel (the panel pasted into), after (the router number the rows go after; none: the end) }. Answers
// { edge, added, have, outside, lostArt, refused }. refused is 'axis' for a block of the other list and 'following'
// while Follow the router is on; the edge is then the same. A row already in the list is skipped and does not move. A
// row outside the Rights is skipped, as Add from the router does. A row the router does not report is kept, since a
// list keeps such items. Across panels an uploaded image is dropped, as a cell paste drops it (ADR 0007).
export function pasteRows(edge, router, block, { axis, rights = null, kind, panel = '', after } = {}) {
  const none = { edge, added: [], have: 0, outside: 0, lostArt: 0, refused: null, axis, from: block.axis };
  if (block.axis !== axis) return { ...none, refused: 'axis' };
  if (!Array.isArray(edge?.members)) return { ...none, refused: 'following' };
  const has = new Set(edge.members);
  let have = 0;
  let outside = 0;
  const fresh = [];
  for (const r of block.rows) {
    if (has.has(r.n)) have++;
    else if (!allowed(rights, kind, r.n)) outside++;
    else fresh.push(r);
  }
  const counts = { ...none, have, outside };
  if (!fresh.length) return counts;
  const i = edge.members.indexOf(after);
  let e = addItems(
    edge,
    router,
    fresh.map((r) => r.n),
    i >= 0 ? { at: i + 1 } : {},
  );
  const items = { ...(e.items || {}) };
  const samePanel = !!panel && block.panel === panel;
  let lostArt = 0;
  for (const { n, ...row } of fresh) {
    const a = artOf(row);
    if (!samePanel && a?.id && !a.lib) {
      delete row.art;
      lostArt++;
    }
    if (row.pinned === false) delete row.pinned;
    if (Object.keys(row).length) items[n] = row;
    else delete items[n];
  }
  e = { ...e, items };
  if (fresh.some((r) => r.pinned)) e = { ...e, members: pinsFirst(e, e.members) };
  return { ...counts, edge: e, added: fresh.map((r) => r.n), lostArt };
}

// The status line for a paste. what is the list's item word ('source' or 'destination').
export function pasteRowsNote(r, what) {
  if (r.refused === 'axis') return `Nothing pasted: these are ${WHAT[r.from]}s, and this list holds ${what}s.`;
  if (r.refused === 'following') return `Nothing pasted: Follow the router is on for ${what}s, so the router sets the list.`;
  const parts = [r.added.length ? `Pasted ${plural(r.added.length, what)}.` : 'Nothing pasted.'];
  if (r.have) parts.push(`Skipped ${plural(r.have, what)} already in the list.`);
  if (r.outside) parts.push(`Skipped ${plural(r.outside, what)} outside your Rights.`);
  if (r.lostArt)
    parts.push(`${plural(r.lostArt, what)} lost ${r.lostArt === 1 ? 'its' : 'their'} uploaded image, since images stay with the panel they were added to.`);
  return parts.join(' ');
}

// After a Remove, the pick goes to the name before the removed ones, else to the name after them. shown is the router
// numbers in drawn order. null when none is left.
export function pickAfterRemove(shown, removed) {
  const first = shown.findIndex((n) => removed.has(n));
  if (first < 0) return null;
  for (let i = first - 1; i >= 0; i--) if (!removed.has(shown[i])) return shown[i];
  for (let i = first + 1; i < shown.length; i++) if (!removed.has(shown[i])) return shown[i];
  return null;
}
