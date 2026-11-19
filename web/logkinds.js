// /log kind chips. Pure, no imports. "All" is not stored (every kind selected), so saved filters stay a list of ids.

export const KINDS = [
  // Router salvo messages (120/248 stage, 121 go/clear) route too, so they show with connects. A Protect set or clear
  // (140/142) changes what a connect can do, so it shows there too.
  { id: 'connect', label: 'Connects', kinds: ['connect', 'salvoStage', 'salvoStageExt', 'salvoGo', 'protectSet', 'protectClear'] },
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
  { id: 'settings', label: 'Settings saves', kinds: ['settings'] },
];
export const KIND_IDS = KINDS.map((g) => g.id);

// Words for a row's why. A why with no entry shows as written. Keep in step with logWhyLabel in
// internal/app/logrules.go (TestLogRulesMatchPage), which writes them in the downloads.
export const WHY_LABEL = {
  take: 'Take',
  undo: 'Undo',
  retry: 'Retry',
  check: 'Check before take',
  confirm: 'Confirm take',
  resync: 'Resync',
  names: 'Re-read names (timer)',
  keepalive: 'Link check',
  connect: 'WebSocket connected',
  disconnect: 'WebSocket dropped',
  rereadNames: 'Re-read names',
  routerNames: 'Router names',
  clearHistory: 'Clear history',
  approvalRequest: 'Request approval',
  stepIn: 'Step in',
  stepOut: 'Step out',
  config: 'Settings saved',
  filterSets: 'Filter sets saved',
  presets: 'Presets saved',
  salvos: 'Presets saved', // salvos: rows logged before the rename
  salvo: 'Router salvo',
  protect: 'Protect',
  protectClear: 'Clear Protect',
  protectUndo: 'Undo of a Protect',
  protectRollback: 'Protect rolled back',
  protectClearAll: "Clear Sode's Protects",
  userSignIn: 'Sign in',
  userLock: 'User locked',
  userSignOut: 'User signed out',
  adminUnlock: 'PIN sign-in (old rows)',
  users: 'Users saved',
  streamConnect: 'Stream connected',
  streamDisconnect: 'Stream dropped',
  panelLost: 'Panel instance lost',
  panelBack: 'Panel instance back',
  panelAlarm: 'Panel alarm',
  panelSetup: 'Panel instance set up',
  assign: 'Instance assigned',
  rename: 'Instance renamed',
  reload: 'Panel page reloaded',
  publicBack: 'Public panel page back',
  save: 'Settings saved',
  import: 'Import',
  api: 'Settings saved through the API',
  setup: 'Setup finished',
  setupClaim: 'Setup take control',
  setupRouter: 'Setup router address',
  setupFinish: 'Setup finished',
  inboxSend: 'Copy sent',
  inboxAccept: 'Copy accepted',
  inboxDecline: 'Copy declined',
  inboxWithdraw: 'Copy withdrawn',
  inboxExpired: 'Copy expired',
  download: 'Communication Log downloaded',
};
const KIND_LABEL = {
  connect: 'Connect',
  interrogate: 'Interrogate',
  keepalive: 'Keepalive',
  tallyDump: 'Tally dump',
  protectDump: 'Protect dump',
  protectSet: 'Protect set',
  protectClear: 'Protect clear',
  srcNames: 'Source names',
  dstNames: 'Destination names',
  other: 'Other',
  salvoStage: 'Router salvo stage',
  salvoStageExt: 'Router salvo extended stage',
  salvoGo: 'Router salvo go',
  browser: 'Browser',
  command: 'Command',
  api: 'API',
  link: 'Router link',
  server: 'Sode',
  settings: 'Settings save',
};

// A row's message words. The stored kind `salvoGo` is both the Go and the clear of a router salvo (121). The
// message's second byte tells them apart, so rows in old log files get the right words too.
export function kindLabel(e) {
  if (e.kind === 'salvoGo' && typeof e.hex === 'string' && e.hex.split(' ')[1] === '01') return 'Router salvo clear';
  return KIND_LABEL[e.kind] || e.kind;
}

export const EVENT_KINDS = new Set(['browser', 'command', 'api', 'link', 'server', 'settings']);
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

// A filter saved before `known` existed knew every kind but Settings saves.
export function addNewKinds(kinds, known) {
  const met = Array.isArray(known) ? known : KIND_IDS.filter((id) => id !== 'settings');
  return [...kinds, ...KIND_IDS.filter((id) => !met.includes(id) && !kinds.includes(id))];
}

export const allKinds = (kinds) => KIND_IDS.every((id) => kinds.includes(id));

export function pressKind(kinds, id) {
  if (allKinds(kinds)) return [id];
  const next = KIND_IDS.filter((k) => (k === id ? !kinds.includes(k) : kinds.includes(k)));
  return next.length ? next : KIND_IDS.slice();
}
