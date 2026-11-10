// What the panel editor reads from its URL and from /api/v1/panels, and what it sends back. Pure.
import { KINDS } from './panelrules.js';
import { saved, createGrid, DEFAULT_ROWS, DEFAULT_COLS } from './gridmodel.js';
import { pagesOf, pageView, storedPages } from './panelpages.js';
import { chord } from './keys.js';
import { nameMismatch } from './panellook.js';

// The Cell tab's lines about Default artwork. d is the item's default or null, own whether the cell sets artwork of
// its own, routerName the router's name for the cell's number now ('' when not known).
export function defaultNote(d, own, routerName) {
  if (!d) return { text: '', warn: '' };
  const what = d.kind === 'preset' ? 'preset' : d.kind;
  const text = own ? `This cell's own artwork shows instead of the Default artwork of its ${what}.` : `This cell shows the Default artwork of its ${what}.`;
  const warn = nameMismatch(d, routerName)
    ? `This default may be for another item: it was set for ${d.name}, and the router now names this ${what} ${routerName}.`
    : '';
  return { text, warn };
}

// Whether a cell's item can carry Default artwork. Me presets cannot: the set is shared, so the server refuses them.
export const takesDefault = (c) =>
  !!c && (c.kind === 'preset' ? c.scope === 'everyone' && !!c.name : (c.kind === 'source' || c.kind === 'destination') && Number.isInteger(c.n));

// The body of PUT (set) or DELETE (clear) /default-artwork for a cell's item. Set sends the cell's own artwork and
// Hide text, and the panel an uploaded image is on, which the server copies.
export function defaultBody(cell, panelId, set) {
  const item = cell.kind === 'preset' ? { kind: cell.kind, scope: cell.scope, name: cell.name } : { kind: cell.kind, n: cell.n };
  if (!set) return item;
  return { ...item, art: cell.art || '', artB: cell.artB || '', hideText: cell.hideText === true, panel: panelId };
}

// /panels/editor?panel=<id> edits one panel; anything else starts a new one.
export function editorTarget(search) {
  const id = new URLSearchParams(search).get('panel') || '';
  return { id: /^[A-Za-z0-9_-]+$/.test(id) ? id : '' };
}

const byName = (a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });

// canEdit is the server's word for what the user may change (an owner, or a Trusted user for any panel).
export function groupPanels(list) {
  const all = [...(list || [])].sort(byName);
  return { mine: all.filter((p) => p.canEdit), shared: all.filter((p) => !p.canEdit) };
}

// The server gives the Page its id on the first save.
export const newPanel = () => ({
  name: 'New panel',
  acts: 'session',
  share: { mode: 'me' },
  rows: DEFAULT_ROWS,
  cols: DEFAULT_COLS,
  pages: [{ id: '', enabled: true, cells: [] }],
  settings: {},
});

// PUT replaces the panel, so what the editor does not own goes back as it came (settings, sharing, unknown fields).
// fields is the sidebar's edits: {settings, acts, clientRights}, each present only once edited. clientRights is
// left out otherwise so the server keeps the stored Rights (an administrator may have widened them). grid is Page 1;
// pages is the Page list the editor holds, and the server refuses a body with both pages and cells.
export function saveBody(loaded, name, grid, fields = {}, pages = pagesOf(loaded || newPanel())) {
  const rest = { ...loaded };
  delete rest.canEdit;
  delete rest.clientRights;
  delete rest.cells;
  const edits = {};
  for (const k of ['settings', 'acts', 'slug', 'clientRights']) if (fields[k] !== undefined) edits[k] = fields[k];
  const { rows, cols, cells } = saved(grid);
  return { ...rest, name: name.trim(), rows, cols, pages: storedPages(pages, cells, rows, cols), ...edits };
}

// Compared through the grid, so a cell the model clips on load does not read as an edit. Keys are sorted because the
// server stores cells with their keys in another order.
const sorted = (c) => Object.fromEntries(Object.entries(c).sort(([a], [b]) => (a < b ? -1 : 1)));
// Page 1's cells come from the grid; the other Pages and every Page's id, name and enabled from pages. Each Page goes
// through the grid model, so one the editor once showed (reading order, its own key order) is no change.
const shape = (name, g, f, pages) =>
  JSON.stringify([
    name.trim(),
    g.rows,
    g.cols,
    saved(g).cells.map(sorted),
    storedPages(pages, [], g.rows, g.cols).map((p) => [p.id, p.name || '', p.enabled !== false, saved(createGrid({ rows: g.rows, cols: g.cols, cells: p.cells })).cells.map(sorted)]),
    sorted(f.settings || {}),
    f.acts || 'session',
    f.slug || '',
    f.clientRights ?? null,
  ]);

// A field not edited counts as what was loaded. clientRights has no loaded side here: any edit is a change.
export function isDirty(loaded, name, grid, fields = {}, pages = pagesOf(loaded || newPanel())) {
  const base = loaded || newPanel();
  const was = shape(base.name, createGrid(pageView(base)), { settings: base.settings, acts: base.acts, slug: base.slug }, pagesOf(base));
  const now = shape(
    name,
    grid,
    {
      settings: fields.settings ?? base.settings,
      acts: fields.acts ?? base.acts,
      slug: fields.slug ?? base.slug,
      clientRights: fields.clientRights,
    },
    pages,
  );
  return was !== now;
}

// Where the smart menu goes. rect is the selection and area the box it must stay inside ({left, top, right,
// bottom}, see menuArea), both in pixels. Right of the selection, else left, else below.
export function placeMenu(rect, menu, area, gap) {
  const top = (t) => Math.max(area.top, Math.min(t, area.bottom - menu.h));
  if (rect.right + gap + menu.w <= area.right) return { side: 'right', left: rect.right + gap, top: top(rect.top) };
  if (rect.left - gap - menu.w >= area.left) return { side: 'left', left: rect.left - gap - menu.w, top: top(rect.top) };
  return { side: 'below', left: Math.max(area.left, Math.min(rect.left, area.right - menu.w)), top: rect.bottom + gap };
}

// The box the menu and its flyouts stay inside: the viewport, cut at the sidebar when the sidebar sits beside the
// grid (it edits the selected cell, so the menu must not cover it). Under the grid, in the narrow layout, it does
// not bound them. side and grid are screen rects; side may be null.
export function menuArea(vp, side, grid) {
  const area = { left: 0, top: 0, right: vp.w, bottom: vp.h };
  if (side && grid && side.left >= grid.right) area.right = Math.min(area.right, side.left);
  return area;
}

// Which side of the menu a flyout (Type, Find) opens on: the menu's own side, away from the selection, when it fits
// there (below counts as right); else the other side when that fits; else whichever has more room. at is placeMenu's
// answer and menuW the menu's width.
export function subSide(at, menuW, subW, area, gap) {
  const roomRight = area.right - gap - (at.left + menuW);
  const roomLeft = at.left - gap - area.left;
  const first = at.side === 'left' ? 'left' : 'right';
  const room = { right: roomRight, left: roomLeft };
  if (room[first] >= subW) return first;
  const other = first === 'left' ? 'right' : 'left';
  if (room[other] >= subW) return other;
  return roomRight >= roomLeft ? 'right' : 'left';
}

// How far a flyout (Find) must move to sit inside the viewport, in pixels; {x: 0, y: 0} when it already does. The
// top and left edges win over the bottom and right, so the box at the top stays reachable when it is too tall.
export function flyoutShift(rect, vp, gap) {
  let x = 0;
  let y = 0;
  if (rect.right > vp.w - gap) x = vp.w - gap - rect.right;
  if (rect.bottom > vp.h - gap) y = vp.h - gap - rect.bottom;
  if (rect.left + x < gap) x = gap - rect.left;
  if (rect.top + y < gap) y = gap - rect.top;
  return { x, y };
}

// The item a Ctrl chord runs (item.chord is the letter): never a disabled one. Ctrl on a Mac too, since macOS takes
// Cmd+M to minimise the window before the page sees it.
export const menuChord = (items, e) => items.find((it) => it.chord && chord(e) === `ctrl+${it.chord}` && !it.disabled) ?? null;

// U+2303 is the Mac's Control sign.
export const chordText = (letter, mac) => (mac ? `\u2303${letter.toUpperCase()}` : `Ctrl+${letter.toUpperCase()}`);

// Ctrl or Cmd+S saves the panel instead of the browser saving the page. By e.code (chord), so any keyboard layout.
export const saveChord = (e) => ['ctrl+s', 'meta+s'].includes(chord(e));

// The note and button above the editor for a sign-in step (sessionStep, plus 'ended', 'refused' and 'offline').
// err is Sode's reason when it refused the page's key. Steps with nothing to say ('ready', 'wait' behind the page
// lock's own pad, 'request') give empty text.
export function signInNote(step, err = '') {
  switch (step) {
    case 'sign-in':
      return { text: 'Sign in with your PIN to edit panels.', button: 'Sign in' };
    case 'no-users':
      return { text: 'Panels need a user, and Sode has none. Add one in Settings, Users.', button: '' };
    case 'ended':
      return { text: "This page's sign-in key was ended. Sign in again to keep editing.", button: 'Sign in again' };
    case 'refused':
      return { text: `Sode would not open the editor for this sign-in${err ? `: ${err}` : ''}. Sign in again.`, button: 'Sign in again' };
    case 'offline':
      return { text: 'Sode is not answering. The page keeps trying; check the network if this stays.', button: '' };
    default:
      return { text: '', button: '' };
  }
}

// Why a panel could not be opened, from jsonCall's error, and what to do about it.
export function openFailNote(e) {
  if (e.status === 0) return 'Sode did not answer, so the panel could not be opened. Check the network, then choose the panel again.';
  if (e.status === 404) return 'No panel here that you can use. Choose another panel, or ask its owner to share it with you.';
  if (e.status === 401) return 'Your sign-in for this page ended. Sign in again, then choose the panel again.';
  return e.message || `The panel could not be opened (${e.status}).`;
}

const WORDS = {
  source: 'Source',
  destination: 'Destination',
  take: 'Take',
  preset: 'Preset',
  clear: 'Clear',
  current: 'Current Source',
  panelName: 'Panel Name',
  instanceName: 'Instance Name',
  goToPage: 'Go to Page',
  nextPage: 'Next Page',
  previousPage: 'Previous Page',
  '': 'Empty',
};
export const kindLabel = (kind) => WORDS[kind] ?? kind.charAt(0).toUpperCase() + kind.slice(1);
// The Type item's label: the one kind the selected squares share, else Mixed.
export function typeLabel(cells) {
  const kinds = new Set(cells.map((c) => c.kind));
  return `Type: ${kinds.size > 1 ? 'Mixed' : kindLabel(cells[0]?.kind ?? '')}`;
}
// With fixed (the panel's Fixed destination is on) a Destination cell would not be drawn, so it cannot be chosen.
export const kindChoices = ({ fixed = false } = {}) =>
  [...KINDS, ''].map((kind) =>
    fixed && kind === 'destination' ? { kind, label: kindLabel(kind), disabled: true, hint: 'Fixed destination is on' } : { kind, label: kindLabel(kind) },
  );

// The tip beside the grip while it is dragged: how many squares, and the first and last name placed.
export function gripTip(plan, nameOf) {
  if (!plan) return '';
  const n = plan.slots.length;
  if (!n) return 'Nothing to add here';
  const name = (s) => nameOf({ kind: s.kind, n: s.n });
  return `${n} square${n > 1 ? 's' : ''}, ${n > 1 ? `${name(plan.slots[0])} to ${name(plan.slots[n - 1])}` : name(plan.slots[0])}`;
}
