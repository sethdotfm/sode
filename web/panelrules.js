// What a panel page reads from its URL and from /api/v1, and the text it shows. Pure.
// A stored panel's cells are { r, c, rs?, cs?, kind: 'source' | 'destination' | 'take', n?, caption?, levels? }
// with r and c counted from 1 (docs/panels-design.md, Stored panel).

export const KINDS = ['source', 'destination', 'take'];
const NEEDS_NUMBER = ['source', 'destination'];

// /panels/<id>. The list and the editor are other pages.
export function panelIdFrom(pathname) {
  const m = /\/panels\/([^/]+)\/?$/.exec(pathname);
  return m && m[1] !== 'editor' ? m[1] : '';
}

// A key in the fragment never reaches a request line or a proxy log. Without one the panel runs as the signed-in user.
export const keyFromHash = (hash) => /(?:^#|&)key=([^&]+)/.exec(hash)?.[1] || '';

export function envFrom(tally, protectedReply) {
  const rows = new Map();
  for (const r of tally.routes || []) {
    if (!rows.has(r.dst)) rows.set(r.dst, new Map());
    rows.get(r.dst).set(r.level, r.src);
  }
  return {
    levels: (tally.levels || []).map((l) => l.level),
    tally: rows,
    protectedDsts: new Set((protectedReply.protected || []).map((p) => p.dst)),
  };
}

export function namesFrom(reply) {
  const off = reply.numberOffset || 0;
  const by = (list) => new Map((list || []).map((x) => [x.n, x.name]));
  const src = by(reply.sources);
  const dst = by(reply.destinations);
  const source = (n) => src.get(n) || `Source ${n + off}`;
  const destination = (n) => dst.get(n) || `Destination ${n + off}`;
  const of = (cell) => cell.caption || (cell.kind === 'source' ? source(cell.n) : cell.kind === 'destination' ? destination(cell.n) : '');
  return { source, destination, of };
}

// The cells that are whole, known and inside the grid. Where two overlap the first one stays.
export function cellsIn(panel) {
  const used = new Set();
  const out = [];
  for (const c of panel.cells || []) {
    if (!c || !KINDS.includes(c.kind)) continue;
    if (NEEDS_NUMBER.includes(c.kind) && !Number.isInteger(c.n)) continue;
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
    case 'refused':
      return err.message || 'The take was refused';
    default:
      return '';
  }
}
