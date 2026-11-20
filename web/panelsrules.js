// What the panels page (/panels) groups, labels and offers. canEdit comes from the server, which alone decides who can change a panel.
import { cellsIn } from './panelrules.js';
import { wordScore } from './findrules.js';
import { primary } from './keys.js';

const same = (a, b) => String(a).toLowerCase() === String(b).toLowerCase();

// 0 when some word matches no field. Each word scores on its best field with the editor Find's scoring; a name hit outranks an owner hit.
export function panelScore(p, q) {
  const words = String(q || '')
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
  if (!words.length) return 1;
  let total = 0;
  for (const w of words) {
    const best = Math.max(wordScore({ label: p.name || '' }, w), wordScore({ label: p.slug || '' }, w) * 0.9, wordScore({ label: p.owner || '' }, w) * 0.8);
    if (!best) return 0;
    total += best;
  }
  return total;
}

export const matches = (p, q) => panelScore(p, q) > 0;

// The stable sort keeps the list's order for equal scores and an empty query.
const ranked = (panels, q) =>
  String(q || '').trim()
    ? panels
        .map((p) => [p, panelScore(p, q)])
        .filter(([, s]) => s > 0)
        .sort((a, b) => b[1] - a[1])
        .map(([p]) => p)
    : panels;

export function groupPanels(panels, me, trusted, filter = '') {
  const shown = ranked(panels, filter);
  const mine = shown.filter((p) => same(p.owner, me));
  const others = shown.filter((p) => !same(p.owner, me));
  const out = [];
  if (mine.length) out.push({ title: 'My panels', panels: mine });
  if (!trusted) {
    if (others.length) out.push({ title: 'Shared with me', panels: others });
    return out;
  }
  const owners = [...new Set(others.map((p) => p.owner))].sort((a, b) => a.localeCompare(b));
  for (const o of owners) out.push({ title: o, panels: others.filter((p) => p.owner === o) });
  return out;
}

// The list's empty line: nothing yet differs from a Find with no hits. Blank until the list loads.
export const emptyText = (loaded, any) => (!loaded ? '' : any ? 'No panel matches.' : 'No panels yet. Press New panel to make one.');

export const actionsFor = (p) => (p.canEdit ? ['launch', 'edit', 'share', 'push', 'delete'] : ['launch', 'copy']);

// The server keeps names for Named users only.
export const shareDraft = (share) => ({ mode: share?.mode || 'me', users: share?.mode === 'named' ? [...(share.users || [])] : [] });

// A named user the Users list no longer has stays shown so it can be unticked.
export function pickList(people, owner, named) {
  const on = new Set(named.map((n) => n.toLowerCase()));
  const out = people.filter((u) => !same(u.name, owner)).map((u) => ({ name: u.name, on: on.has(u.name.toLowerCase()) }));
  for (const n of named) if (!out.some((u) => same(u.name, n)) && !same(n, owner)) out.push({ name: n, on: true });
  return out;
}

export function toggleUser(users, name, on) {
  const rest = users.filter((u) => !same(u, name));
  return on ? [...rest, users.find((u) => same(u, name)) ?? name] : rest;
}

// clientRights left out keeps the stored ones, so a share edit never touches the API client.
export function shareBody(p, draft) {
  const rest = { ...p };
  delete rest.canEdit;
  delete rest.clientRights;
  return { ...rest, share: draft.mode === 'named' ? { mode: 'named', users: [...draft.users] } : { mode: draft.mode } };
}

export function shareChanged(share, draft) {
  const was = shareDraft(share);
  if (was.mode !== draft.mode) return true;
  if (draft.mode !== 'named') return false;
  const key = (u) =>
    u
      .map((n) => n.toLowerCase())
      .sort()
      .join('\n');
  return key(was.users) !== key(draft.users);
}

export function deleteStep(armed, id) {
  return armed === id ? { armed: '', go: true } : { armed: id, go: false };
}

export const thumbCells = (p) => cellsIn(p).map(({ r, c, rs, cs, kind }) => ({ r, c, rs, cs, kind }));

// A Grid panel's picture: a Source bar at the top, a Destination bar at the side and a diagonal of routes, in three sizes.
// The page reads no names, so an "all" edge counts as GRID_ALL_ITEMS, which is Large.
export const GRID_ALL_ITEMS = 256;
export const GRID_MEDIUM_FROM = 13;
export const GRID_LARGE_FROM = 65;
export const GRID_SQUARES = { small: 3, medium: 5, large: 8 }; // keep in step with --sode-ov-cols in panels.css (squares + 1)
export const gridEdgeCount = (edge) => (Array.isArray(edge?.members) ? edge.members.length : GRID_ALL_ITEMS);
export const gridEdgeLabel = (edge) => (Array.isArray(edge?.members) ? String(edge.members.length) : 'ALL');
export function pictureSize(n) {
  if (n >= GRID_LARGE_FROM) return 'large';
  return n >= GRID_MEDIUM_FROM ? 'medium' : 'small';
}
// One character per square, centred. An odd number of spare squares sets half: the tile starts half a square in. The
// Destination bar reads from the bottom up, so its first character is in the lowest square. A label always fits: more than
// 2 characters means a Large picture (8 squares).
function edgeTiles(kind, label, n) {
  const src = kind === 'source';
  const chars = [...label];
  const first = 2 + Math.floor((n - chars.length) / 2);
  const half = (n - chars.length) % 2 === 1;
  return chars.map((ch, i) => {
    const at = src ? first + i : first + chars.length - 1 - i;
    return { kind, r: src ? 1 : at, c: src ? at : 1, rs: 1, cs: 1, label: ch, half };
  });
}
export function gridThumb(p) {
  const size = pictureSize(Math.max(gridEdgeCount(p.sources), gridEdgeCount(p.destinations)));
  const n = GRID_SQUARES[size];
  const cols = n + 1;
  const rows = n + 1;
  const cells = [];
  for (let c = 2; c <= cols; c++) cells.push({ r: 1, c, rs: 1, cs: 1, kind: 'source' });
  for (let r = 2; r <= rows; r++) cells.push({ r, c: 1, rs: 1, cs: 1, kind: 'destination' }, { r, c: r, rs: 1, cs: 1, kind: 'on' });
  const tiles = [...edgeTiles('source', gridEdgeLabel(p.sources), n), ...edgeTiles('destination', gridEdgeLabel(p.destinations), n)];
  return { size, rows, cols, cells, tiles };
}

const edgeSize = (edge, one) => (Array.isArray(edge?.members) ? `${edge.members.length} ${one}${edge.members.length === 1 ? '' : 's'}` : `all ${one}s`);
export const gridSize = (p) => `Grid, ${edgeSize(p.sources, 'source')} by ${edgeSize(p.destinations, 'destination')}`;

// `acts` `api`, `both` and `public` own an API client and hold keys. `both` falls back to Session mode when the page has
// no key, and `public` gets a short-lived key for any page. Keep in step with actsHasClient in internal/app/panelstore.go.
export const hasClient = (p) => p?.acts === 'api' || p?.acts === 'both' || p?.acts === 'public';

export const isPublic = (p) => p?.acts === 'public';

const LAUNCH_NOTE = { api: '', public: '', both: ' (a link without a key asks for a PIN)' };
export const launchNote = (p) => LAUNCH_NOTE[p.acts] ?? ' (asks for a PIN at launch)';

const SHORT = { api: 'API client', both: 'API client or the user', public: 'Public' };
const LABEL = {
  api: 'Acts as an API client',
  both: 'Acts as an API client with a key, else as the user',
  public: 'Acts as an API client for anyone with the link',
};

const MODE_NAME = { session: 'User', both: 'Both', api: 'API', public: 'Public' };

// Keep in step with apiPanelUpdate and pruneKeys. confirm: the switch makes or ends the client (docs/agents/panels.md). null: no switch.
export function actsSwitch(from, to) {
  const was = from || 'session';
  if (was === to) return null;
  const name = MODE_NAME[to];
  if (to === 'session') {
    return {
      confirm: true,
      name,
      text: `${name} ends the panel's API client, every instance key and every setup link. You cannot undo this.`,
    };
  }
  // Public opens the panel to anyone, so a switch to it always asks; a switch away ends the short-lived keys (endPublicKeys).
  if (to === 'public') {
    const how = hasClient({ acts: was }) ? "keeps the panel's API client and every instance key, and" : 'makes the panel its own API client and';
    return { confirm: true, name, text: `${name} ${how} lets anyone with the plain link use the panel with the client's Rights and no sign-in.` };
  }
  const plain = to === 'api' ? 'The plain link stops opening the panel.' : 'The plain link opens it as the signed-in user as well.';
  const left = was === 'public' ? ' Pages open without a key lose their Public key.' : '';
  if (hasClient({ acts: was })) return { confirm: false, name, text: `${name} keeps the panel's API client and every instance key. ${plain}${left}` };
  const next =
    to === 'api' ? 'Make links: Share > Link with key. The plain link stops.' : 'Make links: Share > Link with key. The plain link still opens as the user.';
  return { confirm: true, name, text: `${name} makes the panel its own API client, with no keys yet. ${next}` };
}

// A blank new panel's mode: the new-panel Authentication mode, but User while Public panels are not allowed (the server
// refuses Public then). Missing values are the server's defaults. Save a copy keeps its source's mode on the server.
export function newPanelActs(cfg) {
  const m = cfg?.newPanelActs || 'public';
  return m === 'public' && cfg?.publicPanels === false ? 'session' : m;
}

// The short form for a row, after the owner's name.
export const actsShort = (p) => SHORT[p.acts] || 'signs in as the user';

export const actsLabel = (p) => LABEL[p.acts] || 'Acts as the user';

const SAVED_NOTE = {
  api: 'Saved. Make links: Share > Link with key.',
  public: 'Saved. Public: anyone with the link can use it.',
  session: 'Saved. API client and keys ended.',
};

export const actsSavedNote = (acts) => SAVED_NOTE[acts] || 'Saved.';

// The server lets any page open a Public panel, whatever its share says (ADR 0008).
export const PUBLIC_WARNING = "Anyone with the link can use a Public panel, with its client's Rights. Its sharing does not limit who can open it.";

export function shareLabel(share) {
  if (share?.mode === 'everyone') return 'Everyone';
  if (share?.mode === 'named') return share.users?.length ? share.users.join(', ') : 'Nobody';
  return 'Only the owner';
}

export const launchHref = (id) => `panels/${id}`;
export const editHref = (id) => `panels/editor?panel=${id}`;

// 'launch' (Ctrl/Cmd+Enter), 'edit' (Ctrl/Cmd+E), 'next' or 'prev' (Ctrl/Cmd+J, K), else ''. Ctrl on any platform, Cmd only on a Mac
// (as listStep in keys.js), so the shortcuts work in a field too.
export function listKey(e, mac = false) {
  if (e.defaultPrevented || e.altKey || e.shiftKey) return '';
  if (!primary(e, mac)) return '';
  const key = e.code === 'Enter' || e.key === 'Enter' ? 'Enter' : String(e.key || '').toLowerCase();
  return { e: 'edit', Enter: 'launch', j: 'next', k: 'prev' }[key] || '';
}

// The id the highlight moves to; it stops at the ends.
export function stepPanel(ids, cur, step) {
  if (!ids.length) return '';
  const i = ids.indexOf(cur);
  return ids[Math.min(ids.length - 1, Math.max(0, (i < 0 ? 0 : i) + step))];
}

// instancesOnline of instancesKnown, hidden ones left out by the server. No instance at all reads All Offline.
export function panelStatus(p) {
  const on = p.instancesOnline || 0;
  const known = Math.max(p.instancesKnown || 0, on);
  if (known && on === known) return { text: 'All Online', tone: 'ok' };
  if (on) return { text: 'Some Online', tone: 'warn' };
  return { text: 'All Offline', tone: '' };
}

// Every instance Sode knows, then the watched ones it has not heard from since it started. Hidden ones are left out. Names ignore case.
export function shownInstances(p) {
  const hidden = new Set(p.hiddenInstances || []);
  const seen = new Set((p.instances || []).map((i) => i.device.toLowerCase()));
  const gone = (p.watchingInstances || []).filter((d) => !seen.has(d)).map((d) => ({ device: d, online: false, seen: '' }));
  return [...(p.instances || []), ...gone].filter((i) => !hidden.has(i.device.toLowerCase()));
}

export function hiddenShown(p) {
  const all = new Set([...(p.instances || []).map((i) => i.device.toLowerCase()), ...(p.watchingInstances || [])]);
  return (p.hiddenInstances || []).filter((d) => all.has(d));
}
