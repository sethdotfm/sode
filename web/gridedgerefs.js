// A Grid panel stores its edges as router references (internal/app/panelgridrefs.go). The panel page and the editor
// work on numbers: { members: 'all' | [n], hide: [n], items: { n: settings } }. Pure.
// The server fills each reference's router and name, so the editor sends only n; with one router that loses nothing.

const numOf = (x) => (typeof x === 'number' ? x : x?.n);
const REF_KEYS = new Set(['router', 'n', 'shown', 'name']);

// Safe on an edge already in number form, since the Live Editor hands the run the editor's own edges.
export function edgeIn(edge) {
  if (!edge || typeof edge !== 'object') return edge;
  const out = { ...edge };
  if (Array.isArray(edge.members)) out.members = edge.members.map(numOf);
  if (Array.isArray(edge.hide)) out.hide = edge.hide.map(numOf);
  if (Array.isArray(edge.items)) {
    const items = {};
    for (const it of edge.items) items[it.n] = Object.fromEntries(Object.entries(it).filter(([k]) => !REF_KEYS.has(k)));
    out.items = items;
  }
  return out;
}

export function edgeOut(edge) {
  if (!edge || typeof edge !== 'object') return edge;
  const ref = (n) => ({ n });
  const out = { ...edge };
  if (Array.isArray(edge.members)) out.members = edge.members.map(ref);
  if (Array.isArray(edge.hide)) out.hide = edge.hide.map(ref);
  if (edge.items && !Array.isArray(edge.items)) {
    // An item with no setting left (its pin removed) is not stored.
    out.items = Object.entries(edge.items)
      .map(([n, it]) => ({ n: Number(n), ...it }))
      .filter((it) => Object.entries(it).some(([k, v]) => k !== 'n' && v !== undefined))
      .sort((a, b) => a.n - b.n);
    if (!out.items.length) delete out.items;
  }
  return out;
}

const withEdges = (panel, fn) =>
  panel && typeof panel === 'object' && (panel.sources || panel.destinations)
    ? { ...panel, sources: fn(panel.sources), destinations: fn(panel.destinations) }
    : panel;

export const edgesIn = (panel) => withEdges(panel, edgeIn);
export const edgesOut = (panel) => withEdges(panel, edgeOut);
