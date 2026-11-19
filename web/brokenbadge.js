// The status icon with its text for a Broken or Name moved item in a list (GLOSSARY.md). It imports little, so the
// panels pages can use it.
import { el, icon } from './core.js';
import { statusOf, STATUS_TEXT } from './brokenrules.js';

const ICON = { broken: 'unlink', moved: 'arrow-right-left' };

// null for an item with no marks.
export function statusBadge(status) {
  if (!status) return null;
  const b = el('span', `pb-badge ${status === 'broken' ? 'pb-badge--warn' : 'pb-badge--info'} sode__status-badge`, icon(ICON[status]), STATUS_TEXT[status]);
  b.dataset.status = status;
  return b;
}

// A panel as /api/v1 sends it, with the server's marks.
export const panelStatus = (p) => statusOf({ broken: Array.isArray(p?.broken) ? p.broken : [], moved: Array.isArray(p?.moved) ? p.moved : [] });
