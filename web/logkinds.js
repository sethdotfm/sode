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
  // Events (#139): not messages to the router, so no hex or command byte.
  { id: 'browser', label: 'Browsers', kinds: ['browser'] },
  { id: 'command', label: 'Commands', kinds: ['command'] },
  { id: 'link', label: 'Router link', kinds: ['link', 'server'] },
];
export const KIND_IDS = KINDS.map((g) => g.id);
// The kinds that are events.
export const EVENT_KINDS = new Set(['browser', 'command', 'link', 'server']);
// The groups before events existed: a saved list of all of these was All.
const LEGACY_IDS = ['connect', 'interrogate', 'keepalive', 'dump', 'names', 'other'];

// Reads a saved kinds list. Anything but a list gives the default; a list
// with no known kind means All, so the view is never empty by accident. A
// list of every group from before events means All, which now includes them.
export function normalizeKinds(saved, dflt) {
  if (!Array.isArray(saved)) return dflt.slice();
  const known = KIND_IDS.filter((id) => saved.includes(id));
  if (LEGACY_IDS.every((id) => known.includes(id)) && !known.some((id) => !LEGACY_IDS.includes(id))) return KIND_IDS.slice();
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
