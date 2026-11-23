// History rows of tie takes (docs/tielines-design.md, "History, Undo, Redo, Retry"). Pure, no imports, so Node tests
// it. A tie take is one Group. Each hop before the last has an entry (`tie`). Then each destination asked for has an
// entry with `origin` (the typed source) and `hops`.

// isHop: the entry of a hop before the last. It is part of the route asked for, so no row shows it on its own, and
// filters and quick search never match it.
export const isHop = (e) => !!e?.tie && !e.ext;

// typedEnds: the ends that filters and quick search match, each { n, name, router }. For a tie take the source is the
// typed source (origin), on its own router. null for a hop entry.
export function typedEnds(e) {
  if (isHop(e)) return null;
  const src = e.origin
    ? { n: e.origin.n, name: e.origin.name || '', router: e.origin.router || '' }
    : { n: e.src, name: e.srcName || '', router: e.router || '' };
  return { src, dst: { n: e.dst, name: e.dstName || '', router: e.router || '' } };
}

// soloTies: the Groups of list that are a tie take of one destination asked for, as Map group → the id of that entry.
// Such a take is not a Group for stacking: its row is the entry of the destination, and its hop entries go in that row.
// The Group must be whole (groupN entries loaded), since a Protect of the take comes last. A take with a Protect stays
// a Group, as the Protect has its own row.
export function soloTies(list) {
  const by = new Map();
  for (const e of list) {
    if (!e.group || !(e.groupN > 1) || e.ext) continue;
    if (!by.has(e.group)) by.set(e.group, []);
    by.get(e.group).push(e);
  }
  const out = new Map();
  for (const [g, es] of by) {
    const asked = es.filter((e) => !isHop(e));
    if (es.length === es[0].groupN && asked.length === 1 && asked[0].origin && !asked[0].protect) out.set(g, asked[0].id);
  }
  return out;
}

// hopEntriesOf: the hop entries of e, upstream first, from the entries of its Group. A hop with no entry (its tie line
// already carried the source) has none.
export function hopEntriesOf(e, groupEntries) {
  const byId = new Map(groupEntries.filter(isHop).map((x) => [x.id, x]));
  return (e.hops || []).map((h) => byId.get(h.entry)).filter(Boolean);
}

// stackVia: the part of a stack key that keeps tie rows apart. A tie row stacks only with a row through the same tie
// lines; a route on one router has ''.
export const stackVia = (e) => (e.origin ? ` tie ${(e.hops || []).map((h) => h.tie).join(',')}` : '');

// hopsBefore: the open row of a tie take shows what each tie line carried before, upstream first. Each is
// { name, router, entry, groups: [{ src, levels }] }: src -1 is nothing, null is not known. A hop whose tie line already
// carried the source has entry null and no groups.
export function hopsBefore(e) {
  return (e.hops || []).map((h) => {
    const prev = new Map(h.prev || []);
    const groups = new Map();
    if (h.entry != null) {
      for (const u of [...(h.levels || [])].sort((a, b) => a - b)) {
        const s = prev.has(u) ? prev.get(u) : null;
        if (!groups.has(s)) groups.set(s, []);
        groups.get(s).push(u);
      }
    }
    return { name: h.tieName || h.tie, router: h.router || '', entry: h.entry ?? null, groups: [...groups].map(([src, levels]) => ({ src, levels })) };
  });
}
