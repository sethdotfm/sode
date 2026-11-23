// Few imports, so the panels pages can use it.
import { el, icon } from './core.js';
import { statusOf, STATUS_TEXT } from './brokenrules.js';

const ICON = { broken: 'unlink', moved: 'arrow-right-left' };

// null for an item with no marks.
export function statusBadge(status) {
  if (!status) return null;
  const b = el('span', `pb-badge${status === 'broken' ? ' pb-badge--warn' : ''} sode__status-badge`, icon(ICON[status]), STATUS_TEXT[status]);
  b.dataset.status = status;
  return b;
}

export const panelStatus = (p) => statusOf({ broken: Array.isArray(p?.broken) ? p.broken : [], moved: Array.isArray(p?.moved) ? p.moved : [] });
