// What the panel editor reads from its URL and from /api/v1/panels, and what it sends back. Pure.
import { KINDS } from './panelrules.js';
import { saved, createGrid, DEFAULT_ROWS, DEFAULT_COLS } from './gridmodel.js';
import { chord } from './keys.js';

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

export const newPanel = () => ({ name: 'New panel', acts: 'session', share: { mode: 'me' }, rows: DEFAULT_ROWS, cols: DEFAULT_COLS, cells: [], settings: {} });

// PUT replaces the panel, so what the editor does not own goes back as it came (settings, sharing, unknown fields).
// fields is the sidebar's edits: {settings, acts, clientRights}, each present only once edited. clientRights is
// left out otherwise so the server keeps the stored Rights (an administrator may have widened them).
export function saveBody(loaded, name, grid, fields = {}) {
  const { canEdit, clientRights, ...rest } = loaded;
  const edits = {};
  for (const k of ['settings', 'acts', 'slug', 'clientRights']) if (fields[k] !== undefined) edits[k] = fields[k];
  return { ...rest, name: name.trim(), ...saved(grid), ...edits };
}

// Compared through the grid, so a cell the model clips on load does not read as an edit. Keys are sorted because the
// server stores cells with their keys in another order.
const sorted = (c) => Object.fromEntries(Object.entries(c).sort(([a], [b]) => (a < b ? -1 : 1)));
const shape = (name, g, f) =>
  JSON.stringify([
    name.trim(),
    g.rows,
    g.cols,
    saved(g).cells.map(sorted),
    sorted(f.settings || {}),
    f.acts || 'session',
    f.slug || '',
    f.clientRights ?? null,
  ]);

// A field not edited counts as what was loaded. clientRights has no loaded side here: any edit is a change.
export function isDirty(loaded, name, grid, fields = {}) {
  const base = loaded || newPanel();
  const was = shape(base.name, createGrid(base), { settings: base.settings, acts: base.acts, slug: base.slug });
  const now = shape(name, grid, {
    settings: fields.settings ?? base.settings,
    acts: fields.acts ?? base.acts,
    slug: fields.slug ?? base.slug,
    clientRights: fields.clientRights,
  });
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

export const todoText = (n) => (n ? `${n} unassigned square${n > 1 ? 's' : ''}` : 'No unassigned squares');

const WORDS = { source: 'Source', destination: 'Destination', take: 'Take', preset: 'Preset', clear: 'Clear', current: 'Current Destination', '': 'Empty' };
export const kindLabel = (kind) => WORDS[kind] ?? kind.charAt(0).toUpperCase() + kind.slice(1);
// The Type item's label: the one kind the selected squares share, else Mixed.
export function typeLabel(cells) {
  const kinds = new Set(cells.map((c) => c.kind));
  return `Type: ${kinds.size > 1 ? 'Mixed' : kindLabel(cells[0]?.kind ?? '')}`;
}
export const kindChoices = () => [...KINDS, ''].map((kind) => ({ kind, label: kindLabel(kind) }));

// The tip beside the grip while it is dragged: how many squares, and the first and last name placed.
export function gripTip(plan, nameOf) {
  if (!plan) return '';
  const n = plan.slots.length;
  if (!n) return 'Nothing to add here';
  const name = (s) => nameOf({ kind: s.kind, n: s.n });
  return `${n} square${n > 1 ? 's' : ''}, ${n > 1 ? `${name(plan.slots[0])} to ${name(plan.slots[n - 1])}` : name(plan.slots[0])}`;
}
