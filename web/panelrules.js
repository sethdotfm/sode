// What a panel page reads from its URL and from /api/v1, and the text it shows. Pure.
// A stored panel's cells are { r, c, rs?, cs?, kind, n?, scope?, name?, caption?, levels? } with r and c
// counted from 1 and kind one of KINDS (docs/panels-design.md, Panel page). A Preset cell holds
// scope ('everyone' | 'me') and name instead of n. A Go to Page cell holds page, its target Page's id (panelnav.js).

import { pageCells } from './panelpages.js';
import { fullKey } from './panellink.js';

export const KINDS = ['source', 'destination', 'take', 'preset', 'clear', 'current', 'panelName', 'instanceName', 'pageName', 'info', 'goToPage', 'nextPage', 'previousPage'];
// Kinds that only show something and are never pressed.
export const SHOW_KINDS = ['current', 'panelName', 'instanceName', 'pageName', 'info'];
const NEEDS_NUMBER = ['source', 'destination'];
export const PRESET_SCOPES = ['everyone', 'me'];

// The scope is a fixed word, so a colon in the name cannot make two presets collide.
export const presetKey = (scope, name) => `${scope}:${name}`;
const hasPreset = (c) => PRESET_SCOPES.includes(c.scope) && typeof c.name === 'string' && c.name !== '';
// A target whose Page was deleted still counts, so the panel page can draw it greyed (panelnav.js goToState).
const hasTarget = (c) => typeof c.page === 'string' && c.page !== '';

// /panels/<id>. The list and the editor are other pages.
export function panelIdFrom(pathname) {
  const m = /\/panels\/([^/]+)\/?$/.exec(pathname);
  return m && m[1] !== 'editor' ? m[1] : '';
}

// A key in the fragment never reaches a request line or a proxy log. Without one the panel runs as the signed-in user.
export const keyFromHash = (hash) => fullKey(/(?:^#|&)key=([^&]+)/.exec(hash)?.[1] || '');

// Kiosk (GLOSSARY.md) is the panel's setting, or a link that asks for it without editing the panel.
export const kioskOf = (hash, panel) => !!panel?.settings?.kiosk || /(?:^#|&)kiosk=1(?:&|$)/.test(hash);

// A key in the link that the server refuses was rotated or revoked; only a new link helps.
export function openFailText(status, hadKey) {
  if (status === 404) return 'No panel here that you can use.';
  if (status === 401 && hadKey) return "This link's key no longer works. It was rotated or revoked, so ask for a new link.";
  return 'This panel could not be opened.';
}

// The note above a Session-mode panel for a session key step. A refused or ended key gets text, or the page stays blank.
// retry shows the Sign in button; an ended module only a reload revives.
export function sessionNote(step, err = '') {
  switch (step) {
    case 'sign-in':
      return { text: 'Sign in with your PIN to open this panel.', retry: true };
    case 'no-users':
      return { text: 'This panel needs a user, and Sode has none.', retry: false };
    case 'refused':
      return { text: `Sode would not open this panel for this sign-in${err ? `: ${err}` : ''}. Sign in again.`, retry: true };
    case 'ended':
      return { text: "This page's sign-in key was ended. Reload the page to sign in again.", retry: false };
    default:
      return { text: '', retry: false };
  }
}

// rights is the caller's own Rights from GET /panels/<id>: an unset list allows everything, an empty one nothing.
// Keep in step with config.Rights and outsideRights (internal/app/rights.go).
export const rightsAllow = (rights, list, v) => !Array.isArray(rights?.[list]) || rights[list].includes(v);

export function envFrom(tally, protectedReply, presetsReply = {}, rights = null) {
  const rows = new Map();
  for (const r of tally.routes || []) {
    if (!rows.has(r.dst)) rows.set(r.dst, new Map());
    rows.get(r.dst).set(r.level, r.src);
  }
  return {
    levels: (tally.levels || []).map((l) => l.level),
    tally: rows,
    protectedDsts: new Set((protectedReply.protected || []).map((p) => p.dst)),
    rights: rights || null,
    presets: new Map((presetsReply.presets || []).map((p) => [presetKey(p.scope, p.name), p.items || []])),
  };
}

export function namesFrom(reply) {
  const off = reply.numberOffset || 0;
  const by = (list) => new Map((list || []).map((x) => [x.n, x.name]));
  const src = by(reply.sources);
  const dst = by(reply.destinations);
  const source = (n) => src.get(n) || `Source ${n + off}`;
  const destination = (n) => dst.get(n) || `Destination ${n + off}`;
  const plain = {
    clear: 'Clear',
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
  // '' when the router has not named it, where source and destination make up a name.
  const routerName = (kind, n) => (kind === 'source' ? src : kind === 'destination' ? dst : new Map()).get(n) || '';
  return { source, destination, of, off, routerName };
}

// The cells that are whole, known and inside the grid. Where two overlap the first one stays. A stored panel gives
// Page i's (pageCells).
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

// The panel's fixed destinations (settings.fixedDests), [] when the setting is off. A list the server would refuse
// (fixedList in internal/app/panelfixed.go) also reads as off, so a hand-edited file cannot break the page.
export const FIXED_MAX = 16; // keep in step with panelFixedMax in internal/app/panelfixed.go
const DST_MAX = 0xffff; // keep in step with dstNumberMax in internal/app/panelfixed.go
export function fixedDestsOf(panel) {
  const list = panel?.settings?.fixedDests;
  const ok = Array.isArray(list) && list.length <= FIXED_MAX && list.every((n, i) => Number.isInteger(n) && n >= 0 && n <= DST_MAX && list.indexOf(n) === i);
  return ok ? list : [];
}

// What the panel page draws of Page i. A panel with fixed destinations leaves its Destination cells out; they stay
// stored, so the editor and the thumbnails keep using cellsIn.
export const drawnCells = (panel, i = 0) => {
  const cells = cellsIn(panel, i);
  return fixedDestsOf(panel).length ? cells.filter((c) => c.kind !== 'destination') : cells;
};

export function errorText(err, names) {
  switch (err?.code) {
    case 'no-destination':
      return 'Pick a destination first';
    case 'protected':
      return `${names.destination(err.dst)} is protected`;
    case 'no-common-levels':
      return `${names.source(err.src)} and ${names.destination(err.dst)} have no level in common`;
    case 'outside-rights': {
      if (err.quiet) return "That is outside this panel's rights";
      return `${err.what === 'source' ? names.source(err.src) : err.what === 'preset' ? err.name : names.destination(err.dst)} is outside this panel's rights`;
    }
    case 'no-preset':
      return 'That preset is not available to this panel';
    case 'preset-no-fixed':
      return `${err.name} has no route to this panel's destinations`;
    case 'preset-multi':
      return 'This panel stages one destination at a time, and that preset has more';
    case 'refused':
      return err.message || 'The take was refused';
    default:
      return '';
  }
}

// Offline is what the page shows over a blurred grid. Sode not answering wins: with no answer the router link is unknown.
export function offlineOf({ failed, routerUp }) {
  if (failed) return { title: 'Offline', detail: 'This page lost contact with Sode. Nothing you press will reach the router until it is back.' };
  if (!routerUp) return { title: 'Offline', detail: 'Sode lost contact with the router. Nothing you press will reach it until it is back.' };
  return null;
}

// What a Panel Name or Instance Name cell shows: the name, with the caption as a line above it. A page with no
// instance yet (still naming itself, or Test it live) shows the kind's own words.
export function nameCellText(cell, { panelName = '', instance = '' } = {}) {
  const value = cell.kind === 'panelName' ? panelName || 'Panel Name' : instance || 'Instance Name';
  return { caption: cell.caption || '', value };
}

// The status words for artwork a Copy or Push left out: the server's artSkipped ids for one panel, '' when none.
// The total is panelArtTotal in internal/app/panelstore.go.
export function artLeftOut(ids, panel = '') {
  const n = Array.isArray(ids) ? ids.length : 0;
  if (!n) return '';
  const of = panel ? ` of ${panel}` : '';
  return n === 1
    ? `1 image did not fit under the 64 MB artwork total${of} and was left out.`
    : `${n} images did not fit under the 64 MB artwork total${of} and were left out.`;
}

// The Current cell reads the viewed destination's name; a caption replaces the whole label.
export const currentLabel = (cell, destName) => cell.caption || (destName ? `${destName} is currently:` : 'Currently');
