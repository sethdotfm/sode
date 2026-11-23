// What a panel page reads from its URL and from /api/v1, and the text it shows. Pure. Cell shapes: docs/panels-design.md,
// Panel page. The server fills the router references of Source and Destination cells on save (internal/app/panelrefs.go).

export const KEY_ENDED = 'Key rotated or revoked. Ask for a new link.';
export const SIGN_IN_ENDED = 'Sign-in ended. Reload to sign in.';

import { pageCells } from './panelpages.js';
import { fullKey } from './panellink.js';
import { itemPlain } from './presetref.js';
import { levelsOn } from './routerlevels.js';
import { numText, routerN } from './routerids.js';

export const KINDS = [
  'source',
  'destination',
  'take',
  'preset',
  'clear',
  'undo',
  'current',
  'panelName',
  'instanceName',
  'pageName',
  'info',
  'goToPage',
  'nextPage',
  'previousPage',
];
export const SHOW_KINDS = ['current', 'panelName', 'instanceName', 'pageName', 'info'];
const NEEDS_NUMBER = ['source', 'destination'];
export const PRESET_SCOPES = ['everyone', 'me'];

// The scope is a fixed word, so a colon in the name cannot make two presets collide.
export const presetKey = (scope, name) => `${scope}:${name}`;
// The preset's name now: the cell's id wins, so a renamed preset is still found. A cell with no id keeps its own name.
export const presetName = (env, cell) => (cell.id && env.presetNames?.get(presetKey(cell.scope, cell.id))) || cell.name;
const hasPreset = (c) => PRESET_SCOPES.includes(c.scope) && typeof c.name === 'string' && c.name !== '';
// A target whose Page was deleted still counts, so the panel page can draw it greyed (panelnav.js goToState).
const hasTarget = (c) => typeof c.page === 'string' && c.page !== '';

export function panelIdFrom(pathname) {
  const m = /\/panels\/([^/]+)\/?$/.exec(pathname);
  return m && m[1] !== 'editor' ? m[1] : '';
}

// A key in the fragment never reaches a request line or a proxy log. Without one the panel runs as the signed-in user.
export const keyFromHash = (hash) => fullKey(/(?:^#|&)key=([^&]+)/.exec(hash)?.[1] || '');

export const kioskOf = (hash, panel) => !!panel?.settings?.kiosk || /(?:^#|&)kiosk=1(?:&|$)/.test(hash);

export function openFailText(status, hadKey) {
  if (status === 404) return 'No panel here that you can use.';
  if (status === 401 && hadKey) return KEY_ENDED;
  return 'This panel could not be opened.';
}

// retry shows the Sign in button; an ended module only a reload revives.
export function sessionNote(step, err = '') {
  switch (step) {
    case 'sign-in':
      return { text: 'Sign in to open this panel.', retry: true };
    case 'no-users':
      return { text: 'This panel needs a user, and Sode has none.', retry: false };
    case 'refused':
      return { text: `Sode refused this sign-in${err ? `: ${err}` : ''}. Sign in again.`, retry: true };
    case 'ended':
      return { text: SIGN_IN_ENDED, retry: false };
    default:
      return { text: '', retry: false };
  }
}

// rights is the caller's own Rights from GET /panels/<id>: an unset list allows everything, an empty one nothing.
// Keep in step with config.Rights and outsideRights (internal/app/rights.go).
// List entries are router references {router, n, name} (refN in rightsrules.js); presets are ids.
// router: for 'levels', the UID of the router the level is on (envFrom's env.router).
export const rightsAllow = (rights, list, v, router) => {
  const l = list === 'levels' ? levelsOn(rights?.levels, router) : rights?.[list];
  return !Array.isArray(l) || l.some((x) => (x && typeof x === 'object' ? x.n : x) === v);
};

export function envFrom(tally, protectedReply, presetsReply = {}, rights = null) {
  const rows = new Map();
  const via = new Map();
  for (const r of tally.routes || []) {
    if (!rows.has(r.dst)) rows.set(r.dst, new Map());
    rows.get(r.dst).set(r.level, r.src);
    if (r.via) {
      if (!via.has(r.dst)) via.set(r.dst, new Map());
      via.get(r.dst).set(r.level, r);
    }
  }
  return {
    // router: the UID Rights levels and the panel's level subset name these levels by.
    router: tally.router || '',
    levels: (tally.levels || []).map((l) => l.level),
    // With two or more routers, each router's { router, levels } by place (mergeTally); envAt reads it.
    places: (tally.byPlace || []).map((t) => t && { router: t.router, levels: (t.levels || []).map((l) => l.level) }),
    tally: rows,
    // dst → Map(level → Via row): the routes that a tie line feeds (viarules.js).
    via,
    protectedDsts: new Set((protectedReply.protected || []).map((p) => p.dst)),
    rights: rights || null,
    presets: new Map((presetsReply.presets || []).map((p) => [presetKey(p.scope, p.name), (p.items || []).map(itemPlain)])),
    // Rights name presets by id.
    presetIds: new Map((presetsReply.presets || []).map((p) => [presetKey(p.scope, p.name), p.id])),
    presetNames: new Map((presetsReply.presets || []).filter((p) => p.id).map((p) => [presetKey(p.scope, p.id), p.name])),
  };
}

// reply is GET /names, or the answers of several routers on item numbers (mergeNames in routerids.js). num is the
// number a person reads, with the router prefix when the answer has prefixes.
// env for a route on the router at place (routerids.js): its router and levels, so a take on router B never sends
// router A's levels. A router whose tally has not come yet has no levels.
export function envAt(env, place) {
  if (!place) return env;
  const p = env.places?.[place];
  return { ...env, router: p?.router || '', levels: p?.levels || [] };
}

export function namesFrom(reply) {
  const off = reply.numberOffset || 0;
  const num = numText(reply);
  const by = (list) => new Map((list || []).map((x) => [x.n, x.name]));
  const src = by(reply.sources);
  const dst = by(reply.destinations);
  const source = (n) => src.get(n) || `Source ${num(n)}`;
  const destination = (n) => dst.get(n) || `Destination ${num(n)}`;
  const plain = {
    clear: 'Clear',
    undo: 'Undo',
    current: 'Currently',
    panelName: 'Panel Name',
    instanceName: 'Instance Name',
    pageName: 'Page Name',
    info: 'Info',
    goToPage: 'Go to Page',
    nextPage: 'Next Page',
    previousPage: 'Previous Page',
    preset: (c) => c.name,
  };
  const of = (cell) => {
    if (cell.caption) return cell.caption;
    if (cell.kind === 'source') return source(cell.n);
    if (cell.kind === 'destination') return destination(cell.n);
    const p = plain[cell.kind];
    return typeof p === 'function' ? p(cell) : p || '';
  };
  const routerName = (kind, n) => (kind === 'source' ? src : kind === 'destination' ? dst : new Map()).get(n) || '';
  return { source, destination, of, off, num, routerName };
}

// namesFrom for the editor's cells, which hold router numbers and their router. reply is GET /names of every router
// on item numbers (mergeNames), and places is its makePlaces. source and destination take first-router numbers.
export function cellNamesFrom(reply, places) {
  const all = namesFrom(reply);
  if (!places?.many) return all;
  const on = (c) => (NEEDS_NUMBER.includes(c?.kind) && Number.isInteger(c.n) ? { ...c, n: places.id(c.router || '', c.n) } : c);
  return { ...all, of: (c) => all.of(on(c)), routerName: (kind, n, router = '') => all.routerName(kind, places.id(router, n)) };
}

// Where two cells overlap the first one stays.
export function cellsIn(panel, i = 0) {
  const used = new Set();
  const out = [];
  for (const c of pageCells(panel, i)) {
    if (!c || !KINDS.includes(c.kind)) continue;
    if (NEEDS_NUMBER.includes(c.kind) && !Number.isInteger(c.n)) continue;
    if (c.kind === 'preset' && !hasPreset(c)) continue;
    if (c.kind === 'goToPage' && !hasTarget(c)) continue;
    const rs = c.rs || 1;
    const cs = c.cs || 1;
    if (!(c.r >= 1 && c.c >= 1 && rs >= 1 && cs >= 1 && c.r + rs - 1 <= panel.rows && c.c + cs - 1 <= panel.cols)) continue;
    const keys = [];
    for (let r = c.r; r < c.r + rs; r++) for (let k = c.c; k < c.c + cs; k++) keys.push(`${r},${k}`);
    if (keys.some((k) => used.has(k))) continue;
    keys.forEach((k) => used.add(k));
    out.push({ ...c, rs, cs });
  }
  return out;
}

// [] when the setting is off. A list the server would refuse (fixedList in internal/app/panelfixed.go) reads as off too.
export const FIXED_MAX = 16; // keep in step with panelFixedMax in internal/app/panelfixed.go.
const DST_MAX = 0xffff; // keep in step with dstNumberMax in internal/app/panelfixed.go.
// The server stores router references {router, n, name}, and the editor writes {n}; tests pass plain numbers. A panel
// page has item numbers (panelitems.js), so the check limits the router number part.
export function fixedDestsOf(panel) {
  const raw = panel?.settings?.fixedDests;
  const list = Array.isArray(raw) ? raw.map((x) => (x && typeof x === 'object' ? x.n : x)) : null;
  const ok = list && list.length <= FIXED_MAX && list.every((n, i) => Number.isInteger(n) && n >= 0 && routerN(n) <= DST_MAX && list.indexOf(n) === i);
  return ok ? list : [];
}

// Fixed destinations leave the Destination cells out of the page; they stay stored, so the editor keeps using cellsIn.
export const drawnCells = (panel, i = 0) => {
  const cells = cellsIn(panel, i);
  return fixedDestsOf(panel).length ? cells.filter((c) => c.kind !== 'destination') : cells;
};

export function errorText(err, names) {
  switch (err?.code) {
    case 'nothing-to-undo':
      return 'Nothing to undo';
    case 'no-destination':
      return 'Select a destination first';
    case 'protected':
      return `${names.destination(err.dst)} is protected`;
    // Keep in step with acrossText in gridpanelrights.js.
    case 'across-routers':
      return `${names.source(err.src)} and ${names.destination(err.dst)} are on two routers. Sode has no tie lines yet.`;
    case 'no-common-levels':
      return `${names.source(err.src)} and ${names.destination(err.dst)} have no level in common`;
    case 'outside-rights': {
      if (err.quiet) return "That is outside this panel's Rights";
      return `${err.what === 'source' ? names.source(err.src) : err.what === 'preset' ? err.name : names.destination(err.dst)} is outside this panel's Rights`;
    }
    case 'no-preset':
      return 'That preset is not available to this panel';
    case 'preset-no-fixed':
      return `${err.name} has no route to this panel's destinations`;
    case 'preset-multi':
      return 'Preset has more than one destination. This panel stages one.';
    case 'refused':
      return err.message || 'Sode refused the take';
    default:
      return '';
  }
}

// Whether a destination the Undo cell would restore moved between two envs (History's Undo or another tab). next is GET /panels/<id>/undo's `undo`.
export function undoMoved(was, env, next) {
  const row = (e, dst) => JSON.stringify([...(e.tally.get(dst) || [])].sort((x, y) => x[0] - y[0]));
  return !!next?.routes.some((r) => row(was, r.dst) !== row(env, r.dst));
}

export function undoneText(u, names) {
  if (!u) return '';
  if (u.failed.length) return `Undo failed on ${[...new Set(u.failed)].map(names.destination).join(', ')}`;
  const dsts = [...new Set(u.routes.map((r) => r.dst))];
  const back = u.routes.length === 1 ? ` back to ${names.source(u.routes[0].src)}` : '';
  return `Undone: ${dsts.map(names.destination).join(', ')}${back}`;
}

// No answer from Sode wins: with no answer the router link is unknown.
export function offlineOf({ failed, routerUp }) {
  if (failed) return { title: 'Offline', detail: 'No contact with Sode. Presses do nothing.' };
  if (!routerUp) return { title: 'Offline', detail: 'Sode lost the router. Presses do nothing.' };
  return null;
}

// A page with no instance yet (still naming itself, or the Live Editor) shows the kind's own words.
export function nameCellText(cell, { panelName = '', instance = '' } = {}) {
  const value = cell.kind === 'panelName' ? panelName || 'Panel Name' : instance || 'Instance Name';
  return { caption: cell.caption || '', value };
}

// artSkipped ids from the server for one panel. The total is panelArtTotal in internal/app/panelstore.go.
export function artLeftOut(ids, panel = '') {
  const n = Array.isArray(ids) ? ids.length : 0;
  if (!n) return '';
  const of = panel ? ` of ${panel}` : '';
  return n === 1 ? `1 image${of} left out: over the 64 MB artwork total.` : `${n} images${of} left out: over the 64 MB artwork total.`;
}

export const currentLabel = (cell, destName) => cell.caption || (destName ? `${destName} is currently:` : 'Currently');
