// A panel as a panel page runs it: every source and destination on item numbers (routerids.js). So the bodies key
// items of every router by one number. Pure. places is makePlaces's. It is safe on a panel already in this form, since
// the Live Editor hands the run the editor's panel. An item number goes back to itself.
import { edgesIn } from './gridedgerefs.js';
import { rightsIn, placeOf } from './routerids.js';
import { pagesOf } from './panelpages.js';
import { isGridPanel } from './gridpaneledges.js';

const NUMBERED = new Set(['source', 'destination']);

const cellIn = (c, places) => (c && NUMBERED.has(c.kind) && Number.isInteger(c.n) ? { ...c, n: places.id(c.router || '', c.n) } : c);

export function panelIn(panel, places) {
  if (!panel || typeof panel !== 'object') return panel;
  const out = edgesIn(panel, places);
  if (panel.rights) out.rights = rightsIn(panel.rights, places);
  const fixed = panel.settings?.fixedDests;
  if (Array.isArray(fixed)) {
    const on = (x) => (x && typeof x === 'object' && Number.isInteger(x.n) ? { ...x, n: places.id(x.router || '', x.n) } : x);
    out.settings = { ...panel.settings, fixedDests: fixed.map(on) };
  }
  if (Array.isArray(panel.pages) && panel.pages.length) out.pages = panel.pages.map((g) => ({ ...g, cells: (g.cells || []).map((c) => cellIn(c, places)) }));
  if (Array.isArray(panel.cells)) out.cells = panel.cells.map((c) => cellIn(c, places));
  return out;
}

// The places of the routers a panel (in panelIn's form) uses, the first router always among them, in order. An "all"
// edge (Follow the router) uses every router that places lists.
export function placesUsed(panel, places = null) {
  const used = new Set([0]);
  const add = (n) => Number.isInteger(n) && used.add(placeOf(n));
  if (isGridPanel(panel)) {
    for (const e of [panel.sources, panel.destinations]) {
      if (Array.isArray(e?.members)) e.members.forEach(add);
      else (places?.routers || []).forEach((_, i) => used.add(i));
    }
  } else {
    for (const g of pagesOf(panel)) for (const c of g.cells || []) if (NUMBERED.has(c?.kind)) add(c.n);
    for (const x of panel?.settings?.fixedDests || []) add(x && typeof x === 'object' ? x.n : x);
  }
  return [...used].sort((a, b) => a - b);
}
