// What a panel page reads from its URL and from /api/v1, and the text it shows. Pure.
// A stored panel's cells are { r, c, rs?, cs?, kind, n?, scope?, name?, caption?, levels? } with r and c
// counted from 1 and kind one of KINDS (docs/panels-design.md, Panel page). A Preset cell holds
// scope ('everyone' | 'me') and name instead of n.

export const KINDS = ['source', 'destination', 'take', 'preset', 'clear', 'current'];
const NEEDS_NUMBER = ['source', 'destination'];
export const PRESET_SCOPES = ['everyone', 'me'];

// The scope is a fixed word, so a colon in the name cannot make two presets collide.
export const presetKey = (scope, name) => `${scope}:${name}`;
const hasPreset = (c) => PRESET_SCOPES.includes(c.scope) && typeof c.name === 'string' && c.name !== '';

// /panels/<id>. The list and the editor are other pages.
export function panelIdFrom(pathname) {
  const m = /\/panels\/([^/]+)\/?$/.exec(pathname);
  return m && m[1] !== 'editor' ? m[1] : '';
}

// A key in the fragment never reaches a request line or a proxy log. Without one the panel runs as the signed-in user.
export const keyFromHash = (hash) => /(?:^#|&)key=([^&]+)/.exec(hash)?.[1] || '';

// Kiosk (GLOSSARY.md) is the panel's setting, or a link that asks for it without editing the panel.
export const kioskOf = (hash, panel) => !!panel?.settings?.kiosk || /(?:^#|&)kiosk=1(?:&|$)/.test(hash);

// A key in the link that the server refuses was rotated or revoked; only a new link helps.
export function openFailText(status, hadKey) {
  if (status === 404) return 'No panel here that you can use.';
  if (status === 401 && hadKey) return 'This link\'s key no longer works. It was rotated or revoked, so ask for a new link.';
  return 'This panel could not be opened.';
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
  const plain = { clear: 'Clear', current: 'Currently', preset: (c) => c.name };
  const of = (cell) => {
    if (cell.caption) return cell.caption;
    if (cell.kind === 'source') return source(cell.n);
    if (cell.kind === 'destination') return destination(cell.n);
    const p = plain[cell.kind];
    return typeof p === 'function' ? p(cell) : p || '';
  };
  return { source, destination, of, off };
}

// The cells that are whole, known and inside the grid. Where two overlap the first one stays.
export function cellsIn(panel) {
  const used = new Set();
  const out = [];
  for (const c of panel.cells || []) {
    if (!c || !KINDS.includes(c.kind)) continue;
    if (NEEDS_NUMBER.includes(c.kind) && !Number.isInteger(c.n)) continue;
    if (c.kind === 'preset' && !hasPreset(c)) continue;
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
    case 'preset-multi':
      return 'This panel stages one destination at a time, and that preset has more';
    case 'refused':
      return err.message || 'The take was refused';
    default:
      return '';
  }
}
