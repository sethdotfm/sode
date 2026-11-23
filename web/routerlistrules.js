// Settings > Routers: the Routers list, with Add router and Remove router (settingsrouterlist.js). Pure, no imports
// (Node-tested). Keep the keys in step with internal/app/routeradd.go.

export const LIST_ATTRS = ['prefix', 'title', 'address', 'matrix'];
export const ATTR_LABELS = { prefix: 'Prefix', title: 'Title', address: 'Address', matrix: 'Matrix' };

export const listKey = (uid, attr) => `router/${uid}/${attr}`;
export const addKey = (n) => `router/new:${n}/add`;
export const removeKey = (uid) => `router/${uid}/remove`;

// The keys this list owns: a router's prefix, title, address and matrix, Add and Remove.
export const LIST_KEY = /^router\/[^/]+\/(prefix|title|address|matrix|add|remove)$/;
export const keyAttr = (key) => key.slice(key.lastIndexOf('/') + 1);
export const keyRouter = (key) => key.split('/')[1];

// freePrefix gives the first prefix that used does not hold: A to Z, then AA to ZZ.
export function freePrefix(used) {
  const have = new Set(used);
  const letters = [...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'];
  for (const p of [...letters, ...letters.flatMap((a) => letters.map((b) => a + b))]) {
    if (!have.has(p)) return p;
  }
  return '';
}

// readRouter turns the typed text of one row into the values it saves.
export function readRouter({ prefix = '', title = '', address = '', matrix = '0', levels }) {
  const out = {
    prefix: String(prefix).trim().toUpperCase(),
    title: String(title).trim(),
    address: String(address).trim(),
    matrix: Number(String(matrix).trim() || 0),
  };
  if (levels !== undefined) out.levels = Number(String(levels).trim() || 0);
  return out;
}

const whole = (n, lo, hi) => Number.isInteger(n) && n >= lo && n <= hi;

// routerListProblem: rows are the routers that stay after the save, each {name, prefix, title, matrix, levels?}. It
// gives the first problem as text, or ''.
export function routerListProblem(rows) {
  const seen = new Set();
  for (const r of rows) {
    if (!/^[A-Z]{1,2}$/.test(r.prefix)) return `${r.name}: the prefix must be one or two letters, A to Z.`;
    if (seen.has(r.prefix)) return `Two routers have the prefix ${r.prefix}. Give each router its own prefix.`;
    seen.add(r.prefix);
    if ([...r.title].length > 32) return `${r.name}: the title must be at most 32 characters.`;
    if (!whole(r.matrix, 0, 255)) return `${r.name}: the matrix must be a number from 0 to 255.`;
    if (r.levels !== undefined && r.levels !== 0 && !whole(r.levels, 1, 256)) return `${r.name}: the levels must be a number from 1 to 256.`;
  }
  return '';
}

const KINDS = [
  ['preset', 'preset', 'presets'],
  ['filterSet', 'filter set', 'filter sets'],
  ['panel', 'panel', 'panels'],
];

const joinAnd = (l) => (l.length < 2 ? l.join('') : `${l.slice(0, -1).join(', ')} and ${l.at(-1)}`);

// usesNote: what a Remove router breaks, from the routerUses answer. head is one sentence, lines name each item.
export function usesNote(prefix, items) {
  if (!items.length) return { head: `Router ${prefix} is in no preset, filter set or panel.`, lines: [] };
  const counts = KINDS.map(([kind, one, many]) => {
    const n = items.filter((it) => it.kind === kind).length;
    return n ? `${n} ${n === 1 ? one : many}` : '';
  }).filter(Boolean);
  const name = (kind) => KINDS.find((k) => k[0] === kind)?.[1] || kind;
  const lines = items.map((it) => `${name(it.kind).replace(/^./, (c) => c.toUpperCase())} ${it.name} (${it.owner || 'Everyone'})`);
  return { head: `Remove router ${prefix} makes ${joinAnd(counts)} Broken:`, lines };
}
