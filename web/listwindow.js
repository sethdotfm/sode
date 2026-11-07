// Window of rows drawn from a long list (thousands on a big router), growing near an edge. Pure, import-free.

export const WINDOW = 150; // rows drawn at first
export const STEP = 75; // rows added when an edge is reached

// Rows lo..hi-1 drawn; a short list is drawn whole.
export function windowAround(n, at, size = WINDOW) {
  if (n <= size) return { lo: 0, hi: n };
  const lo = Math.max(0, Math.min(at - Math.floor(size / 2), n - size));
  return { lo, hi: lo + size };
}

// Same object when nothing more that way.
export function growEdge(win, n, below, step = STEP) {
  if (below) return win.hi >= n ? win : { lo: win.lo, hi: Math.min(n, win.hi + step) };
  return win.lo <= 0 ? win : { lo: Math.max(0, win.lo - step), hi: win.hi };
}
