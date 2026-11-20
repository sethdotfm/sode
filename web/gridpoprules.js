// The Grid popup's rules (gridpopup.js). Pure: no DOM, no state.
// The popup runs the Grid panel body (gridpanelrun.js), which reads /api/v1 answers. On the main page the socket already
// keeps the tally, the protected list and the names, so these build the same answers from that state.

// GET /tally. levels are the hello's ({ ui, label, swatch, color }); tally is dst -> (level -> src).
export function tallyReplyOf({ levels, levelNumbering, defaultLevels, tally }) {
  const routes = [];
  for (const [dst, t] of tally) for (const [level, src] of t) routes.push({ dst, level, src });
  return {
    levels: levels.map((l) =>
      l.color ? { level: l.ui, label: l.label, swatch: l.swatch, color: l.color } : { level: l.ui, label: l.label, swatch: l.swatch },
    ),
    levelNumbering,
    defaultLevels,
    routes,
  };
}

// GET /protected. prot is dst -> (level -> { dev, kind }).
export function protectedReplyOf(prot) {
  const rows = [];
  for (const [dst, t] of prot) for (const [level, { dev }] of t) rows.push({ dst, level, device: dev });
  return { protected: rows };
}

// GET /names. srcs and dsts are the main page's items ({ n, name }).
export const namesReplyOf = ({ srcs, dsts, numberOffset }) => ({
  numberOffset,
  sources: srcs.map(({ n, name }) => ({ n, name })),
  destinations: dsts.map(({ n, name }) => ({ n, name })),
});

// Where the popup opens. src and dst are router numbers from the patch boxes, or null for a box with no pick. dst is the
// first destination. first is true while this tab has no popup state yet.
// Returns null to bring the popup back as it was left. Otherwise { src, dst, levels }: aim at that crosspoint, or at
// the one item when the other is null. levels is true when the Levels box takes the main page's levels.
export function aimOf({ src, dst, first }) {
  const pair = src != null && dst != null;
  if (pair) return { src, dst, levels: true };
  if (src == null && dst == null) return first ? { src: null, dst: null, levels: true } : null;
  return { src, dst, levels: first };
}

const sameList = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);

// The staged list under the patch boxes: one row for each source and level set, in the order first staged. routes is
// routesOf (gridpanelstage.js): { src, dst, levels }.
export function stagedRows(routes) {
  const rows = [];
  for (const r of routes) {
    let row = rows.find((x) => x.src === r.src && sameList(x.levels, r.levels));
    if (!row) rows.push((row = { src: r.src, levels: r.levels, dsts: [] }));
    row.dsts.push(r.dst);
  }
  return rows;
}

export const routesWord = (n) => (n === 1 ? '1 route' : `${n} routes`);

// One Take (desktop.js): the staged routes with the patch boxes' route, as one list of routes. box is { src, dsts,
// levels } in router numbers. On a destination in both, the boxes' levels replace the staged ones, as a later press on
// the grid does. The same source on the same destination stays one route.
export function withBoxRoute(routes, { src, dsts, levels }) {
  const out = [];
  for (const r of routes) {
    const left = dsts.includes(r.dst) ? r.levels.filter((l) => !levels.includes(l)) : r.levels;
    if (left.length) out.push({ ...r, levels: left });
  }
  for (const dst of dsts) {
    const same = out.find((r) => r.src === src && r.dst === dst);
    if (same) same.levels = [...same.levels, ...levels].sort((a, b) => a - b);
    else out.push({ src, dst, levels: [...levels] });
  }
  return out;
}

// How many routes the main page's Take sends (desktop.js): the staged routes, with the boxes' route when they hold one
// (withBoxRoute). Without staged routes it is 0: a plain patch says Take with no count.
export const takeCount = (routes, box) => (!routes.length ? 0 : box ? withBoxRoute(routes, box).length : routes.length);
