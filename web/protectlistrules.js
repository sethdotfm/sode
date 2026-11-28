// Settings > Routers > Protects: the groups, the find and the filters of the list, its count line, and Free Sode's
// Protects by router. Pure, so Node tests it (web/test/protectlistrules.test.mjs). settingsprotects.js draws it.
//
// A row is { key, uid, n, num, label, hold }. uid is the routerData UID (blank is the first router). num is the number
// as shown, with its prefix (B.23). label is the number and the name. hold is a hold of holdsOn (protectrules.js).
// A router is { uid, prefix, title, link, device, at }. link is the router's link state. device is Sode's device
// number there (0: Protect is off). at is the time of its last known rows, while the link is not up.

import { KIND_ROUTER_LOCK } from './protectrules.js';

// The kind filters, each with its test. Sode's are every Protect that Sode set; Mine are the signed-in user's.
export const KINDS = [
  { id: 'all', text: 'All', test: () => true },
  { id: 'sode', text: "Sode's", test: (r) => r.hold.sode },
  { id: 'mine', text: 'Mine', test: (r) => r.hold.mine },
  { id: 'other', text: 'Other devices', test: (r) => !r.hold.sode && r.hold.kind !== KIND_ROUTER_LOCK },
  { id: 'lock', text: 'Router locks', test: (r) => r.hold.kind === KIND_ROUTER_LOCK },
];

// With more routers than this, the router filter is a select, not a row of buttons.
export const ROUTER_BUTTONS_MAX = 6;

export const ALL_ROUTERS = '*';

// One key for each hold: a destination can have Sode's Protect on some levels and another device's on others.
export const rowKey = (uid, n, h) => `${uid}|${n}|${h.sode ? 's' : h.kind}|${h.device}`;

const kindTest = (kind) => (KINDS.find((k) => k.id === kind) || KINDS[0]).test;

// The find box takes a name, a number as shown (B.23, 23) or part of a name. Case does not count.
export function findMatch(row, q) {
  const t = q.trim().toLowerCase();
  if (!t) return true;
  if (row.label.toLowerCase().includes(t)) return true;
  const bare = row.num.includes('.') ? row.num.slice(row.num.indexOf('.') + 1) : row.num;
  return t === bare || t === row.num.toLowerCase();
}

// f: { q, kind, router }. router is a UID or ALL_ROUTERS.
export const matches = (row, f) => (f.router === ALL_ROUTERS || f.router === row.uid) && kindTest(f.kind)(row) && findMatch(row, f.q || '');

export const filtered = (f) => !!(f.q || '').trim() || f.kind !== 'all' || f.router !== ALL_ROUTERS;

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

// "158 Protects and Router locks on 4 routers. Sode set 91." With a filter on: "Shows 12 of 158."
export function countLine(rows, routers, f) {
  if (!rows.length) return 'No Protects or Router locks.';
  if (filtered(f)) return `Shows ${rows.filter((r) => matches(r, f)).length} of ${rows.length}.`;
  const what = rows.length === 1 ? 'Protect or Router lock' : 'Protects and Router locks';
  return `${rows.length} ${what} on ${plural(routers.length, 'router', 'routers')}. Sode set ${rows.filter((r) => r.hold.sode).length}.`;
}

// The count on each kind button: in the router and find of f, so the counts say what a press shows.
export function kindCounts(rows, f) {
  const out = {};
  for (const k of KINDS) out[k.id] = rows.filter((r) => matches(r, { ...f, kind: k.id })).length;
  return out;
}

// The count on each router button, in the kind and find of f.
export function routerCounts(rows, routers, f) {
  const out = { [ALL_ROUTERS]: rows.filter((r) => matches(r, { ...f, router: ALL_ROUTERS })).length };
  for (const rt of routers) out[rt.uid] = rows.filter((r) => matches(r, { ...f, router: rt.uid })).length;
  return out;
}

export const isUp = (rt) => rt.link === 'up';

// Why Sode cannot free a Protect on router rt now, or '' when it can. linked: the page's own link to Sode is up.
export function freeBlock(rt, linked = true) {
  if (!linked) return 'The link to the Sode server is down.';
  if (!rt.device) return "Protect is off for this router, because Sode's device number is 0.";
  if (!isUp(rt)) return 'The router is not connected.';
  return '';
}

// The groups to draw: one for each router of f.router, each with its rows that match f. With a filter on, a router
// with no match and no result to show is left out. results: uid → the last Free Sode's Protects result there.
export function groupsOf(rows, routers, f, results = {}) {
  const on = filtered(f);
  const out = [];
  for (const rt of routers) {
    if (f.router !== ALL_ROUTERS && f.router !== rt.uid) continue;
    const all = rows.filter((r) => r.uid === rt.uid);
    const shown = all.filter((r) => matches(r, f));
    if (on && !shown.length && !results[rt.uid]) continue;
    const sode = all.filter((r) => r.hold.sode).length;
    out.push({ router: rt, rows: shown, total: all.length, sode, other: all.length - sode, stale: !isUp(rt) });
  }
  return out;
}

// The words of a group heading's count: "42 here. Sode set 30. Other devices set 12."
export const groupCount = (g) => (g.total ? `${g.total} here. Sode set ${g.sode}. Other devices set ${g.other}.` : 'None.');

const clock = (at) => {
  const d = at instanceof Date ? at : new Date(at);
  return [d.getHours(), d.getMinutes(), d.getSeconds()].map((x) => String(x).padStart(2, '0')).join(':');
};

// The note of a router that is not up: its rows are the last ones that Sode knows.
export function staleNote(rt) {
  if (isUp(rt)) return '';
  if (!rt.at) return 'Sode has not read the Protects of this router while this page was open. These rows can be old.';
  return `These are the last known rows, read at ${clock(rt.at)}.`;
}

// The destinations of Sode's Protects on each router: Free Sode's Protects frees each destination once.
export function sodeDsts(rows, uid) {
  return new Set(rows.filter((r) => r.uid === uid && r.hold.sode).map((r) => r.n)).size;
}

// The dialog of Free Sode's Protects: one line for each router, with its count and why it cannot be ticked. only: a
// router UID to tick alone ('' ticks each router that it can).
export function freeLines(rows, routers, only = '', linked = true) {
  return routers.map((rt) => {
    const count = sodeDsts(rows, rt.uid);
    const block = freeBlock(rt, linked);
    const why = block || (count ? '' : 'Sode has no Protect on this router.');
    return { uid: rt.uid, router: rt, count, why, can: !why, ticked: !why && (!only || only === rt.uid) };
  });
}

// The dialog's button: "Free 86 Protects on 2 routers", or "Free" (off) with nothing ticked.
export function freeButton(lines, ticked) {
  const on = lines.filter((l) => l.can && ticked.has(l.uid));
  const n = on.reduce((s, l) => s + l.count, 0);
  if (!n) return { text: 'Free', disabled: true };
  return { text: `Free ${plural(n, 'Protect', 'Protects')} on ${plural(on.length, 'router', 'routers')}`, disabled: false };
}

// The result of Free Sode's Protects: one line for each router (uid → { ok, text }) and the status line.
// m is the protectClearAllResult. prefixOf gives a router's prefix from its UID.
export function freeResult(m, prefixOf, at = new Date()) {
  const per = {};
  const parts = [];
  for (const r of m.routers || []) {
    const p = prefixOf(r.router) || r.router;
    const freed = r.cleared ? `Sode freed ${plural(r.cleared, 'Protect', 'Protects')} at ${clock(at)}.` : '';
    if (r.err) {
      per[r.router] = { ok: false, text: `${freed}${freed ? ' ' : ''}Not freed: ${r.err}.` };
      parts.push(`${p}: ${r.err}`);
    } else {
      per[r.router] = { ok: true, text: freed || `Sode had no Protect to free at ${clock(at)}.` };
      parts.push(`${r.cleared} on ${p}`);
    }
  }
  const fail = !m.ok;
  const head = fail
    ? m.cleared
      ? `Sode freed ${plural(m.cleared, 'Protect', 'Protects')}, with problems`
      : 'Not freed'
    : `Sode freed ${plural(m.cleared || 0, 'Protect', 'Protects')}`;
  const tail = (parts.length ? parts.join(', ') : m.err || '').replace(/\.+$/, '');
  return { per, ok: !fail, text: `${head}${tail ? `: ${tail}` : ''}.` };
}

const why = (err) => (err || 'refused').trim().replace(/\.+$/, '');

// The words of a row's result after Free in the row. ok: the row's Free Protect worked. err: why not.
export const rowResult = (ok, err) => (ok ? { ok: true, text: 'Protect freed' } : { ok: false, text: `Not freed: ${why(err)}.` });

// The status line under the list after a Free in the row.
export const rowStatus = (ok, label, err) => (ok ? `Protect freed: ${label}, every level.` : `Free Protect failed: ${label}: ${why(err)}.`);
