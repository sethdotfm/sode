// What the panels page (/panels) groups, labels and offers. Pure. Panels come from GET /api/v1/panels
// with canEdit set by the server, which is the only place that decides who may change one.
import { cellsIn } from './panelrules.js';

const same = (a, b) => String(a).toLowerCase() === String(b).toLowerCase();

export const matches = (p, q) => {
  const s = String(q || '').trim().toLowerCase();
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

// Share, Link with key and Push to panels join this list in their own tickets.
export const actionsFor = (p) => (p.canEdit ? ['launch', 'edit', 'delete'] : ['launch', 'copy']);

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
