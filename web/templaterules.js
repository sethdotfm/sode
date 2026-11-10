// Templates in the panel editor: what a save sends, how an apply fills the grid, and how the list reads. Pure.
// The server keeps a Source's or Preset's contents only when `sources` is true and a Destination's only when `dests` is
// (internal/app/paneltemplates.go), so a stored template is already unassigned; nothing here strips it again.
import { createGrid, saved } from './gridmodel.js';
import { pagesOf, pageCells, storedPages } from './panelpages.js';

// grid is Page 1 and pages the panel's Page list: a template carries every Page with its id.
export function saveTemplateBody({ name, scope, grid, pages = pagesOf({}), settings, withSettings = false, withSources = false, withDests = false, pin = '' }) {
  const { rows, cols, cells } = saved(grid);
  const body = { name: name.trim(), scope, sources: withSources, dests: withDests, rows, cols, pages: storedPages(pages, cells, rows, cols) };
  if (withSettings) body.settings = settings || {};
  if (scope === 'everyone' && pin) body.pin = pin;
  return body;
}

// Page 1 as a grid with no selection. Rows and columns come from the template, so applying resizes the panel.
export const templateGrid = (t) => createGrid({ rows: t.rows, cols: t.cols, cells: pageCells(t) });

const byName = (a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });

export function groupTemplates(list) {
  const all = [...(list || [])].sort(byName);
  return { me: all.filter((t) => t.scope === 'me'), everyone: all.filter((t) => t.scope === 'everyone') };
}

export function templateNote(t) {
  const parts = [`${t.rows} x ${t.cols}`];
  if (t.settings) parts.push('with settings');
  if (t.sources && t.dests) parts.push('with Sources and Destinations');
  else if (t.sources) parts.push('with Sources, Destinations unassigned');
  else if (t.dests) parts.push('with Destinations, Sources unassigned');
  else parts.push('cells unassigned');
  return parts.join(', ');
}

// The server answers 403 trusted_required to a user who is not Trusted and sent no right PIN.
export const pinNeeded = (status, code) => status === 403 && code === 'trusted_required';
