// A Grid panel stores its edges as router references (internal/app/panelgridrefs.go). The panel page and the editor
// work on numbers: { members: 'all' | [n], hide: [n], items: { n: settings }, routers: { n: uid } }. Pure.
// routers keeps the router of each number read, so a save sends each reference back on its router. The first reference
// of a number sets its router. The number form has one entry for each number. So a later reference for that number on
// another router goes into aside, and a save sends it back as it came.

const REF_KEYS = new Set(['router', 'n', 'shown', 'name']);
const LISTS = ['members', 'hide', 'items'];

// Safe on an edge already in number form, since the Live Editor hands the run the editor's own edges.
export function edgeIn(edge) {
  if (!edge || typeof edge !== 'object') return edge;
  const out = { ...edge };
  const routers = { ...(edge.routers || {}) };
  const aside = {};
  // A plain number (number form) is always kept.
  const keep = (list, x) => {
    if (typeof x === 'number') return true;
    const uid = x?.router || '';
    if (!(x.n in routers)) routers[x.n] = uid;
    if (routers[x.n] === uid) return true;
    (aside[list] ||= []).push(x);
    return false;
  };
  for (const list of ['members', 'hide']) {
    if (Array.isArray(edge[list])) out[list] = edge[list].filter((x) => keep(list, x)).map((x) => (typeof x === 'number' ? x : x.n));
  }
  if (Array.isArray(edge.items)) {
    const items = {};
    for (const it of edge.items.filter((x) => keep('items', x))) items[it.n] = Object.fromEntries(Object.entries(it).filter(([k]) => !REF_KEYS.has(k)));
    out.items = items;
  }
  // A blank router is the first router, which edgeOut fills.
  const named = Object.fromEntries(Object.entries(routers).filter(([, uid]) => uid));
  if (Object.keys(named).length) out.routers = named;
  else delete out.routers;
  if (Object.keys(aside).length) out.aside = { ...(edge.aside || {}), ...aside };
  return out;
}

// own is the first router's UID: a number read with no router, or added in the editor, is on it. With own blank the
// reference goes as {n}, and the server puts it on the first router.
export function edgeOut(edge, own = '') {
  if (!edge || typeof edge !== 'object') return edge;
  const routers = edge.routers || {};
  const ref = (n) => {
    const router = routers[n] || own;
    return router ? { router, n } : { n };
  };
  const out = { ...edge };
  delete out.routers;
  delete out.aside;
  if (Array.isArray(edge.members)) out.members = edge.members.map(ref);
  if (Array.isArray(edge.hide)) out.hide = edge.hide.map(ref);
  if (edge.items && !Array.isArray(edge.items)) {
    // An item with no setting left (its pin removed) is not stored.
    out.items = Object.entries(edge.items)
      .map(([n, it]) => ({ ...ref(Number(n)), ...it }))
      .filter((it) => Object.entries(it).some(([k, v]) => !REF_KEYS.has(k) && v !== undefined))
      .sort((a, b) => a.n - b.n);
  }
  // The references kept aside go back only into a list the edge still has: an "all" edge has no members list.
  for (const list of LISTS) {
    const kept = edge.aside?.[list];
    if (kept?.length && Array.isArray(out[list])) out[list] = [...out[list], ...kept];
  }
  if (Array.isArray(out.items) && !out.items.length) delete out.items;
  return out;
}

const withEdges = (panel, fn) =>
  panel && typeof panel === 'object' && (panel.sources || panel.destinations)
    ? { ...panel, sources: fn(panel.sources), destinations: fn(panel.destinations) }
    : panel;

export const edgesIn = (panel) => withEdges(panel, edgeIn);
export const edgesOut = (panel, own = '') => withEdges(panel, (e) => edgeOut(e, own));
