// The plan a Grid panel's confirm shows, from the routes it will send. Pure. Names are the current Router names.
// names is namesFrom (panelrules.js). Items keep the order their destination was first staged. With tally (the last
// GET /tally answer), each item also has now: what the destination has on the take's levels before the take.
export function planOf(routes, names, tally = null) {
  const items = new Map();
  for (const r of routes) {
    if (!items.has(r.dst)) items.set(r.dst, { dst: r.dst, dstName: names.destination(r.dst), groups: [] });
    items.get(r.dst).groups.push({ src: r.src, srcName: names.source(r.src), levels: r.levels });
  }
  const out = [...items.values()];
  if (tally) for (const item of out) item.now = currentlyOf(item, tally, names);
  return { verb: 'take', items: out };
}

// As the main page's bySource (routes.js): one row per source, in level order. src is null when the tally does not
// say (unknown, or outside the Rights) and -1 when nothing is routed. The words are tallyText's (core.js).
export function currentlyOf(item, tally, names) {
  const levels = [...new Set(item.groups.flatMap((g) => g.levels))].sort((a, b) => a - b);
  const on = new Map((tally.routes || []).filter((r) => r.dst === item.dst).map((r) => [r.level, r.src]));
  const rows = new Map();
  for (const l of levels) {
    const src = on.has(l) ? on.get(l) : null;
    if (!rows.has(src)) rows.set(src, []);
    rows.get(src).push(l);
  }
  return [...rows].map(([src, ls]) => ({ src, srcName: src == null ? 'unknown' : src < 0 ? 'nothing' : names.source(src), levels: ls }));
}
