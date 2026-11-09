// What the panels page (/panels) groups, labels and offers. Pure. Panels come from GET /api/v1/panels
// with canEdit set by the server, which is the only place that decides who may change one.
import { cellsIn } from './panelrules.js';

const same = (a, b) => String(a).toLowerCase() === String(b).toLowerCase();

export const matches = (p, q) => {
  const s = String(q || '')
    .trim()
    .toLowerCase();
  return !s || p.name.toLowerCase().includes(s) || p.owner.toLowerCase().includes(s);
};

// A Trusted user is sent every panel, so grouping by owner is only a view over that list.
export function groupPanels(panels, me, trusted, filter = '') {
  const shown = panels.filter((p) => matches(p, filter));
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

export const actsLabel = (p) => (p.acts === 'api' ? 'Acts as an API client' : 'Acts as the user');

export function shareLabel(share) {
  if (share?.mode === 'everyone') return 'Everyone';
  if (share?.mode === 'named') return share.users?.length ? share.users.join(', ') : 'Nobody yet';
  return 'Only the owner';
}

export const launchHref = (id) => `panels/${id}`;
export const editHref = (id) => `panels/editor?panel=${id}`;
