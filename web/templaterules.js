// Templates in the panel editor: what a save sends, how an apply fills the grid, and how the list reads. Pure.
// The server drops cell contents unless `contents` is true (internal/app/paneltemplates.go), so a stored
// template is already unassigned; nothing here strips it again.
import { createGrid, saved } from './gridmodel.js';

export function saveTemplateBody({ name, scope, grid, settings, withSettings = false, withContents = false, pin = '' }) {
  const body = { name: name.trim(), scope, contents: withContents, ...saved(grid) };
  if (withSettings) body.settings = settings || {};
  if (scope === 'everyone' && pin) body.pin = pin;
  return body;
}

// A grid with no selection. Rows and columns come from the template, so applying resizes the panel.
export const templateGrid = (t) => createGrid({ rows: t.rows, cols: t.cols, cells: t.cells });

const byName = (a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });

export function groupTemplates(list) {
  const all = [...(list || [])].sort(byName);
  return { me: all.filter((t) => t.scope === 'me'), everyone: all.filter((t) => t.scope === 'everyone') };
}

export function templateNote(t) {
  const carries = [];
  if (t.settings) carries.push('settings');
  if (t.contents) carries.push('cell contents');
  return `${t.rows} x ${t.cols}, ${carries.length ? `with ${carries.join(' and ')}` : 'cells unassigned'}${t.contents || !t.settings ? '' : ', cells unassigned'}`;
}

// The server answers 403 trusted_required to a user who is not Trusted and sent no right PIN.
export const pinNeeded = (status, code) => status === 403 && code === 'trusted_required';
