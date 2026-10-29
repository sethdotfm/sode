// The /log message-kind chips (#132): which kinds are shown, and what
// pressing All or a kind does. Pure, no imports. "All" is not stored: it is
// every kind selected, so saved filters keep their shape (a list of ids).

// Filter groups, in the order shown.
export const KINDS = [
  { id: 'connect', label: 'Connects', kinds: ['connect'] },
  { id: 'interrogate', label: 'Interrogates', kinds: ['interrogate'] },
  { id: 'keepalive', label: 'Keepalives', kinds: ['keepalive'] },
  { id: 'dump', label: 'Dumps', kinds: ['tallyDump', 'protectDump'] },
  { id: 'names', label: 'Name requests', kinds: ['srcNames', 'dstNames'] },
  { id: 'other', label: 'Other', kinds: ['other'] },
];
export const KIND_IDS = KINDS.map((g) => g.id);

// Reads a saved kinds list. Anything but a list gives the default; a list
// with no known kind means All, so the view is never empty by accident.
export function normalizeKinds(saved, dflt) {
  if (!Array.isArray(saved)) return dflt.slice();
  const known = KIND_IDS.filter((id) => saved.includes(id));
  return known.length ? known : KIND_IDS.slice();
}

export const allKinds = (kinds) => KIND_IDS.every((id) => kinds.includes(id));

// Pressing a kind: with All on, only that kind; otherwise toggle it, and
// turning off the last one goes back to All.
export function pressKind(kinds, id) {
  if (allKinds(kinds)) return [id];
  const next = KIND_IDS.filter((k) => (k === id ? !kinds.includes(k) : kinds.includes(k)));
  return next.length ? next : KIND_IDS.slice();
}
