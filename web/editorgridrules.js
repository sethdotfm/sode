// The Grid panel editor's Sources and Destinations lists: the rows of an edge and each edit of it. Pure.
// An edge is { members: 'all' | [n, ...], hide, items } (gridpaneledges.js, ADR 0011). An edit gives a new edge, or the
// same edge when nothing changed.
import { allowed } from './rightsrules.js';
import { filterFn } from './filter.js';
import { chord } from './keys.js';
import { artRun } from './arttoolrules.js';

const isAll = (edge) => !Array.isArray(edge?.members);
const pinnedIn = (edge, n) => edge?.items?.[n]?.pinned === true;
const pinCount = (edge) => Object.values(edge?.items || {}).filter((it) => it?.pinned === true).length;

// The members in panel order: an "all" edge is the router in router order, less hide.
function membersOf(edge, router) {
  if (!isAll(edge)) return edge.members;
  const hide = new Set(edge?.hide || []);
  return (router || [])
    .map((x) => x.n)
    .sort((a, b) => a - b)
    .filter((n) => !hide.has(n));
}

// router is the /names list for the edge; rights the user's own (null: no limits); kind the Rights key ('sources' or
// 'dsts'). Entries outside the Rights stay in the edge and are only counted in outside.
export function edgeRows(edge, router, rights, kind) {
  const names = new Map((router || []).map((x) => [x.n, x.name]));
  const all = membersOf(edge, router);
  const rows = all.filter((n) => allowed(rights, kind, n)).map((n) => ({ n, name: names.get(n) ?? '', pinned: pinnedIn(edge, n), gone: !names.has(n) }));
  // "all" is resolved for each viewer, so it holds nothing outside the Rights.
  return { rows, all: isAll(edge), outside: isAll(edge) ? 0 : all.length - rows.length, pins: pinCount(edge) };
}

// Sets or clears an item's pin. Other item settings (art) stay; an item with nothing left goes.
function withItem(edge, n, patch) {
  const items = { ...(edge.items || {}) };
  const it = { ...(items[n] || {}), ...patch };
  for (const k of Object.keys(it)) if (it[k] === undefined || it[k] === false) delete it[k];
  if (Object.keys(it).length) items[n] = it;
  else delete items[n];
  return { ...edge, items };
}

const withoutHide = (edge) => {
  const out = { ...(edge || {}) };
  delete out.hide;
  return out;
};

export function clearEdge(edge, rights, kind) {
  const keep = isAll(edge) ? [] : edge.members.filter((n) => !allowed(rights, kind, n));
  let e = { ...withoutHide(edge), members: keep, items: { ...(edge?.items || {}) } };
  for (const n of Object.keys(e.items)) if (!keep.includes(Number(n))) e = withItem(e, n, { pinned: undefined });
  return e;
}

export const followRouter = (edge) => ({ ...withoutHide(edge), members: 'all' });

// Follow the router switched off: the edge keeps what it shows now, in the same order, as a list.
export const stopFollowing = (edge, router) => (isAll(edge) ? { ...withoutHide(edge), members: membersOf(edge, router) } : edge);

// Add from the router: the router's items the panel does not have, in router order, within the Rights.
export function routerRows(router, edge, rights, kind) {
  const has = new Set(membersOf(edge, router));
  return (router || [])
    .filter((x) => !has.has(x.n) && allowed(rights, kind, x.n))
    .sort((a, b) => a.n - b.n)
    .map((x) => ({ n: x.n, name: x.name ?? '' }));
}

// Find uses the main page's filter language on the numbers as shown (off is numberOffset). Text it cannot read
// matches nothing.
export function findRows(rows, text, off) {
  const f = filterFn(text, { fuzzy: true });
  return f ? rows.filter((r) => f(r.n + off, r.name)) : [];
}

// s is { selected: Set of router numbers, anchor }. They are router numbers, so they stay through moves and edits. Shift adds
// the rows in shown from the anchor to n; with the anchor out of shown it adds n alone.
export function selectRows(s, shown, n, { shift = false, toggle = false } = {}) {
  const at = shown.findIndex((r) => r.n === n);
  const from = shown.findIndex((r) => r.n === s.anchor);
  if (shift && at >= 0 && from >= 0) {
    const selected = new Set(s.selected);
    for (let i = Math.min(at, from); i <= Math.max(at, from); i++) selected.add(shown[i].n);
    return { selected, anchor: s.anchor };
  }
  if (toggle) {
    const selected = new Set(s.selected);
    if (!selected.delete(n)) selected.add(n);
    return { selected, anchor: n };
  }
  return { selected: shift ? new Set([...s.selected, n]) : new Set([n]), anchor: n };
}

// Appends the selected items in router order, less the members the edge has. An "all" edge becomes a list first.
export function addItems(edge, router, selected) {
  const list = membersOf(edge, router);
  const has = new Set(list);
  const add = [...selected].filter((n) => !has.has(n)).sort((a, b) => a - b);
  if (!add.length) return edge;
  return { ...withoutHide(edge), members: [...list, ...add] };
}

// The panel's list keys, by chord (keys.js), while the list itself has focus. go moves the cursor (extend: Shift
// selects from the anchor), move moves the selected rows. Ctrl+A is also Cmd+A, as in a text field.
const EDGE_KEYS = {
  up: { go: 'up' },
  down: { go: 'down' },
  home: { go: 'home' },
  end: { go: 'end' },
  pageup: { go: 'pageUp' },
  pagedown: { go: 'pageDown' },
  'alt+up': { move: 'up' },
  'alt+down': { move: 'down' },
  'alt+home': { move: 'top' },
  'alt+end': { move: 'end' },
  // macOS gives Ctrl+Up and Ctrl+Down to Mission Control unless that is turned off, so Ctrl+Shift and Cmd+Shift also move.
  'ctrl+up': { move: 'up' },
  'ctrl+down': { move: 'down' },
  'ctrl+shift+up': { move: 'up' },
  'ctrl+shift+down': { move: 'down' },
  'shift+meta+up': { move: 'up' },
  'shift+meta+down': { move: 'down' },
  'ctrl+a': { all: true },
  'meta+a': { all: true },
  ' ': { toggle: true },
  escape: { none: true },
};
export function edgeListKey(e) {
  const c = chord(e);
  const plain = EDGE_KEYS[c];
  if (plain) return plain;
  const go = c.startsWith('shift+') && EDGE_KEYS[c.slice(6)]?.go;
  return go ? { go, extend: true } : null;
}

// Edits of several selected entries at once. ns is a Set of router numbers. Each edit copies the edge one time, since
// a selection can hold thousands.

// Rows in drawn order, since the list shows them so and a move or pin goes by what shows.
const selectedShown = (edge, router, rights, kind, ns) => {
  const rows = drawnOrder(edgeRows(edge, router, rights, kind).rows);
  return { shown: rows.map((r) => r.n), block: rows.filter((r) => ns.has(r.n)).map((r) => r.n) };
};

// A list edge's members with the pinned ones first, in the order the list and the panel draw them. An item pinned
// moves up with them, and stays at the top of the rest when unpinned.
const pinsFirst = (edge, list) => [...list.filter((n) => pinnedIn(edge, n)), ...list.filter((n) => !pinnedIn(edge, n))];

// Moves the selected rows, in drawn order, to before the row before (a router number), or to the end when before is
// null. A selected row is not a place, so the block lands before the next row that is not selected. Pinned rows stay
// above the rest: a block of only pinned or only unpinned rows stops at that line. Entries outside the Rights stay in
// the list. A move is a reorder, so an "all" edge becomes a list.
export function moveRows(edge, router, rights, kind, ns, before) {
  const { shown, block } = selectedShown(edge, router, rights, kind, ns);
  if (!block.length) return edge;
  let at = before === null ? -1 : shown.indexOf(before);
  while (at >= 0 && ns.has(shown[at])) at = at + 1 < shown.length ? at + 1 : -1;
  const firstFree = shown.findIndex((n) => !pinnedIn(edge, n) && !ns.has(n));
  const pins = block.filter((n) => pinnedIn(edge, n)).length;
  if (pins === 0 && at >= 0 && pinnedIn(edge, shown[at])) at = firstFree;
  if (pins === block.length && (at < 0 || !pinnedIn(edge, shown[at]))) at = firstFree;
  const was = pinsFirst(edge, membersOf(edge, router));
  const moved = new Set(block);
  const rest = was.filter((n) => !moved.has(n));
  const i = at < 0 ? rest.length : rest.indexOf(shown[at]);
  const next = [...rest.slice(0, i), ...block, ...rest.slice(i)];
  // What shows is the drawn order, so a move that keeps it is no edit.
  if (next.every((n, k) => n === was[k])) return edge;
  return { ...withoutHide(edge), members: next };
}

// The keyboard's move. dir is 'up' or 'down' (past the row next to the block), 'top' or 'end'.
export function stepRows(edge, router, rights, kind, ns, dir) {
  const { shown, block } = selectedShown(edge, router, rights, kind, ns);
  if (!block.length) return edge;
  const first = shown.indexOf(block[0]);
  const last = shown.indexOf(block.at(-1));
  if (dir === 'top') return moveRows(edge, router, rights, kind, ns, shown[0]);
  if (dir === 'end') return moveRows(edge, router, rights, kind, ns, null);
  if (dir === 'up') return first > 0 ? moveRows(edge, router, rights, kind, ns, shown[first - 1]) : edge;
  if (last >= shown.length - 1) return edge;
  return moveRows(edge, router, rights, kind, ns, last + 2 < shown.length ? shown[last + 2] : null);
}

// Sets or clears keys of one item in items (a copy the caller owns). An item with nothing left goes.
function patchItem(items, n, patch) {
  const it = { ...(items[n] || {}) };
  for (const [k, v] of Object.entries(patch)) {
    if (v === undefined || v === false || v === '') delete it[k];
    else it[k] = v;
  }
  if (Object.keys(it).length) items[n] = it;
  else delete items[n];
}

// Pins the selected rows in drawn order, or unpins them all. On a list edge the pinned members go
// first; an "all" edge stays "all", since its order is the router's.
export function pinItems(edge, router, rights, kind, ns, on) {
  const e = edge || { members: 'all' };
  const items = { ...(e.items || {}) };
  let changed = false;
  for (const n of selectedShown(e, router, rights, kind, ns).block) {
    if (pinnedIn(e, n) === on) continue;
    patchItem(items, n, { pinned: on });
    changed = true;
  }
  if (!changed) return edge;
  const out = { ...e, items };
  return isAll(out) ? out : { ...out, members: pinsFirst(out, out.members) };
}

// Each selected entry goes out of a list, or is hidden on "all". Its pin goes, so a removed item does not come back
// pinned; its art stays.
export function removeItems(edge, ns) {
  if (!ns.size) return edge;
  const e = edge || { members: 'all' };
  const items = { ...(e.items || {}) };
  for (const n of ns) if (items[n]?.pinned) patchItem(items, n, { pinned: false });
  if (!isAll(e)) return { ...e, members: e.members.filter((x) => !ns.has(x)), items };
  const hide = e.hide || [];
  const had = new Set(hide);
  return { ...e, hide: [...hide, ...[...ns].filter((n) => !had.has(n))], items };
}

const ITEM_ART = ['art', 'artB', 'hideText']; // the artwork keys a Grid item stores, as a cell's

// The order the panel page draws the rows in: pinned first, each part in list order. Keep in step with gridpaneledges.js
// pinnedFirst.
export const drawnOrder = (rows) => [...rows.filter((r) => r.pinned), ...rows.filter((r) => !r.pinned)];

// One artwork choice (arttoolrules.js artRun) on the rows ns, router numbers in drawn order. Icon A goes on each, or
// Icon B runs from the first. left counts the rows past the end of the run. The edge is the same when nothing changes.
export function itemsArt(edge, ns, choice) {
  const run = artRun(choice);
  if (!run) return { edge, left: 0 };
  const e = edge || { members: 'all' };
  const items = { ...(e.items || {}) };
  let changed = false;
  let left = 0;
  for (const n of ns) {
    const patch = run();
    if (!patch) {
      left++;
      continue;
    }
    for (const k of Object.keys(patch)) if (!ITEM_ART.includes(k)) delete patch[k];
    const was = items[n] || {};
    if (Object.keys(patch).every((k) => (was[k] || '') === (patch[k] || ''))) continue;
    patchItem(items, n, patch);
    changed = true;
  }
  return { edge: changed ? { ...e, items } : edge, left };
}

// Hide text on the rows ns, as the Cell tab sets it on a cell. Off is false where hasDefault(n) says a Default artwork
// could hide the name, else removed (panellook.js withDefault). The edge is the same when nothing changes.
export function itemsHideText(edge, ns, hide, hasDefault) {
  const e = edge || { members: 'all' };
  const items = { ...(e.items || {}) };
  let changed = false;
  for (const n of ns) {
    const v = hide ? true : hasDefault(n) ? false : undefined;
    const was = items[n] || {};
    if (was.hideText === v) continue;
    const it = { ...was };
    if (v === undefined) delete it.hideText;
    else it.hideText = v;
    if (Object.keys(it).length) items[n] = it;
    else delete items[n];
    changed = true;
  }
  return changed ? { ...e, items } : edge;
}
