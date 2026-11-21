// The routes a name on the grid has now, as preset items { src, dsts, levels } (presets.md). Pure.
// tally is envFrom's (panelrules.js): Map dst -> Map level -> src. levels are the chosen levels, never [].
// A level with no source on it is skipped. ok(n) says a source or destination is on the grid (the Rights).

// One item per source the destination gets, in level order: a breakaway gives more than one.
export function dstNow(tally, dst, levels, ok = () => true) {
  const row = tally.get(dst);
  const by = new Map();
  let outside = 0;
  for (const l of [...levels].sort((a, b) => a - b)) {
    const src = row?.get(l);
    if (src == null || src < 0) continue;
    if (!ok(src)) {
      outside++;
      continue;
    }
    if (!by.has(src)) by.set(src, []);
    by.get(src).push(l);
  }
  return { items: [...by].map(([src, ls]) => ({ src, dsts: [dst], levels: ls })), outside };
}

// One item per set of levels: the destinations that get src on those levels and no others share it. dsts is the
// grid's order.
export function srcNow(tally, src, levels, dsts) {
  const by = new Map();
  const sorted = [...levels].sort((a, b) => a - b);
  for (const d of dsts) {
    const row = tally.get(d);
    const on = row ? sorted.filter((l) => row.get(l) === src) : [];
    if (!on.length) continue;
    const k = on.join(',');
    if (!by.has(k)) by.set(k, { src, dsts: [], levels: on });
    by.get(k).dsts.push(d);
  }
  return { items: [...by.values()], outside: 0 };
}
