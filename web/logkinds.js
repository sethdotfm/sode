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

// Words for a row's why. Every why the server writes has an entry (TestEveryLogReasonHasWords); one with no entry
// shows as written. The download files use them too (logfile.js).
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
  protectClear: 'Free Protect',
  protectUndo: 'Undo of a Protect',
  protectRollback: 'Protect rolled back',
  protectClearAll: "Free Sode's Protects",
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
  settingsSave: 'Settings saved',
  settingsReview: 'Settings review',
  bookmarks: 'Bookmarks saved',
  tieLines: 'Tie lines saved',
  profileSave: 'Profile saved',
  profileGet: 'Profile read',
  browserCopy: 'Copy this browser to My user',
  protectSet: 'Protect',
  tieTake: 'Take across routers',
  tiePlan: 'Tie line picker plan',
  tieUndo: 'Undo check across routers',
  swapList: 'Swap list read',
  swapMine: 'Swap in my items',
  pinRecheck: 'PIN recheck',
  userSetPin: 'PIN changed',
  userSignOutAll: 'Sign out everywhere',
  approvalAsk: 'Request approval',
  approvalAnswer: 'Approval answer',
  approvalCancel: 'Approval request cancelled',
  approvalCards: 'Approval requests read',
  setupRestoreKey: 'Setup restore key',
  inboxList: 'Inbox read',
  inboxReview: 'Copy reviewed',
  // Reads. A row shows only when Sode refuses one (a locked page or a Role).
  hi: 'Page connected',
  activity: 'Activity',
  history: 'History read',
  log: 'Log read',
  uptime: 'Uptime read',
  usersList: 'Users read',
  exportSession: 'Export access',
  sessionKey: 'Session key made',
  brokenCount: 'Broken count read',
  palettePanels: 'Panels for the Palette read',
  routerUses: 'Router uses read',
  routerTry: 'Router address test',
  oneOffTieLines: 'One-off tie lines read',
  // API rows (apiaudit.go and the /api/v1 routes).
  read: 'Log read through the API',
  refused: 'Refused',
  authFailed: 'API sign-in failed',
  endedKeyUsed: 'Ended API key used',
  keyCreate: 'API key made',
  keyCreateFor: 'API key made for a user',
  keyRevoke: 'API key revoked',
  keyRevokeAny: 'API key revoked by an Administrator',
  keyRevokeAll: 'All API keys revoked',
  keyEnded: 'API key ended',
  keyExpired: 'API key expired',
  keyListOwn: 'Own API keys read',
  keyListAll: 'All API keys read',
  myClients: 'My API clients read',
  myClientSave: 'My API client saved',
  myClientDelete: 'My API client deleted',
  myClientKey: 'My API client key made',
  myClientKeyRevoke: 'My API client key revoked',
  guestApi: 'Guest API access changed',
  keylessScopes: 'Keyless API access changed',
  logApiReads: 'Log reads through the API changed',
  apiLimits: 'API limits changed',
  requireHttps: 'Require HTTPS changed',
  panelCreate: 'Panel made',
  panelUpdate: 'Panel saved',
  panelCopy: 'Panel copied',
  panelDelete: 'Panel deleted',
  panelPush: 'Panel Push',
  panelRestore: 'Panel put back from a checkpoint',
  panelNetworks: 'Panel networks saved',
  panelKey: 'Panel key made',
  panelKeyRotate: 'Panel key replaced',
  panelKeyRevoke: 'Panel key revoked',
  panelSetupLink: 'Panel setup link made',
  panelSetCreate: 'Checkpoint set made',
  panelSetUpdate: 'Checkpoint set saved',
  panelSetDelete: 'Checkpoint set deleted',
  panelSetPush: 'Checkpoint set Push',
  templateCreate: 'Template made',
  templateDelete: 'Template deleted',
  iconAdd: 'Icon added to the Icon library',
  iconRename: 'Icon renamed in the Icon library',
  iconDelete: 'Icon deleted from the Icon library',
  iconRescan: 'Icon library Rescan',
  artDefaultSet: 'Default artwork set',
  artDefaultClear: 'Default artwork cleared',
  // Whys of rows written before the v2 renames (docs/persisted-names.md).
  refreshNames: 'Re-read names',
  userUnlock: 'Sign in',
  unlock: 'PIN sign-in (old rows)',
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
