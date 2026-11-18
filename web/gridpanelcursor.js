// The Grid panel cursor. Pure. A cursor is { i, j }: display order indices of a destination row and a source column
// (gridAxes). -1 is the bar on that axis: i -1 is the Source bar, j -1 the Destination bar. (-1, -1) is the corner.
// gridpanelkeys.js maps keys to these moves.

export const CORNER = Object.freeze({ i: -1, j: -1 });

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

// A screen move from a crosspoint stops at the first one; from a bar it goes onto the crosspoints.
const page = (at, by, n) => clamp(at + by, at >= 0 ? 0 : -1, n - 1);

// dims: { rows, cols, page: { rows, cols } }, page being the whole items one screen shows.
const STEPS = {
  up: (c) => ({ i: c.i - 1 }),
  down: (c) => ({ i: c.i + 1 }),
  left: (c) => ({ j: c.j - 1 }),
  right: (c) => ({ j: c.j + 1 }),
  pageUp: (c, d) => ({ i: page(c.i, -Math.max(1, d.page.rows), d.rows) }),
  pageDown: (c, d) => ({ i: page(c.i, Math.max(1, d.page.rows), d.rows) }),
  pageLeft: (c, d) => ({ j: page(c.j, -Math.max(1, d.page.cols), d.cols) }),
  pageRight: (c, d) => ({ j: page(c.j, Math.max(1, d.page.cols), d.cols) }),
  corner: () => CORNER,
  rowStart: () => ({ j: 0 }),
  rowEnd: (c, d) => ({ j: d.cols - 1 }),
  colEnd: (c, d) => ({ i: d.rows - 1 }),
};

export const MOVES = new Set(Object.keys(STEPS));

export function moveCursor(c, move, dims) {
  const next = { ...c, ...STEPS[move](c, dims) };
  return { i: clamp(next.i, -1, dims.rows - 1), j: clamp(next.j, -1, dims.cols - 1) };
}

// Router numbers under the cursor; undefined on a bar.
export const cursorAt = (c, axes) => ({ src: c.j >= 0 ? axes.srcs[c.j] : undefined, dst: c.i >= 0 ? axes.dsts[c.i] : undefined });

// The crosspoint a press at the cursor stages, or null on a bar.
export const pointOf = (c, axes) => (c.i >= 0 && c.j >= 0 ? cursorAt(c, axes) : null);

// On new axes (a live update, Rights, a Filter) the cursor stays on its items. A gone item keeps the index.
const follow = (k, was, now) => {
  if (k < 0) return -1;
  const at = now.indexOf(was[k]);
  return at >= 0 ? at : Math.min(k, now.length - 1);
};
export const followCursor = (c, was, now) => ({ i: follow(c.i, was.dsts, now.dsts), j: follow(c.j, was.srcs, now.srcs) });

// 'srcs' on the Source bar, 'dsts' on the Destination bar, 'corner', or null on a crosspoint.
export const barOf = (c) => (c.i < 0 ? (c.j < 0 ? 'corner' : 'srcs') : c.j < 0 ? 'dsts' : null);

const ACROSS = new Set(['left', 'right', 'pageLeft', 'pageRight']);

// The names a Shift move picks: the bar's, or on a crosspoint the sources for a move across, else the destinations.
export const rangeKind = (c, move) => barOf(c) ?? (ACROSS.has(move) ? 'srcs' : 'dsts');

// A Shift move (gridpanelkeys.js range actions) stays on that bar's items, or on the crosspoints.
export function rangeCursor(c, move, dims) {
  const n = moveCursor(c, move, dims);
  const bar = barOf(c);
  if (bar === 'srcs') return { i: -1, j: Math.max(0, n.j) };
  if (bar === 'dsts') return { i: Math.max(0, n.i), j: -1 };
  return { i: Math.max(0, n.i), j: Math.max(0, n.j) };
}
