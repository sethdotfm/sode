// The /log message-kind chips (#132): which kinds are shown, and what
// pressing All or a kind does. Pure, no imports. "All" is not stored: it is
// every kind selected, so saved filters keep their shape (a list of ids).

// Reads a saved kinds list. Anything but a list gives the default; a list
// with no known kind means All, so the view is never empty by accident.
export function normalizeKinds(saved, ids, dflt) {
  if (!Array.isArray(saved)) return dflt.slice();
  const known = ids.filter((id) => saved.includes(id));
  return known.length ? known : ids.slice();
}

export const allKinds = (kinds, ids) => ids.every((id) => kinds.includes(id));

// Pressing All selects every kind.
export const pressAll = (ids) => ids.slice();

// Pressing a kind: with All on, only that kind; otherwise toggle it, and
// turning off the last one goes back to All.
export function pressKind(kinds, ids, id) {
  if (allKinds(kinds, ids)) return [id];
  const next = ids.filter((k) => (k === id ? !kinds.includes(k) : kinds.includes(k)));
  return next.length ? next : ids.slice();
}
