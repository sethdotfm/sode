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

// The main page's plan items (plans.js) from routesOf: one item for each destination, one group for each source on it.
// Item and group ends are router numbers here; the page turns them into its items.
export function planRoutes(routes) {
  const by = new Map();
  for (const r of routes) {
    if (!by.has(r.dst)) by.set(r.dst, { dst: r.dst, groups: [] });
    by.get(r.dst).groups.push({ src: r.src, levels: r.levels });
  }
  return [...by.values()];
}
