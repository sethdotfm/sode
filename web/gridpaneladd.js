// A press policy for the Grid panel body (gridpanelrun.js pressMode). A press adds routes to a preset and stages
// nothing, so nothing is sent. A crosspoint adds its route; a name adds the routes it has now (gridpanelnow.js).
// The Grid popup makes it while a preset adds patches (presetadd.js). Pure apart from the host's functions.
import { dstNow, srcNow } from './gridpanelnow.js';

// The rows of a preset as marks: each route's "src,dst", and the sources and destinations that have a row.
export function presetMarks(items) {
  const m = { rows: items.length, xps: new Set(), srcs: new Set(), dsts: new Set() };
  for (const it of items) {
    m.srcs.add(it.src);
    for (const d of it.dsts) {
      m.dsts.add(d);
      m.xps.add(`${it.src},${d}`);
    }
  }
  return m;
}

// What a press on kind ('xp', 'srcs' or 'dsts') and n adds. w is the run's { levels, tally, srcs, dsts }. levels are the
// press levels and tally is envFrom's. srcs is the Set of sources on the grid, dsts the grid's destinations in order.
export function pressItems(kind, n, w) {
  if (kind === 'xp') return { items: [{ src: n.src, dsts: [n.dst], levels: w.levels }], outside: 0 };
  if (kind === 'dsts') return dstNow(w.tally, n, w.levels, (s) => w.srcs.has(s));
  return srcNow(w.tally, n, w.levels, w.dsts);
}

// host: { add(r) -> { ok, text }, rows() -> preset items, words: { name, route } }. r is { kind, n, levels, items,
// outside }. marks() is presetMarks with the words: words.name marks a name with a row, words.route ends a marked
// crosspoint's label. It keeps one object until fresh(), after the rows change, so the run repaints on a new one.
export function addPolicy(host) {
  let marks = null;
  return {
    marks: () => (marks ||= { ...presetMarks(host.rows()), words: host.words }),
    fresh() {
      marks = null;
    },
    press: (kind, n, w) => host.add({ kind, n, levels: w.levels, ...pressItems(kind, n, w) }),
  };
}
