// The phone's position strips on a Grid panel (gridpanelpos.js). Pure. s is one axis of the grid's shown().
const THUMB_MIN = 32; // px: the shortest thumb, so it stays in sight on a 2500-item axis

// The thumb's length and place along a track of len px.
export function thumbOf(s, len) {
  const view = s.max > 0 ? len * (len / (len + s.max)) : len;
  const size = Math.min(len, Math.max(THUMB_MIN, view));
  const at = s.max > 0 ? ((len - size) * Math.min(s.offset, s.max)) / s.max : 0;
  return { size, at };
}

export const rangeText = (s) => (s.n ? `${s.first}–${s.last}/${s.n}` : '');
