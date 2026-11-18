// Drawn lines on a Grid panel. Pure. A point is { x, y } in crosspoint units from the first crosspoint's
// top left, in display order (gridAxes). So a line holds while the grid scrolls under it. A cell is { i, j } as in
// gridpanelcursor.js.
import { fromHere } from './gridpanelpick.js';
import { stageable } from './gridpanelrights.js';
import { stageRoutes } from './gridpanelstage.js';

// Under half a crosspoint a drag stays a click.
export const CLICK = 0.5;

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

export const isLine = (a, b) => Math.hypot(b.x - a.x, b.y - a.y) >= CLICK;

// Content px to units. m is the view's metrics(): xp a crosspoint, w the Destination bar width, h the Source bar height.
export const unitsOf = (x, y, m) => ({ x: (x - m.w) / m.xp, y: (y - m.h) / m.xp });

// One cell per row spanned, start row first, at the column where the line crosses the middle of that row. The end
// rows use the end point when the line does not reach the middle. A line that ends on its start row is no cells.
export function lineCells(a, b, { rows, cols }) {
  const i0 = clamp(Math.floor(a.y), 0, rows - 1);
  const i1 = clamp(Math.floor(b.y), 0, rows - 1);
  if (i0 === i1) return [];
  const lo = Math.min(a.y, b.y);
  const hi = Math.max(a.y, b.y);
  const step = i1 > i0 ? 1 : -1;
  const out = [];
  for (let i = i0; i !== i1 + step; i += step) {
    const y = clamp(i + 0.5, lo, hi);
    const x = a.x + ((b.x - a.x) * (y - a.y)) / (b.y - a.y);
    out.push({ i, j: clamp(Math.floor(x), 0, cols - 1) });
  }
  return out;
}

// The slopes a line snaps to, so its targets are evenly spaced: { cols, rows } per step, from the angle off vertical.
// Steps are k columns per row (k >= 0) or one column per k rows (k >= 2). A slope needs room in the drawn line, so a
// small wobble off vertical never snaps to one column per many rows.
function snapStep(dx, dy) {
  const t = Math.abs(dx) / Math.abs(dy);
  const want = Math.atan(t);
  const steps = [0, 1, Math.floor(t), Math.ceil(t)].filter((m) => m <= Math.max(1, Math.ceil(Math.abs(dx)))).map((m) => ({ cols: m, rows: 1 }));
  if (t > 0) {
    for (const k of [Math.floor(1 / t), Math.ceil(1 / t)]) if (k >= 2 && k <= Math.floor(Math.abs(dy))) steps.push({ cols: 1, rows: k });
  }
  const off = (s) => Math.abs(Math.atan(s.cols / s.rows) - want);
  const best = steps.reduce((x, y) => (off(y) < off(x) ? y : x));
  return { cols: best.cols * Math.sign(dx), rows: best.rows * Math.sign(dy) };
}

// A line with nothing picked, snapped to evenly spaced targets. cells is one per step, start first, and stops at the
// grid edge. a and b are the centres of the first and last target, the line the preview draws. A line that ends on
// its start row is no cells.
export function snapLine(a, b, { rows, cols }) {
  const i0 = clamp(Math.floor(a.y), 0, rows - 1);
  const j0 = clamp(Math.floor(a.x), 0, cols - 1);
  const c = { x: j0 + 0.5, y: i0 + 0.5 };
  const none = { cells: [], a: c, b: c };
  if (clamp(Math.floor(b.y), 0, rows - 1) === i0) return none;
  const dx = b.x - c.x;
  const dy = b.y - c.y;
  const st = snapStep(dx, dy);
  const n = Math.round((dx * st.cols + dy * st.rows) / (st.cols * st.cols + st.rows * st.rows));
  const cells = [];
  for (let k = 0; k <= n; k++) {
    const i = i0 + k * st.rows;
    const j = j0 + k * st.cols;
    if (i < 0 || i >= rows || j < 0 || j >= cols) break;
    cells.push({ i, j });
  }
  if (cells.length < 2) return none;
  const last = cells.at(-1);
  return { cells, a: c, b: { x: last.j + 0.5, y: last.i + 0.5 } };
}

// The routes a line stages, as [{ src, dst }] in drag order. With nothing picked, the snapped targets (snapLine).
// With picks, the Diagonal rule from the start crosspoint over the rows spanned. The targets are every row spanned,
// or only the picked ones when more than one destination is picked. Sources are the picked sources wrapping from the
// start, or the start's source when fewer than two are picked. full is the axes before a Filter, so picked sources
// that a Filter hides still count, as in a Diagonal.
export function linePairs(p, a, b, axes, full = axes) {
  const dims = { rows: axes.dsts.length, cols: axes.srcs.length };
  if (!p.srcs.length && !p.dsts.length) return snapLine(a, b, dims).cells.map(({ i, j }) => ({ src: axes.srcs[j], dst: axes.dsts[i] }));
  const cells = lineCells(a, b, dims);
  if (!cells.length) return [];
  const start = { src: axes.srcs[cells[0].j], dst: axes.dsts[cells[0].i] };
  const span = cells.map(({ i }) => axes.dsts[i]);
  const targets = p.dsts.length > 1 ? span.filter((d) => p.dsts.includes(d)) : span;
  const srcs = p.srcs.length > 1 ? fromHere(p.srcs, full.srcs, start.src) : [start.src];
  return targets.map((dst, k) => ({ src: srcs[k % srcs.length], dst }));
}

// The line the preview draws: snapped with nothing picked, else as drawn. A snapped line with no targets yet shows as
// drawn, so the drag still shows.
export function lineShown(p, a, b, axes) {
  if (p.srcs.length || p.dsts.length) return { a, b };
  const s = snapLine(a, b, { rows: axes.dsts.length, cols: axes.srcs.length });
  return s.cells.length ? { a: s.a, b: s.b } : { a, b };
}

// A release: pairs through stageable (protected rows skipped), then stageRoutes. Returns stageRoutes' { st, send,
// confirm } plus skipped (the protected pairs) and over (past the cap, so nothing is staged). As a press: Take mode
// only stages, and Instant mode sends at once. A line left with one route never unstages it.
export function lineStage(st, pairs, env, levels) {
  if (!pairs.length) return { st, send: null, confirm: false, skipped: [], over: false };
  const { pairs: kept, err } = stageable(pairs, env);
  const keep = new Set(kept);
  const skipped = pairs.filter((x) => !keep.has(x));
  if (err) return { st: { ...st, error: err }, send: null, confirm: false, skipped, over: false };
  let base = st;
  if (kept.length === 1) {
    const [{ src, dst }] = kept;
    base = { ...st, staged: st.staged.filter((x) => !(x.src === src && x.dst === dst)) };
  }
  const r = stageRoutes(base, kept, levels);
  if (r.st.error) return { ...r, st: { ...st, error: r.st.error }, skipped, over: r.st.error.code === 'cap' };
  return { ...r, skipped, over: false };
}
