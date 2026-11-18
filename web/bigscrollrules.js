// Big scroll bar geometry (bigscroll.js). Pure, import-free. All lengths in px along the bar's axis.

// The thumb for a view of `view` px over `content` px on a track of `track` px, never under `min`.
// travel is how far the thumb moves from one end to the other.
export function thumbFor(view, content, track, min) {
  if (content <= view || track <= 0) return { len: track, travel: 0 };
  const len = Math.min(track, Math.max(min, Math.round((track * view) / content)));
  return { len, travel: track - len };
}

// The thumb's offset from the track start for a scroll position.
export function thumbAt(pos, scrollMax, travel) {
  if (scrollMax <= 0 || travel <= 0) return 0;
  return Math.round((Math.min(scrollMax, Math.max(0, pos)) / scrollMax) * travel);
}

// The scroll position after the thumb is dragged `moved` px from where it was at `from`.
// It maps through travel, not track, so a long drag reaches both ends with a minimum thumb.
export function dragTo(from, moved, scrollMax, travel) {
  if (travel <= 0) return from;
  return Math.min(scrollMax, Math.max(0, from + (moved * scrollMax) / travel));
}

// -1 or 1 to page toward a press at `at` on the track, 0 on the thumb.
export function pageDir(at, thumbStart, thumbLen) {
  if (at < thumbStart) return -1;
  if (at >= thumbStart + thumbLen) return 1;
  return 0;
}

// A wheel event's movement in px along the bar. A sideways bar takes the vertical wheel too, so a
// mouse with one wheel can move it. deltaMode 1 is lines, 2 is pages.
export function wheelPx(dx, dy, mode, axis, page) {
  const d = axis === 'x' ? dx || dy : dy;
  if (mode === 1) return d * 40;
  if (mode === 2) return d * page;
  return d;
}
