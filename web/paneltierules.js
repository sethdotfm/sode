// A panel's routes across routers (docs/tielines-design.md, Panels). Pure: no DOM, no clock, no network.
//
// A panel's tie line setting is Off or Reserved (`tieLines` on the panel read). At Off a press refuses a route across
// routers. At Reserved the panel sends it on its Reserved tie lines, and the server always uses those. The page asks
// the plan first: POST /takes/plan answers 409 tie_line_needed with the one path through the Reserved tie
// lines. The take then sends the ties of that path, with what each tie line carries now and the Affected destinations.
// Thus the stale check of the server compares what the confirm showed. A panel never shows the Tie line picker.
import { acrossRouters } from './routerids.js';
import { expectOf } from './tiepickrules.js';

// tiesOn: the panel's setting is Reserved, so a press can stage a route across routers.
export const tiesOn = (panel) => panel?.tieLines?.mode === 'reserved';

// crossing: some route of the take goes across routers. Routes are on item numbers.
export const crossing = (routes) => (routes || []).some((r) => acrossRouters(r.src, r.dst));

const ref = (e) => ({ router: e.router, n: e.n });

// planOf reads the 409 tie_line_needed body: for each route of the request by its place (route), the first path. A
// route with no path is a no_path, which the server answers with its own code. null when the body has no route.
export function planOf(body) {
  const out = new Map();
  for (const r of body?.routes || []) {
    const p = r.paths?.[0];
    if (!p) return null;
    out.set(r.route, {
      ties: p.hops.slice(0, -1).map((h) => ({ id: h.tie.id, expect: expectOf(h.now) })),
      affected: (p.affected || []).map(ref),
      // For the confirm: the names of the Affected destinations, and how many the caller cannot see.
      shown: (p.affected || []).map((e) => e.name || `${e.n}`),
      others: p.others || 0,
      via: p.hops
        .slice(0, -1)
        .map((h) => h.tie?.name || h.tie?.id)
        .filter(Boolean),
    });
  }
  return out.size ? out : null;
}

// withTies puts each route's ties and Affected destinations on the wire routes of the take (by place).
export const withTies = (wire, plan) =>
  wire.map((r, i) => {
    const p = plan?.get(i);
    return p ? { ...r, ties: p.ties, affected: p.affected } : r;
  });

// storedClash: a Preset cell's route keeps the tie lines that the preset stores (ties, ids). The panel sends only on its
// Reserved tie lines (docs/tielines-design.md, Presets, rule 6). So a route whose stored tie lines are not the path of
// the plan is refused before Sode sends anything. Gives the refusal, or ''.
export function storedClash(routes, plan) {
  for (const [i, r] of (routes || []).entries()) {
    const p = plan?.get(i);
    if (!r.ties?.length || !p) continue;
    if (p.ties.map((t) => t.id).join(' ') !== r.ties.join(' ')) {
      return `The preset sends through tie lines that this panel does not Reserve (the panel uses ${p.via.join(', ')}). Sode sent nothing.`;
    }
  }
  return '';
}

// bare drops the stored tie lines of each route, since a take sends the ties of the plan (withTies).
export const bare = (routes) =>
  routes.map((r) => {
    const o = { ...r };
    delete o.ties;
    return o;
  });

// affectedLines are the lines that the confirm of a Take mode panel adds: each tie line and what it also changes
// (docs/tielines-design.md, Panels, rule 11). One line for each route across routers.
export function affectedLines(plan) {
  const out = [];
  for (const p of plan?.values() || []) {
    const via = `Via ${p.via.join(', ')}`;
    const n = p.shown.length + p.others;
    if (!n) {
      out.push(`${via}: no other destination changes`);
      continue;
    }
    const names = [...p.shown, ...(p.others ? [`${p.others} ${p.others === 1 ? 'other' : 'others'}`] : [])];
    out.push(`${via} also changes ${names.join(', ')}`);
  }
  return out;
}

// acrossOffText is the refusal of a press at Off. Keep in step with errorText in panelrules.js.
export const acrossOffText = (src, dst) => `${src} and ${dst} are on two routers. Tie lines are Off for this panel.`;

// The editor (editorties.js). tieLines is GET /tielines: each { id, name, from, to, reserve }. A link is one way
// between two routers: { key, from, to, ties }, in the order of the first tie line on it.
export function linksOf(tieLines) {
  const out = [];
  for (const t of tieLines || []) {
    if (!t.from || !t.to) continue;
    const key = `${t.from.router} ${t.to.router}`;
    let l = out.find((x) => x.key === key);
    if (!l) out.push((l = { key, from: t.from.router, to: t.to.router, ties: [] }));
    l.ties.push(t);
  }
  return out;
}

// reservedOn is the id of the tie line that setting Reserves on link, or ''.
export const reservedOn = (setting, link) => link.ties.find((t) => setting?.reserved?.includes(t.id))?.id || '';

// setReserve Reserves id on link ('' for none) and keeps the other links as they were.
export function setReserve(setting, link, id) {
  const others = (setting?.reserved || []).filter((x) => !link.ties.some((t) => t.id === x));
  return { mode: 'reserved', reserved: id ? [...others, id] : others };
}

// heldBy is the panel that Reserves tie line t, when it is not panel id: that panel keeps it (one panel only).
export const heldBy = (t, id) => (t.reserve && t.reserve.id !== id ? t.reserve.name : '');

// tiesKey is the setting as the unsaved-change check compares it: Off is no setting.
export const tiesKey = (setting) => (setting?.mode === 'reserved' ? `reserved:${[...(setting.reserved || [])].sort().join(',')}` : '');
