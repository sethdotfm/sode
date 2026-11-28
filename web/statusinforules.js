// The status details' rules (statusinfo.js). Pure, no DOM.
import { durationText } from './panelwatchrules.js';
import { listRows } from './routerlinkrules.js';

// The uptime rule's gate (policy.go, needLog), read from the last access event. A refused uptime writes a locked row to
// the Communication Log, so the page asks only when this is true. Unknown access is no.
export function logReadable(access, locks) {
  if (!access || !access.role || access.locked || access.held) return false;
  return !locks?.log || !!access.user;
}

// Whole minutes from a minute on, since the text stays open while it counts.
export const spanText = (secs) => durationText(secs < 60 ? secs : Math.floor(secs / 60) * 60);

// When the router link last came up, from an uptimeResult. null: not up now, or no record. over: up for the whole range.
export function routerUpSince(data) {
  let state = data.start || '';
  let since = state === 'up' ? null : undefined;
  for (const t of data.trans || []) {
    if (t.t > data.now) break;
    if (t.s === 'up' && state !== 'up') since = t.t;
    state = t.s;
  }
  if (state !== 'up') return null;
  return since == null ? { over: true } : { since };
}

// The hover box (statusinfo.js) never scrolls, since it closes when the pointer leaves it. So it shows at most this
// many router rows.
export const HOVER_ROWS = 8;

// The router rows of the hover box: lost routers first, as in the router list (listRows). With more than max routers it
// shows max - 1 rows, and more counts the rest. So the "And N more" line takes the last row, and N is never 1.
export function hoverRows(list, max = HOVER_ROWS) {
  const rows = listRows(list);
  if (rows.length <= max) return { rows, more: 0 };
  return { rows: rows.slice(0, max - 1), more: rows.length - (max - 1) };
}

// The last line of a capped hover box. It points to the router list, which has every router.
export const moreText = (n) => `And ${n} more. Click the Routers icon to see all.`;
