// A Grid panel stores its edges as router references (internal/app/panelgridrefs.go). The panel page and the editor
// work on item numbers (routerids.js), which keep the router of each reference. The form is { members: 'all' | [n],
// hide: [n], items: { n: settings } }. Pure. places is makePlaces's: the routers in the order of GET /routers. Without it, the
// first router that a reference names is the first router.
import { makePlaces, placeOf, routerN } from './routerids.js';

const REF_KEYS = new Set(['router', 'n', 'shown', 'name']);

export const UNKNOWN_ROUTER = 'Sode did not give its router list. Not saved.';

// Safe on an edge already in number form, since the Live Editor hands the run the editor's own edges.
export function edgeIn(edge, places = makePlaces()) {
  if (!edge || typeof edge !== 'object') return edge;
  const out = { ...edge };
  // A plain number is already an item number.
  const id = (x) => (typeof x === 'number' ? x : places.id(x?.router || '', x.n));
  for (const list of ['members', 'hide']) {
    if (Array.isArray(edge[list])) out[list] = edge[list].map(id);
  }
  if (Array.isArray(edge.items)) {
    const items = {};
    for (const it of edge.items) items[id(it)] = Object.fromEntries(Object.entries(it).filter(([k]) => !REF_KEYS.has(k)));
    out.items = items;
  }
  return out;
}

// own is places, or the first router's UID. An item of the first router with no UID goes as {n}, which names the one
// router. An item of another router with no UID throws: the server would put it on the first router.
export function edgeOut(edge, own = '') {
  if (!edge || typeof edge !== 'object') return edge;
  const places = own && typeof own === 'object' ? own : null;
  const ref = (id) => {
    const router = places ? places.uid(placeOf(id)) : placeOf(id) === 0 ? own : '';
    if (!router && placeOf(id) !== 0) throw new Error(UNKNOWN_ROUTER);
    return router ? { router, n: routerN(id) } : { n: routerN(id) };
  };
  const out = { ...edge };
  if (Array.isArray(edge.members)) out.members = edge.members.map(ref);
  if (Array.isArray(edge.hide)) out.hide = edge.hide.map(ref);
  if (edge.items && !Array.isArray(edge.items)) {
    // An item with no setting left (its pin removed) is not stored.
    out.items = Object.entries(edge.items)
      .map(([n, it]) => [Number(n), it])
      .sort((a, b) => a[0] - b[0])
      .map(([n, it]) => ({ ...ref(n), ...it }))
      .filter((it) => Object.entries(it).some(([k, v]) => !REF_KEYS.has(k) && v !== undefined));
    if (!out.items.length) delete out.items;
  }
  return out;
}

const withEdges = (panel, fn) =>
  panel && typeof panel === 'object' && (panel.sources || panel.destinations)
    ? { ...panel, sources: fn(panel.sources), destinations: fn(panel.destinations) }
    : panel;

export const edgesIn = (panel, places = makePlaces()) => withEdges(panel, (e) => edgeIn(e, places));
export const edgesOut = (panel, own = '') => withEdges(panel, (e) => edgeOut(e, own));
