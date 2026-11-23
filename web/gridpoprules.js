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

const many = (n, one) => `${n} ${one}${n === 1 ? '' : 's'}`;

// The popup's note after a click adds to a preset. r is the body's { kind, n, items, outside } (gridpanelnow.js), rows
// what the editor added. src(n) and dst(n) are labels.
export function addedLine(r, rows, { src, dst }) {
  const left = r.outside ? ` ${many(r.outside, 'level')} left out: the source is not in your Rights.` : '';
  if (r.kind === 'xp') return `Added ${src(r.n.src)} to ${dst(r.n.dst)}.`;
  if (r.kind === 'dsts') {
    const from = r.items.length === 1 ? src(r.items[0].src) : many(r.items.length, 'source');
    return `Added ${dst(r.n)} from ${from}.${left}`;
  }
  const n = r.items.reduce((k, it) => k + it.dsts.length, 0);
  const to = n === 1 ? dst(r.items[0].dsts[0]) : many(n, 'destination');
  return `Added ${src(r.n)} to ${to}${rows > 1 ? ` (${many(rows, 'row')})` : ''}.`;
}

// The note when a click on a name finds no route to add.
export function noRouteLine(r, { src, dst }) {
  if (r.kind === 'srcs') return `${src(r.n)} goes to no destination on these levels.`;
  return r.outside ? `${dst(r.n)} gets its source from outside your Rights.` : `${dst(r.n)} has no source on these levels.`;
}

// The preset items a press adds. A crosspoint stores what the boxes' Add stores: no levels (the default levels) while
// the main Levels box is empty (boxLevels []). A name stores the levels its routes have now.
export const storedItems = (r, boxLevels) => (r.kind === 'xp' && !boxLevels.length ? [{ ...r.items[0], levels: [] }] : r.items);

export const rowsWord = (n) => `${many(n, 'row')} in the preset`;
