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

// Where the smart menu goes. rect and vp are in pixels. Right of the selection, else left, else below.
export function placeMenu(rect, menu, vp, gap) {
  const top = (t) => Math.max(0, Math.min(t, vp.h - menu.h));
  if (rect.right + gap + menu.w <= vp.w) return { side: 'right', left: rect.right + gap, top: top(rect.top) };
  if (rect.left - gap - menu.w >= 0) return { side: 'left', left: rect.left - gap - menu.w, top: top(rect.top) };
  return { side: 'below', left: Math.max(0, Math.min(rect.left, vp.w - menu.w)), top: rect.bottom + gap };
}

// One letter per menu item, from its name: the first of its letters no earlier item took, else the first free
// letter of the alphabet. Names are stable (Merge keeps M while it reads Unmerge), so a letter never moves.
export function menuLetters(names) {
  const taken = new Set();
  return names.map((name) => {
    const pool = [...name.toLowerCase().replace(/[^a-z]/g, ''), ...'abcdefghijklmnopqrstuvwxyz'];
    const l = pool.find((c) => !taken.has(c));
    taken.add(l);
    return l;
  });
}

// The item a letter activates: never a disabled one.
export const menuTarget = (items, letter) => items.find((it) => it.letter === letter && !it.disabled) ?? null;

export const todoText = (n) => (n ? `${n} unassigned square${n > 1 ? 's' : ''}` : 'No unassigned squares');

const WORDS = { source: 'Source', destination: 'Destination', take: 'Take', preset: 'Preset', clear: 'Clear', current: 'Current Destination', '': 'Empty' };
export const kindLabel = (kind) => WORDS[kind] ?? kind.charAt(0).toUpperCase() + kind.slice(1);
export const kindChoices = () => [...KINDS, ''].map((kind) => ({ kind, label: kindLabel(kind) }));

// The tip beside the grip while it is dragged: how many squares, and the first and last name placed.
export function gripTip(plan, nameOf) {
  if (!plan) return '';
  const n = plan.slots.length;
  if (!n) return 'Nothing to add here';
  const name = (s) => nameOf({ kind: s.kind, n: s.n });
  return `${n} square${n > 1 ? 's' : ''}, ${n > 1 ? `${name(plan.slots[0])} to ${name(plan.slots[n - 1])}` : name(plan.slots[0])}`;
}
