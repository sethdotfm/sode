// The ordered list's rules: move, drop, remove. Pure; orderedlist.js draws the list. Out of range gives the same list
// back, so a caller can tell nothing changed.

export function moveItem(list, from, to) {
  if (from < 0 || from >= list.length || to < 0 || to >= list.length || from === to) return list;
  const out = [...list];
  const [x] = out.splice(from, 1);
  out.splice(to, 0, x);
  return out;
}

// The place the dragged row ends at when dropped before or after row over. With no side, the drop takes over's place:
// after it when dragged down, before it when dragged up.
export function dropTo(from, over, after = from < over) {
  const slot = over + (after ? 1 : 0);
  return from < slot ? slot - 1 : slot;
}

export const removeItem = (list, i) => (i < 0 || i >= list.length ? list : list.filter((_, j) => j !== i));

export const canMove = (i, n) => ({ up: i > 0, down: i < n - 1 });

// The scroll step for a drag at y over a scroller from top to bottom, in px a frame. It is negative in the band zone
// px under the top, positive in the band above the bottom, and 0 between them. It grows to max at the edge and stays
// at max past it. A short scroller's bands are a third of its height each.
export function edgeStep(y, top, bottom, zone, max) {
  const z = Math.min(zone, (bottom - top) / 3);
  if (z <= 0) return 0;
  const near = y < top + z ? (top + z - y) / z : y > bottom - z ? (y - (bottom - z)) / z : 0;
  if (!near) return 0;
  return Math.sign(y - (top + bottom) / 2) * Math.ceil(max * Math.min(1, near));
}
