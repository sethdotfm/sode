// Which rows of a long list are drawn. A list opened by focus holds every
// source or destination (thousands on a big router); only a window of it is in
// the DOM, and the window grows as the highlight or the scroll gets near an
// edge. Pure and import-free, so tests can load it alone.

export const WINDOW = 150; // rows drawn at first
export const STEP = 75;    // rows added when an edge is reached

// The window of `size` rows around row `at` in a list of n: { lo, hi } with
// rows lo..hi-1 drawn. A list no longer than size is drawn whole.
export function windowAround(n, at, size = WINDOW) {
  if (n <= size) return { lo: 0, hi: n };
  const lo = Math.max(0, Math.min(at - Math.floor(size / 2), n - size));
  return { lo, hi: lo + size };
}

// win grown by step rows at its end (below) or start (above); the same
// object when there is nothing more that way.
export function growEdge(win, n, below, step = STEP) {
  if (below) return win.hi >= n ? win : { lo: win.lo, hi: Math.min(n, win.hi + step) };
  return win.lo <= 0 ? win : { lo: Math.max(0, win.lo - step), hi: win.hi };
}
