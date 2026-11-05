// Settings search (pure, no imports; Node-tested). An index of every section
// and every setting in the Settings view, searched by label, by a dictionary
// of the words people actually type, and by the section's title.
//
// RULE: a new settings section needs an entry in DICT under its slug (see
// `slug`); test/settingsindex.test.mjs fails otherwise. Settings are keyed by
// the id of their control. DICT may name ids that don't exist (yet); those
// are ignored.
//
// This has its own scorer instead of core.js `search`: that one scores
// numbered router items and needs the DOM, and this must load in Node.

export const slug = (s) => String(s).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

// Both the This device and Global defaults copy of a setting.
const both = (k, terms) => ({ ['cfg-' + k]: terms, ['gcfg-' + k]: terms });

export const DICT = {
  // ---- sections
  'this-device': ['this browser', 'local', 'personal', 'preferences', 'prefs', 'appearance', 'display', 'device', 'my settings', 'theme', 'dark', 'light', 'colour', 'color', 'clock', 'time', '24h', 'layout', 'my default', 'set as my default', 'clear my default', 'profile', 'follow'],
  'global-defaults': ['global', 'everyone', 'default', 'defaults', 'all devices', 'all browsers', 'shared', 'appearance', 'preferences'],
  'router-source-destination-names': ['names', 'source names', 'destination names', 'labels', 'refresh', 'poll', 'reread', 'router names', 'name length', 'characters', 'truncate'],
  'level-colors': ['level colours', 'colour', 'color', 'colors', 'palette', 'hex', 'swatch', 'level', 'levels', 'rgb'],
  'takes': ['take', 'patch', 'route', 'confirm', 'confirm dialog', 'lockout', 'letter', 'enter', 'default levels', 'resync', 'reread', 'delay', 'safety'],
  'branding': ['brand', 'logo', 'name', 'title', 'border', 'colour', 'color', 'icon', 'identify', 'favicon', 'tab'],
  'router': ['ip', 'address', 'host', 'hostname', 'port', 'connection', 'connect', 'tcp', 'ultrix', 'swp08', 'sw-p-08', 'status', 'server'],
  'matrix': ['level', 'levels', 'offset', 'number', 'numbering', 'wire', 'matrix', 'zero', 'start at 0', 'start at 1', 'count', 'router levels', 'video', 'audio', 'va'],
  'server-backup': ['server configuration', 'config file', 'config.yaml', 'backup', 'restore', 'export', 'import', 'secrets', 'pins', 'download', 'upload'],
  'access': ['password', 'pin', 'lock', 'locked', 'unlock', 'security', 'admin', 'passcode', 'protect', 'tries', 'wrong', 'lockout', 'timeout', 'idle', 'auto lock', 'auto-lock', 'permissions', 'login', 'settings pin'],
  'sode-networking': ['audit', 'networking', 'network', 'proxy', 'proxies', 'trusted', 'x-forwarded-for', 'forwarded', 'ip address', 'client ip', 'reverse proxy', 'log', 'logging', 'who'],
  'history': ['recent', 'undo', 'clear', 'delete', 'erase', 'filter', 'filters', 'filter sets', 'set', 'search', 'hidden', 'hide'],
  'users': ['user', 'users', 'people', 'operators', 'names', 'pin', 'password', 'passwords', 'lock', 'lock screen', 'auto lock', 'auto-lock', 'timeout', 'idle', 'inactivity', 'logout', 'access', 'who', 'accounts', 'permissions'],
  'export-import': ['export', 'import', 'backup', 'restore', 'yaml', 'file', 'download', 'upload', 'share', 'share link', 'move', 'copy', 'transfer', 'another site', 'studio', 'settings file', 'save', 'load', 'presets', 'filter sets'],
  'transfer-open': ['export', 'import', 'backup', 'restore', 'yaml', 'file', 'download', 'upload', 'share link', 'review', 'transfer', 'copy to another site'],
  // Sections other branches may add; ignored until they exist.
  'salvos': ['preset', 'presets', 'salvo', 'salvos', 'scene', 'batch', 'multi take', 'macro', 'group'],
  'fire-mode': ['fire', 'native', 'sync', 'salvo', 'router salvo', 'simultaneous', 'together'],
  'grids': ['grid', 'grids', 'panel', 'panels', 'tablet', 'buttons'],

  // ---- This device / Defaults
  ...both('theme', ['dark', 'light', 'colour', 'color', 'mode', 'night', 'appearance', 'theme', 'dark mode', 'light mode']),
  ...both('layout', ['desktop', 'phone', 'mobile', 'tablet', 'touch', 'size', 'narrow', 'responsive']),
  ...both('clock', ['time', '24h', '24 hour', '12 hour', '12h', 'am pm', 'hours', 'format', 'clock', 'timestamp']),
  ...both('flip', ['order', 'swap', 'reverse', 'destination first', 'source first', 'dst', 'src', 'boxes', 'flip']),
  ...both('scrolltop', ['scroll', 'top', 'jump', 'focus', 'page', 'scroll to top']),
  ...both('search-filters', ['quick search', 'search', 'filter', 'filters', 'hidden', 'respect', 'history search']),
  ...both('set-stack', ['stack', 'stacked', 'filter sets', 'sets', 'and', 'or', 'combine', 'all', 'any', 'filter']),
  ...both('next-mode', ['next', 'increment', 'plus one', 'number', 'fuzzy', 'chip', 'ctrl shift s', 'following']),

  // ---- My defaults (per-row buttons shown while logged in)
  ...Object.fromEntries(['theme', 'layout', 'clock', 'flip', 'scrollTop', 'searchFilters', 'setStack', 'nextMode'].flatMap((n) => [['my-default-' + n, ['my default', 'save as my default', 'set as my default', 'profile', 'personal', 'follow me', 'account']], ['my-default-clear-' + n, ['my default', 'clear my default', 'remove', 'profile', 'personal', 'account']]])),

  // ---- Account (inside This device; only shown while logged in as a user)
  'account-profile': ['profile', 'my profile', 'my settings', 'user settings', 'presets', 'filters', 'filter sets', 'account'],
  'account-transfer': ['export', 'import', 'backup', 'restore', 'share', 'transfer', 'my settings', 'presets', 'filter sets', 'account'],
  'lock-pages': ['lock', 'lock main screen', 'lock communication log', 'lock log', 'pin', 'users', 'protect', 'screen lock'],
  'cfg-auto-lock': ['auto lock', 'auto-lock', 'idle', 'inactivity', 'timeout', 'minutes', 'lock after', 'hand lock'],
  'config-file': ['server configuration', 'config file', 'config.yaml', 'yaml', 'backup', 'restore', 'export', 'import', 'download', 'upload', 'migrate', 'copy settings'],
  'config-export': ['export', 'download', 'save', 'backup', 'config file', 'config.yaml', 'yaml', 'server configuration', 'settings file'],
  'config-import': ['import', 'upload', 'restore', 'load', 'replace', 'config file', 'config.yaml', 'yaml', 'server configuration', 'settings file'],
  'config-pins': ['include pins', 'private backup', 'secret', 'hashes', 'pin', 'password', 'backup', 'export'],
  'users-manage': ['manage users', 'add user', 'remove user', 'edit users', 'users', 'pin', 'trusted', 'never log out', 'persistent', 'sign out everywhere', 'lock main screen', 'lock log', 'auto lock', 'idle', 'longest pin'],
  'account': ['account', 'logged in', 'login', 'logout', 'log out', 'sign out', 'signout', 'user', 'me', 'my', 'session', 'password', 'pin'],
  'account-pin': ['pin', 'password', 'passcode', 'change', 'my pin', 'account', 'reset'],
  'soft-lock': ['lock', 'lock screen', 'screen lock', 'keyboard', 'mashing', 'accidental', 'cat', 'clean', 'unlock', 'ctrl+l', 'letter', 'protect'],
  'soft-lock-btn': ['lock', 'lock screen', 'keyboard', 'mashing', 'accidental', 'unlock', 'ctrl+l'],
  'account-login': ['log in', 'login', 'sign in', 'signin', 'user', 'pin', 'account', 'session', 'tab', 'name'],
  'account-logout': ['logout', 'log out', 'sign out', 'signout', 'session', 'account', 'lock', 'leave'],

  // ---- Names
  'names-refresh': ['refresh', 'reload', 'update', 'reread', 'poll', 'names', 'now', 'fetch'],
  'cfg-namerefresh': ['poll', 'interval', 'seconds', 'refresh', 'how often', 'timer', 'off', 'names'],
  'cfg-namelen': ['length', 'characters', 'chars', 'width', 'short', 'long', '4', '8', '12', '16', 'truncate'],
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
  'cfg-offset': ['offset', 'wire', 'level offset', 'shift', 'level', 'zero based', 'one based'],
  'cfg-lvlnum': ['level numbering', 'level', 'numbering', 'number', 'v', 'va', 'video', 'zero', '0', '1', 'start'],
  'cfg-numoff': ['number', 'numbers', 'offset', 'start at 0', 'start at 1', 'zero', 'one', 'sources', 'destinations', 'numbering', 'index'],

  // ---- Access
  'cfg-pin': ['pin', 'password', 'passcode', 'lock', 'secret', 'admin', 'protect settings', 'security', 'change pin'],
  'cfg-pin2': ['pin', 'password', 'confirm', 'repeat', 'retype', 'again'],
  'cfg-pin-clear': ['remove pin', 'delete pin', 'no pin', 'unlock', 'disable', 'password off', 'clear'],
  'cfg-pin-tries': ['wrong pin', 'tries', 'attempts', 'lock out', 'lockout', 'brute force', 'incorrect', 'guesses', 'reset pin'],

  // ---- Audit
  'cfg-proxies': ['proxy', 'proxies', 'trusted', 'network', 'x-forwarded-for', 'forwarded', 'reverse proxy', 'nginx', 'ip', 'client ip', 'cidr', 'tls', 'https'],

  // ---- History
  'cfg-hidden': ['hidden', 'hide', 'destinations', 'dst', 'filter', 'global filter', 'panel', 'mv', 'multiviewer', 'recent'],
  'clear-history': ['clear', 'delete', 'erase', 'wipe', 'reset', 'undo', 'history', 'recent', 'remove all'],
  'open-log': ['log', 'communication log', 'audit', 'uptime', 'messages', 'sent', 'wire', 'router log', 'events', 'tx', 'ack', 'diagnostics', 'debug', 'graph'],
};

// ---- matching

const words = (s) => String(s).toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim().split(' ').filter(Boolean);

// Edit distance, stopping early past `max`.
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

// How well one typed word matches one word of text: 0 = not at all.
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

// sections: [{ id, title, settings: [{ id, label }] }] -> flat index, in page order.
export function buildIndex(sections, dict = DICT) {
  const out = [];
  for (const sec of sections) {
    const sw = words(sec.title);
    const mk = (kind, id, label) => ({
      kind, id, sectionId: sec.id, section: sec.title, label,
      lw: words(label), tw: (dict[id] || []).flatMap(words), sw,
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

// The entries matching q, best first (ties keep page order); `.total` counts
// the matches before the cut to n.
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
