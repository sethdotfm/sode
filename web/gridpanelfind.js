// Find and Filter on a Grid panel's Source bar and Destination bar. Pure.
// Find marks matches in place and steps through them; Filter shows only matches. Each bar has its own box; one mode
// serves both for the visit, so a change of box keeps it.
// Matching and ranking are the main page's fuzzy scoring, with no cut-off. Find goes to the best match first. A
// Filter shows its matches in panel order.
import { fuzzyScore } from './fuzzy.js';

// at is the router number of the current match, or null for the first one.
const empty = () => ({ text: '', at: null });

// mode is 'find' or 'filter'. last is the box typed in or stepped last.
export const createFind = () => ({ srcs: empty(), dsts: empty(), mode: 'find', last: 'srcs' });

const other = (kind) => (kind === 'srcs' ? 'dsts' : 'srcs');
const hasText = (f, kind) => !!f[kind].text.trim();

// The box n and N on the grid step: the last one, or the other when the last is empty. null when both are empty.
export function stepBox(f) {
  if (hasText(f, f.last)) return f.last;
  return hasText(f, other(f.last)) ? other(f.last) : null;
}

// order is router numbers in display order; list is that edge of GET /names ({ n, name }). offset is the reply's
// numberOffset, so a typed number matches the number as shown. Best first; equal scores keep display order.
export function findMatches(order, list, text, offset = 0) {
  const q = text.trim().toLowerCase();
  if (!q) return [];
  const by = new Map((list || []).map((x) => [x.n, x.name || '']));
  const hits = [];
  order.forEach((n, i) => {
    const s = fuzzyScore({ num: String(n + offset), lname: (by.get(n) ?? '').toLowerCase() }, q);
    if (s >= 0) hits.push([s, i, n]);
  });
  return hits.sort((a, b) => b[0] - a[0] || a[1] - b[1]).map((x) => x[2]);
}

// kind is 'srcs' or 'dsts'.
export const typed = (f, kind, text) => ({ ...f, [kind]: { ...f[kind], text, at: null }, last: kind });

export const clearFind = (f, kind) => ({ ...f, [kind]: { ...f[kind], text: '', at: null } });

export const switchMode = (f) => ({ ...f, mode: f.mode === 'find' ? 'filter' : 'find' });

// matches is findMatches for that box. A current match that went (a live update, a rename) gives the first.
export const currentMatch = (f, kind, matches) => (matches.includes(f[kind].at) ? f[kind].at : (matches[0] ?? null));

// dir 1 is the next match, -1 the one before; both wrap.
export function stepMatch(f, kind, matches, dir) {
  if (!matches.length) return f;
  const i = matches.indexOf(currentMatch(f, kind, matches));
  const next = (i + dir + matches.length) % matches.length;
  return { ...f, [kind]: { ...f[kind], at: matches[next] }, last: kind };
}

// One edge in display order, as the grid shows it. pins is how many lead the order (pinsOf); a Filter never hides
// them, as pinned items stay at the edge with or without a filter.
export function shownOrder(order, pins, f, kind, matches) {
  if (f.mode !== 'filter' || !hasText(f, kind)) return order;
  const on = new Set(matches);
  return [...order.slice(0, pins), ...order.slice(pins).filter((n) => on.has(n))];
}
