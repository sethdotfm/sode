// What the panels page (/panels) groups, labels and offers. Pure. Panels come from GET /api/v1/panels
// with canEdit set by the server, which is the only place that decides who may change one.
import { cellsIn } from './panelrules.js';
import { wordScore } from './findrules.js';

const same = (a, b) => String(a).toLowerCase() === String(b).toLowerCase();

// Fuzzy score of a panel for a query, 0 when some word matches no field. Each word takes its best field
// (name, short name, owner) with the editor Find's scoring; a name hit outranks an owner hit.
export function panelScore(p, q) {
  const words = String(q || '')
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
  if (!words.length) return 1;
  let total = 0;
  for (const w of words) {
    const best = Math.max(wordScore({ label: p.name || '' }, w), wordScore({ label: p.slug || '' }, w) * 0.9, wordScore({ label: p.owner || '' }, w) * 0.8);
    if (!best) return 0;
    total += best;
  }
  return total;
}

export const matches = (p, q) => panelScore(p, q) > 0;

// Best match first inside a group; the stable sort keeps the list's own order for equal scores and an empty query.
const ranked = (panels, q) =>
  String(q || '').trim()
    ? panels
        .map((p) => [p, panelScore(p, q)])
        .filter(([, s]) => s > 0)
        .sort((a, b) => b[1] - a[1])
        .map(([p]) => p)
    : panels;

// A Trusted user is sent every panel, so grouping by owner is only a view over that list.
export function groupPanels(panels, me, trusted, filter = '') {
  const shown = ranked(panels, filter);
  const mine = shown.filter((p) => same(p.owner, me));
  const others = shown.filter((p) => !same(p.owner, me));
  const out = [];
  if (mine.length) out.push({ title: 'My panels', panels: mine });
  if (!trusted) {
    if (others.length) out.push({ title: 'Shared with me', panels: others });
    return out;
  }
  const owners = [...new Set(others.map((p) => p.owner))].sort((a, b) => a.localeCompare(b));
  for (const o of owners) out.push({ title: o, panels: others.filter((p) => p.owner === o) });
  return out;
}

// Link with key and Push to panels join this list in their own tickets.
export const actionsFor = (p) => (p.canEdit ? ['launch', 'edit', 'share', 'push', 'delete'] : ['launch', 'copy']);

// The People tab. Sharing is only the share field of PUT /api/v1/panels/{id}; the server keeps
// names for Named users only and applies the change to open lists and launches.
export const shareDraft = (share) => ({ mode: share?.mode || 'me', users: share?.mode === 'named' ? [...(share.users || [])] : [] });

// Every user but the owner, ticked when named. A named user missing from the list (a name the
// Users list no longer has) stays shown so it can be unticked.
export function pickList(people, owner, named) {
  const on = new Set(named.map((n) => n.toLowerCase()));
  const out = people.filter((u) => !same(u.name, owner)).map((u) => ({ name: u.name, on: on.has(u.name.toLowerCase()) }));
  for (const n of named) if (!out.some((u) => same(u.name, n)) && !same(n, owner)) out.push({ name: n, on: true });
  return out;
}

export function toggleUser(users, name, on) {
  const rest = users.filter((u) => !same(u, name));
  return on ? [...rest, users.find((u) => same(u, name)) ?? name] : rest;
}

// The panel as PUT wants it. canEdit is the server's answer, and clientRights left out keeps the
// stored ones, so a share edit never touches the API client.
export function shareBody(p, draft) {
  const { canEdit, clientRights, ...rest } = p;
  return { ...rest, share: draft.mode === 'named' ? { mode: 'named', users: [...draft.users] } : { mode: draft.mode } };
}

export function shareChanged(share, draft) {
  const was = shareDraft(share);
  if (was.mode !== draft.mode) return true;
  if (draft.mode !== 'named') return false;
  const key = (u) =>
    u
      .map((n) => n.toLowerCase())
      .sort()
      .join('\n');
  return key(was.users) !== key(draft.users);
}

export function deleteStep(armed, id) {
  return armed === id ? { armed: '', go: true } : { armed: id, go: false };
}

export const thumbCells = (p) => cellsIn(p).map(({ r, c, rs, cs, kind }) => ({ r, c, rs, cs, kind }));

// `acts` `api` and `both` own an API client and hold keys; `both` falls back to Session mode when the page has no key.
// Keep in step with actsHasClient in internal/app/panelstore.go.
export const hasClient = (p) => p?.acts === 'api' || p?.acts === 'both';

// The short form for a row, after the owner's name.
export const actsShort = (p) => (p.acts === 'api' ? 'API client' : p.acts === 'both' ? 'API client or the user' : 'signs in as the user');

export const actsLabel = (p) =>
  p.acts === 'api' ? 'Acts as an API client' : p.acts === 'both' ? 'Acts as an API client with a key, else as the user' : 'Acts as the user';

export function shareLabel(share) {
  if (share?.mode === 'everyone') return 'Everyone';
  if (share?.mode === 'named') return share.users?.length ? share.users.join(', ') : 'Nobody yet';
  return 'Only the owner';
}

export const launchHref = (id) => `panels/${id}`;
export const editHref = (id) => `panels/editor?panel=${id}`;

// What a key does on the list page, for the selected panel: 'launch' (Ctrl/Cmd+Enter), 'edit' (Ctrl/Cmd+E), 'next' or
// 'prev' (Ctrl/Cmd+J, K), else ''. Ctrl on any platform, Cmd only on a Mac (as listStep in keys.js), so the shortcuts work in a field too.
export function listKey(e, mac = false) {
  if (e.defaultPrevented || e.altKey || e.shiftKey) return '';
  if (!((e.ctrlKey && !e.metaKey) || (mac && e.metaKey && !e.ctrlKey))) return '';
  const key = e.code === 'Enter' || e.key === 'Enter' ? 'Enter' : String(e.key || '').toLowerCase();
  return { e: 'edit', Enter: 'launch', j: 'next', k: 'prev' }[key] || '';
}

// Ctrl/Cmd+J and K in the find box: the id the highlight moves to, stopping at the ends.
export function stepPanel(ids, cur, step) {
  if (!ids.length) return '';
  const i = ids.indexOf(cur);
  return ids[Math.min(ids.length - 1, Math.max(0, (i < 0 ? 0 : i) + step))];
}
