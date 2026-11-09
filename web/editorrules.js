// What the panel editor reads from its URL and from /api/v1/panels, and what it sends back. Pure.
import { KINDS } from './panelrules.js';
import { saved, createGrid, DEFAULT_ROWS, DEFAULT_COLS } from './gridmodel.js';

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
export function saveBody(loaded, name, grid) {
  const { canEdit, clientRights, ...rest } = loaded;
  return { ...rest, name: name.trim(), ...saved(grid) };
}

// Compared through the grid, so a cell the model clips on load does not read as an edit. Keys are sorted because the
// server stores cells with their keys in another order.
const sorted = (c) => Object.fromEntries(Object.entries(c).sort(([a], [b]) => (a < b ? -1 : 1)));
const shape = (name, g) => JSON.stringify([name.trim(), g.rows, g.cols, saved(g).cells.map(sorted)]);

export const isDirty = (loaded, name, grid) => shape(name, grid) !== shape((loaded || newPanel()).name, createGrid(loaded || newPanel()));

// Where the smart menu goes. rect and vp are in pixels. Right of the selection, else left, else below.
export function placeMenu(rect, menu, vp, gap) {
  const top = (t) => Math.max(0, Math.min(t, vp.h - menu.h));
  if (rect.right + gap + menu.w <= vp.w) return { side: 'right', left: rect.right + gap, top: top(rect.top) };
  if (rect.left - gap - menu.w >= 0) return { side: 'left', left: rect.left - gap - menu.w, top: top(rect.top) };
  return { side: 'below', left: Math.max(0, Math.min(rect.left, vp.w - menu.w)), top: rect.bottom + gap };
}

const WORDS = { source: 'Source', destination: 'Destination', take: 'Take', preset: 'Preset', clear: 'Clear', current: 'Current Destination', '': 'Empty' };
export const kindLabel = (kind) => WORDS[kind] ?? kind.charAt(0).toUpperCase() + kind.slice(1);
export const kindChoices = () => [...KINDS, ''].map((kind) => ({ kind, label: kindLabel(kind) }));
