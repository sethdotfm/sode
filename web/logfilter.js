// /log filter and sort rules. Pure (node tests). Download filtered rows uses them too, on every row the server
// sends (logdownload.js), so the server has no copy of them. web/test/log_filter.json holds the cases.
import { KINDS, EVENT_KINDS, WHY_LABEL, kindLabel, eventProblem } from './logkinds.js';
import { filterFn } from './filter.js';
import { rowPos, byNewest } from './logpager.js';

export const isEvent = (e) => EVENT_KINDS.has(e.kind);
export const problem = (e) => (isEvent(e) ? eventProblem(e) : e.result !== 'ack' || e.tries > 1);

const anything = () => true;

// Source and destination matchers for f. Text that does not parse matches everything, as on the page.
export const logMatch = (f) => ({ src: filterFn(f.src) || anything, dst: filterFn(f.dst) || anything });

// f: { kinds, problems, src, dst, from, to }. from and to are in ms or null; to counts to its own end. off: numberOffset.
export function logPasses(e, f, off = 0, m = logMatch(f)) {
  const t = e.t ?? Date.parse(e.time);
  if (f.from != null && t < f.from) return false;
  if (f.to != null && t > f.to) return false;
  const kind = KINDS.some((g) => g.kinds.includes(e.kind)) ? e.kind : 'other';
  const kinds = KINDS.some((g) => f.kinds?.includes(g.id)) ? f.kinds : KINDS.map((g) => g.id);
  if (!KINDS.some((g) => g.kinds.includes(kind) && kinds.includes(g.id))) return false;
  if (f.problems && !problem(e)) return false;
  if (f.src?.trim() && (e.src < 0 || !m.src(e.src + off, e.srcName || ''))) return false;
  if (f.dst?.trim() && (e.dst < 0 || !m.dst(e.dst + off, e.dstName || ''))) return false;
  return true;
}

// Sorting: ties keep newest first. Rows without the column go last in both directions.
const resultRank = (e) => (isEvent(e) ? (problem(e) ? 2 : 0) : e.result === 'ack' ? (e.tries > 1 ? 1 : 0) : 2);

export function sortKeys() {
  return {
    time: rowPos,
    message: kindLabel,
    level: (e) => (e.level ? e.level : e.wire >= 0 ? 1000 + e.wire : null),
    dst: (e) => (e.dst >= 0 ? e.dst : null),
    src: (e) => (e.src >= 0 ? e.src : null),
    why: (e) => (e.why ? `${WHY_LABEL[e.why] || e.why} ${e.by || ''}` : null),
    result: (e) => [resultRank(e), e.result, e.tries],
    hex: (e) => e.hex,
  };
}

export const newestFirst = (sort) => sort.col === 'time' && sort.dir === 'desc';

function cmp(a, b) {
  if (Array.isArray(a)) {
    for (let i = 0; i < a.length; i++) {
      const c = cmp(a[i], b[i]);
      if (c) return c;
    }
    return 0;
  }
  if (typeof a === 'string') return a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' });
  return a - b;
}

export function cmpKeyed(sort, x, xv, y, yv) {
  if (xv == null || yv == null) return xv == null ? (yv == null ? byNewest(x, y) : 1) : -1;
  return (sort.dir === 'asc' ? 1 : -1) * cmp(xv, yv) || byNewest(x, y);
}

// rows in the page's order. keys: sortKeys().
export function sortRows(rows, sort, keys) {
  if (newestFirst(sort) || !keys[sort.col]) return rows.slice().sort(byNewest);
  const key = keys[sort.col];
  return rows
    .map((e) => ({ e, v: key(e) }))
    .sort((x, y) => cmpKeyed(sort, x.e, x.v, y.e, y.v))
    .map((x) => x.e);
}
