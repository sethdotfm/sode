// The words of Via (docs/tielines-design.md, Via; GLOSSARY.md). Pure, so node tests run them. The server works out each
// chain; the page only words it.
//
// A Via row is one tally row whose source a tie line feeds (GET /tally, and the page's `viaRows` message). It is
// `{ dst, level, src, via: [{ id, name }], origin: { router, n } | null, mixed, nothing, unknown, loop, levels }`.
// origin is the router UID and number at the start of the chain. It is null for nothing, unknown and loop, and for an
// origin that a Hidden caller cannot see. levels is only on a mixed row. It is the origin of each level upstream of
// the nearest tie line, from level 1 (null for none). levelsRouter is the UID of that router.
//
// label(end) gives the label of an end ({ router, n }), as the page shows it ("A.1 CAM 1").

const sameEnd = (a, b) => !!a && !!b && a.router === b.router && a.n === b.n;

// viaNames: "TIE 3, TIE 9", upstream first.
export const viaNames = (row) => (row?.via || []).map((v) => v.name || v.id).join(', ');

// viaWord: "Via TIE 3, TIE 9". The short Via mark of a Grid crosspoint says only "Via".
export const viaWord = (row) => `Via ${viaNames(row)}`;

// originText: what the destination gets at the start of the chain. '' for an origin that the caller cannot see.
export function originText(row, label) {
  if (row.loop) return '∞ loop';
  if (row.unknown) return 'unknown';
  if (row.nothing) return 'nothing';
  return row.origin ? label(row.origin) : '';
}

// viaText: "A.1 CAM 1 Via TIE 3", "A.1 CAM 1 Via TIE 3, TIE 9 (mixed)", "unknown Via TIE 3", "nothing Via TIE 3".
export function viaText(row, label) {
  const o = originText(row, label);
  return `${o ? `${o} ` : ''}${viaWord(row)}${row.mixed ? ' (mixed)' : ''}`;
}

// The same chain and the same marks: two levels with the same key show as one row.
const viaKey = (r) => (r ? JSON.stringify([r.via, r.origin, !!r.mixed, !!r.nothing, r.unknown || '', !!r.loop, r.levels || null]) : '');

// viaGroups groups levels by source and by Via. srcAt(level) is the source (null unknown, -1 nothing); viaAt(level)
// is the Via row of that level, or null. Gives `[{ src, via, levels }]` in the order of levels. A group has `via` only
// when there is one.
export function viaGroups(levels, srcAt, viaAt) {
  const groups = new Map();
  for (const u of levels) {
    const src = srcAt(u);
    const via = viaAt(u) || null;
    const k = `${src} ${viaKey(via)}`;
    if (!groups.has(k)) groups.set(k, via ? { src, via, levels: [] } : { src, levels: [] });
    groups.get(k).levels.push(u);
  }
  return [...groups.values()];
}

// pieGroups: the level pie of a mixed row, as [{ levels, origin }]: the levels upstream of the nearest tie line that
// carry each origin. origin null is nothing, unknown or a loop on those levels.
export function pieGroups(row) {
  const out = [];
  (row?.levels || []).forEach((o, i) => {
    const g = out.find((x) => (o ? sameEnd(x.origin, o) : !x.origin));
    if (g) g.levels.push(i + 1);
    else out.push({ origin: o || null, levels: [i + 1] });
  });
  return out;
}

// fedThrough gives every destination that source src ({ router, n }) feeds through tie lines. routers is
// `[[uid, via]]`, each `via` a Map dst → Map level → Via row. Gives [{ router, dst, levels, row }] in the order of routers,
// then by dst. A mixed row counts on the levels whose origin is src.
export function fedThrough(src, routers) {
  const out = [];
  for (const [uid, via] of routers) {
    for (const dst of [...via.keys()].sort((a, b) => a - b)) {
      const rows = [...via.get(dst)].filter(([, r]) => sameEnd(r.origin, src)).sort(([a], [b]) => a - b);
      if (rows.length) out.push({ router: uid, dst, levels: rows.map(([l]) => l), row: rows[0][1] });
    }
  }
  return out;
}

// viaOn: the Via row of a Grid crosspoint. That is the row of the lowest level on which dst has src, when a tie line
// feeds it. env is envFrom's (panelrules.js). null for none.
export function viaOn(env, src, dst) {
  const t = env.tally.get(dst);
  const v = env.via?.get(dst);
  if (!t || !v) return null;
  const l = [...t]
    .filter(([u, s]) => s === src && v.has(u))
    .map(([u]) => u)
    .sort((a, b) => a - b)[0];
  return l === undefined ? null : v.get(l);
}

// hopsVia: the Via of a tie take's History entry, from its hops: "Via TIE 3, TIE 9". '' for an entry with no hops.
export const hopsVia = (e) => (e?.origin && e.hops?.length ? viaWord({ via: e.hops.map((h) => ({ id: h.tie, name: h.tieName })) }) : '');

// alsoNote: an External on the upstream end of a tie line. "also changes B.42, B.50 Via TIE 3; C.1 Via TIE 3, TIE 9".
// label(also) gives the label of one destination of e.also ({ router, n, name }). alsoOthers counts the ones that a
// Hidden caller cannot see. '' for an entry with no also.
export function alsoNote(e, label) {
  const also = e?.also || [];
  const others = e?.alsoOthers || 0;
  if (!also.length && !others) return '';
  const byVia = new Map();
  for (const a of also) {
    const k = viaWord(a);
    if (!byVia.has(k)) byVia.set(k, []);
    byVia.get(k).push(label(a));
  }
  const parts = [...byVia].map(([via, names]) => `${names.join(', ')} ${via}`);
  if (others) parts.push(`${others} ${others === 1 ? 'other' : 'others'}`);
  return `also changes ${parts.join('; ')}`;
}
