// Settings search: pure, no imports (Node-tested), own scorer because core.js `search` needs the DOM.
// RULE: a new settings section needs an entry in DICT under its slug (see `slug`); settingsindex.test.mjs fails otherwise.
// Settings are keyed by their control's id; DICT ids that don't exist (yet) are ignored.

export const slug = (s) =>
  String(s)
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');

const both = (k, terms) => ({ ['cfg-' + k]: terms, ['gcfg-' + k]: terms });

export const DICT = {
  // ---- sections
  'this-browser': [
    'this device',
    'this browser',
    'local',
    'personal',
    'preferences',
    'prefs',
    'appearance',
    'display',
    'device',
    'my settings',
    'theme',
    'dark',
    'light',
    'colour',
    'color',
    'clock',
    'time',
    '24h',
    'layout',
    'my default',
    'set as my default',
    'clear my default',
    'profile',
    'follow',
  ],
  'my-user': [
    'me',
    'account',
    'logged in',
    'login',
    'log in',
    'logout',
    'log out',
    'sign in',
    'sign out',
    'lock',
    'lock the screen',
    'profile',
    'my profile',
    'change pin',
    'import',
    'export',
  ],
  'global-defaults': ['global', 'everyone', 'default', 'defaults', 'all devices', 'all browsers', 'shared', 'shared settings', 'appearance', 'preferences'],
  'router-source-destination-names': [
    'names',
    'source names',
    'destination names',
    'labels',
    'refresh',
    'poll',
    'reread',
    'router names',
    'name length',
    'characters',
    'truncate',
  ],
  'level-colors': ['level colours', 'colour', 'color', 'colors', 'palette', 'hex', 'swatch', 'level', 'levels', 'rgb'],
  takes: ['take', 'patch', 'route', 'confirm', 'confirm dialog', 'lockout', 'letter', 'enter', 'default levels', 'resync', 'reread', 'delay', 'safety'],
  branding: ['brand', 'logo', 'name', 'title', 'border', 'colour', 'color', 'icon', 'identify', 'favicon', 'tab'],
  'router-matrix': [
    'router',
    'ip',
    'address',
    'host',
    'hostname',
    'port',
    'connection',
    'connect',
    'tcp',
    'ultrix',
    'swp08',
    'sw-p-08',
    'status',
    'server',
    'level',
    'levels',
    'offset',
    'number',
    'numbering',
    'wire',
    'matrix',
    'zero',
    'start at 0',
    'start at 1',
    'count',
    'router levels',
    'video',
    'audio',
    'va',
  ],
  'server-backup': ['server configuration', 'config file', 'config.yaml', 'backup', 'restore', 'export', 'import', 'secrets', 'pins', 'download', 'upload'],
  access: [
    'password',
    'pin',
    'lock',
    'locked',
    'unlock',
    'security',
    'admin',
    'passcode',
    'protect',
    'tries',
    'wrong',
    'lockout',
    'timeout',
    'idle',
    'auto lock',
    'auto-lock',
    'permissions',
    'login',
    'settings pin',
  ],
  proxy: [
    'audit',
    'sode networking',
    'networking',
    'network',
    'proxy',
    'proxies',
    'trusted',
    'x-forwarded-for',
    'forwarded',
    'ip address',
    'client ip',
    'reverse proxy',
    'log',
    'logging',
    'who',
  ],
  history: ['history', 'recent', 'external', 'undo', 'clear', 'delete', 'erase', 'filter', 'filters', 'filter sets', 'set', 'search', 'hidden', 'hide'],
  users: [
    'user',
    'users',
    'people',
    'operators',
    'names',
    'pin',
    'password',
    'passwords',
    'lock',
    'lock screen',
    'auto lock',
    'auto-lock',
    'timeout',
    'idle',
    'inactivity',
    'logout',
    'access',
    'who',
    'accounts',
    'permissions',
  ],
  'export-import': [
    'export',
    'import',
    'backup',
    'restore',
    'yaml',
    'file',
    'download',
    'upload',
    'share',
    'share link',
    'move',
    'copy',
    'transfer',
    'another site',
    'studio',
    'settings file',
    'save',
    'load',
    'presets',
    'filter sets',
  ],
  'transfer-open': ['export', 'import', 'backup', 'restore', 'yaml', 'file', 'download', 'upload', 'share link', 'review', 'transfer', 'copy to another site'],
  // Sections other branches may add; ignored until they exist.
  salvos: ['preset', 'presets', 'salvo', 'salvos', 'scene', 'batch', 'multi take', 'macro', 'group'],
  'fire-mode': ['fire', 'native', 'sync', 'salvo', 'router salvo', 'simultaneous', 'together'],
  grids: ['grid', 'grids', 'panel', 'panels', 'tablet', 'buttons'],

  // ---- This browser / Defaults
  ...both('theme', ['dark', 'light', 'colour', 'color', 'mode', 'night', 'appearance', 'theme', 'dark mode', 'light mode']),
  ...both('layout', ['desktop', 'phone', 'mobile', 'tablet', 'touch', 'size', 'narrow', 'responsive']),
  ...both('clock', ['time', '24h', '24 hour', '12 hour', '12h', 'am pm', 'hours', 'format', 'clock', 'timestamp']),
  ...both('flip', ['order', 'swap', 'reverse', 'destination first', 'source first', 'dst', 'src', 'boxes', 'flip']),
  ...both('scrolltop', ['scroll', 'top', 'jump', 'focus', 'page', 'scroll to top']),
  ...both('search-filters', ['quick search', 'search', 'filter', 'filters', 'hidden', 'respect', 'history search']),
  ...both('set-stack', ['stack', 'stacked', 'filter sets', 'sets', 'and', 'or', 'combine', 'all', 'any', 'filter']),
  ...both('next-mode', ['next', 'increment', 'plus one', 'number', 'fuzzy', 'chip', 'ctrl shift s', 'following']),

  // ---- My defaults (per-row buttons shown while signed in)
  ...Object.fromEntries(
    ['theme', 'layout', 'clock', 'flip', 'scrollTop', 'searchFilters', 'setStack', 'nextMode'].flatMap((n) => [
      ['my-default-' + n, ['my default', 'save as my default', 'set as my default', 'profile', 'personal', 'follow me', 'account']],
      ['my-default-clear-' + n, ['my default', 'clear my default', 'remove', 'profile', 'personal', 'account']],
    ]),
  ),

  // ---- Me (inside This browser; only shown while signed in as a user)
  'account-profile': ['profile', 'my profile', 'my settings', 'user settings', 'presets', 'filters', 'filter sets', 'account'],
  'account-transfer': ['export', 'import', 'backup', 'restore', 'share', 'transfer', 'my settings', 'presets', 'filter sets', 'account'],
  'lock-pages': ['lock', 'lock main screen', 'lock communication log', 'lock log', 'pin', 'users', 'protect', 'screen lock'],
  'cfg-auto-lock': ['auto lock', 'auto-lock', 'idle', 'inactivity', 'timeout', 'minutes', 'lock after', 'hand lock'],
  'config-file': [
    'server configuration',
    'config file',
    'config.yaml',
    'yaml',
    'backup',
    'restore',
    'export',
    'import',
    'download',
    'upload',
    'migrate',
    'copy settings',
  ],
  api: ['api', 'api enabled', 'enable api', 'guest api', 'rate limit', 'take cap', 'log api reads', 'api reads', 'panel', 'rest', 'keys', 'clients'],
  'api-save': ['save api settings', 'api'],
  'config-export': ['export', 'download', 'save', 'backup', 'config file', 'config.yaml', 'yaml', 'server configuration', 'settings file'],
  'config-import': ['import', 'upload', 'restore', 'load', 'replace', 'config file', 'config.yaml', 'yaml', 'server configuration', 'settings file'],
  'config-pins': ['include pins', 'private backup', 'secret', 'hashes', 'pin', 'password', 'backup', 'export'],
  'users-manage': [
    'manage users',
    'add user',
    'remove user',
    'edit users',
    'users',
    'pin',
    'trusted',
    'never log out',
    'persistent',
    'sign out everywhere',
    'lock main screen',
    'lock log',
    'auto lock',
    'idle',
    'longest pin',
    'sign-in mode',
    'username',
    'username and pin',
    'pin only',
    'shared pin',
    'api client',
    'api clients',
    'api key',
    'api keys',
    'rights',
    'edit user rights',
    'visibility',
    'hidden',
    'no auto-lock',
    'guest',
    'guest scopes',
    'scopes',
    'revoke key',
  ],
  account: ['account', 'logged in', 'login', 'logout', 'log out', 'sign out', 'signout', 'user', 'me', 'my', 'session', 'password', 'pin'],
  'account-keys': ['api', 'api key', 'api keys', 'key', 'keys', 'my keys', 'panel', 'token', 'qr', 'qr code', 'copy key', 'revoke', 'expiry', 'account'],
  'account-pin': ['pin', 'password', 'passcode', 'change', 'my pin', 'account', 'reset'],
  lock: ['lock', 'lock screen', 'screen lock', 'keyboard', 'mashing', 'accidental', 'cat', 'clean', 'unlock', 'ctrl+l', 'letter', 'protect'],
  'lock-btn': ['lock', 'lock screen', 'keyboard', 'mashing', 'accidental', 'unlock', 'ctrl+l'],
  'account-login': ['log in', 'login', 'sign in', 'signin', 'user', 'pin', 'account', 'session', 'tab', 'name'],
  'account-logout': ['logout', 'log out', 'sign out', 'signout', 'session', 'account', 'lock', 'leave'],

  // ---- Names
  'names-refresh': ['refresh', 'reload', 'update', 're-read', 're read', 'reread', 'poll', 'names', 'now', 'fetch'],
  'cfg-namerefresh': ['poll', 'interval', 'seconds', 'refresh', 're-read', 're read', 'how often', 'timer', 'off', 'names'],
  'cfg-namelen': ['length', 'characters', 'chars', 'width', 'short', 'long', '4', '8', '12', '16', 'truncate'],
  'cfg-salvogroup': ['salvo', 'router salvo', 'native', 'group', 'group number', 'crosspoint list', 'fire mode', 'ultrix', '127'],
  'cfg-firemode': ['fire mode', 'native', 'router salvo', 'preset', 'sequential', 'multi destination', 'multiple destinations', 'crosspoint list'],
  'cfg-namereq': ['standard', 'extended', 'request', 'protocol', 'names', 'sw-p-08', 'swp08', '12 char'],

  // ---- Level colors
  'cfg-colors-reset': ['reset', 'default colours', 'default colors', 'restore'],

  // ---- Takes
  'cfg-default': ['default levels', 'levels', 'all levels', 'which levels', 'preselect', 'v a', 'video audio'],
  'cfg-dismiss': ['banner', 'result', 'success', 'dismiss', 'close', 'auto', 'seconds', 'timeout', 'progress', 'notification'],
  'cfg-lockout': ['confirm', 'lockout', 'letter', 'delay', 'ms', 'milliseconds', 'confirm dialog', 'wait', 'safety', 'accidental'],
  'cfg-resync': ['resync', 'reread', 'poll', 'routes', 'tally', 'interval', 'seconds', 'refresh', 'dump'],
  'cfg-enter': ['enter', 'return', 'confirm', 'keyboard', 'key', 'submit', 'confirm dialog'],

  // ---- Branding
  'cfg-brandtext': ['brand', 'logo', 'name', 'title', 'text', 'page title', 'tab title', 'rename'],
  'cfg-brandborder': ['border', 'colour', 'color', 'frame', 'outline', 'identify', 'brand', 'multiple servers', 'edge'],

  // ---- Router
  'cfg-router': ['router', 'ip', 'address', 'host', 'hostname', 'port', 'tcp', 'connect', 'ultrix', 'host:port'],
  'cfg-routername': ['router name', 'name', 'label', 'title', 'call it'],
  'cfg-routershow': ['status', 'show', 'header', 'display', 'address', 'name', 'connected'],

  // ---- Matrix
  'cfg-matrix': ['matrix', 'number', 'id', 'router matrix', 'index', 'zero'],
  'cfg-levels': ['levels', 'count', 'number of levels', 'how many', 'level count', '17'],
  'cfg-offset': ['offset', 'wire', 'router level', 'level offset', 'shift', 'level', 'zero based', 'one based'],
  'cfg-lvlnum': ['level numbering', 'level', 'numbering', 'number', 'v', 'va', 'video', 'zero', '0', '1', 'start'],
  'cfg-numoff': ['number', 'numbers', 'offset', 'start at 0', 'start at 1', 'zero', 'one', 'sources', 'destinations', 'numbering', 'index'],

  // ---- Access
  'cfg-pin': ['pin', 'password', 'passcode', 'lock', 'secret', 'admin', 'protect settings', 'security', 'change pin'],
  'cfg-pin2': ['pin', 'password', 'confirm', 'repeat', 'retype', 'again'],
  'cfg-pin-clear': ['remove pin', 'delete pin', 'no pin', 'unlock', 'disable', 'password off', 'clear'],
  'cfg-pin-tries': ['wrong pin', 'tries', 'attempts', 'lock out', 'lockout', 'brute force', 'incorrect', 'guesses', 'reset pin'],
  'cfg-kill-enabled': ['kill', 'emergency', 'stop', 'shut down', 'shutdown', 'panic', 'off switch', 'restart policy', 'docker', 'crash'],

  // ---- Audit
  'cfg-proxies': [
    'proxy',
    'proxies',
    'trusted',
    'network',
    'x-forwarded-for',
    'forwarded',
    'reverse proxy',
    'nginx',
    'ip',
    'client ip',
    'cidr',
    'tls',
    'https',
  ],
  'cfg-require-https': ['require https', 'https', 'tls', 'ssl', 'secure', 'hsts', 'x-forwarded-proto', 'insecure', 'refuse', 'proxy', 'reset https'],

  // ---- History
  'cfg-hidden': ['hidden', 'hide', 'destinations', 'dst', 'filter', 'global filter', 'panel', 'external', 'external follow', 'mv', 'multiviewer', 'recent'],
  'clear-history': ['clear', 'delete', 'erase', 'wipe', 'reset', 'undo', 'history', 'recent', 'remove all'],
  'open-log': ['log', 'communication log', 'audit', 'uptime', 'messages', 'sent', 'wire', 'router log', 'events', 'tx', 'ack', 'diagnostics', 'debug', 'graph'],
};

// ---- matching

const words = (s) =>
  String(s)
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
    .split(' ')
    .filter(Boolean);

function lev(a, b, max) {
  if (Math.abs(a.length - b.length) > max) return max + 1;
  let prev = Array.from({ length: b.length + 1 }, (_, i) => i);
  for (let i = 1; i <= a.length; i++) {
    const cur = [i];
    let low = i;
    for (let j = 1; j <= b.length; j++) {
      cur[j] = Math.min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1));
      low = Math.min(low, cur[j]);
    }
    if (low > max) return max + 1;
    prev = cur;
  }
  return prev[b.length];
}

function wordScore(t, w) {
  if (t === w) return 100;
  if (w.startsWith(t)) return 80;
  if (t.length >= 3 && w.includes(t)) return 55;
  if (t.length < 4) return 0;
  const max = t.length >= 7 ? 2 : 1;
  if (lev(t, w, max) <= max) return 40;
  if (w.length > t.length && lev(t, w.slice(0, t.length), 1) <= 1) return 35; // typo in a prefix
  return 0;
}

const top = (t, ws) => ws.reduce((m, w) => Math.max(m, wordScore(t, w)), 0);

export function buildIndex(sections, dict = DICT) {
  const out = [];
  for (const sec of sections) {
    const sw = words(sec.title);
    const mk = (kind, id, label) => ({
      kind,
      id,
      sectionId: sec.id,
      section: sec.title,
      label,
      lw: words(label),
      tw: (dict[id] || []).flatMap(words),
      sw,
    });
    out.push(mk('section', sec.id, sec.title));
    for (const s of sec.settings || []) out.push(mk('setting', s.id, s.label));
  }
  return out;
}

function scoreEntry(e, toks) {
  let sum = 0;
  for (const t of toks) {
    const s = Math.max(top(t, e.lw), top(t, e.tw) * 0.85, top(t, e.sw) * 0.4);
    if (!s) return 0;
    sum += s;
  }
  return sum;
}

export function find(index, query, n = 8) {
  const toks = words(query);
  const hits = [];
  if (toks.length) {
    index.forEach((e, i) => {
      const s = scoreEntry(e, toks);
      if (s > 0) hits.push([s, i, e]);
    });
    hits.sort((a, b) => b[0] - a[0] || a[1] - b[1]);
  }
  const out = hits.slice(0, n).map((h) => h[2]);
  out.total = hits.length;
  return out;
}
