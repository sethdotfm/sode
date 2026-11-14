// /log kind chips. Pure, no imports. "All" is not stored (every kind selected), so saved filters stay a list of ids.

export const KINDS = [
  // Native salvo messages (120/248 stage, 121 go/clear) route too, so they show with connects.
  { id: 'connect', label: 'Connects', kinds: ['connect', 'salvoStage', 'salvoStageExt', 'salvoGo'] },
  { id: 'interrogate', label: 'Interrogates', kinds: ['interrogate'] },
  { id: 'keepalive', label: 'Keepalives', kinds: ['keepalive'] },
  { id: 'dump', label: 'Dumps', kinds: ['tallyDump', 'protectDump'] },
  { id: 'names', label: 'Name requests', kinds: ['srcNames', 'dstNames'] },
  { id: 'other', label: 'Other', kinds: ['other'] },
  // Events: not router messages, so no hex or command byte.
  { id: 'browser', label: 'Browsers', kinds: ['browser'] },
  { id: 'command', label: 'Commands', kinds: ['command'] },
  { id: 'api', label: 'API', kinds: ['api'] },
  { id: 'link', label: 'Router link', kinds: ['link', 'server'] },
];
export const KIND_IDS = KINDS.map((g) => g.id);

// Words for a row's why. A why with no entry shows as written.
export const WHY_LABEL = {
  take: 'Take',
  undo: 'Undo',
  retry: 'Retry',
  check: 'Check before take',
  confirm: 'Confirm take',
  resync: 'Resync',
  names: 'Name re-read',
  keepalive: 'Link check',
  connect: 'WebSocket connected',
  disconnect: 'WebSocket dropped',
  refreshNames: 'Re-read names',
  clearHistory: 'Clear history',
  config: 'Settings saved',
  filterSets: 'Filter sets saved',
  presets: 'Presets saved',
  salvos: 'Presets saved', // salvos: rows logged before the rename
  unlock: 'Advanced Settings PIN',
  lock: 'Settings locked',
  salvo: 'Native salvo',
  userUnlock: 'User PIN',
  userLock: 'User locked',
  userSignOut: 'User signed out',
  adminUnlock: 'Admin password (old rows)',
  users: 'Users saved',
  streamConnect: 'Stream connected',
  streamDisconnect: 'Stream dropped',
  panelLost: 'Panel instance lost',
  panelBack: 'Panel instance back',
  panelAlarm: 'Panel alarm',
  panelSetup: 'Panel instance set up',
  assign: 'Instance assigned',
  rename: 'Instance renamed',
};
export const EVENT_KINDS = new Set(['browser', 'command', 'api', 'link', 'server']);
const EVENT_PROBLEM = { command: ['refused', 'locked', 'error', 'fallback'], api: ['refused', 'error'], link: ['down'] };
// Only server rows count an err as a problem: a browser disconnect also carries one for an abnormal close.
export const eventProblem = (e) => (e.kind === 'server' ? !!e.err : !!EVENT_PROBLEM[e.kind]?.includes(e.result));

// Saved lists of exactly these (pre-events) meant All.
const LEGACY_IDS = ['connect', 'interrogate', 'keepalive', 'dump', 'names', 'other'];

// Anything but a list gives the default; no known kind means All, so the view is never empty.
export function normalizeKinds(saved, dflt) {
  if (!Array.isArray(saved)) return dflt.slice();
  const known = KIND_IDS.filter((id) => saved.includes(id));
  if (LEGACY_IDS.every((id) => known.includes(id)) && !known.some((id) => !LEGACY_IDS.includes(id))) return KIND_IDS.slice();
  return known.length ? known : KIND_IDS.slice();
}

export const allKinds = (kinds) => KIND_IDS.every((id) => kinds.includes(id));

export function pressKind(kinds, id) {
  if (allKinds(kinds)) return [id];
  const next = KIND_IDS.filter((k) => (k === id ? !kinds.includes(k) : kinds.includes(k)));
  return next.length ? next : KIND_IDS.slice();
}
