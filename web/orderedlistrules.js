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
