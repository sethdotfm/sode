// Settings > Routers: the keys of the router list, Add router and Remove router, the shown name and the link state
// (settingsrouterdetail.js, settingsrouterlist.js). Pure, no imports (Node-tested). Keep the keys in step with
// internal/app/routeradd.go.

// routerDisplay: the one shown form of a router, the Router prefix and then the Router title ("B Studio B"). A router
// with no title, or with a title that only repeats the default ("Router B"), shows "Router B". With one router Sode
// hides the prefix (GLOSSARY.md), so callers use it only when there are two or more.
export function routerDisplay(r) {
  const title = String(r?.title || '').trim();
  const prefix = r?.prefix || '';
  return !title || title === `Router ${prefix}` ? `Router ${prefix}`.trim() : `${prefix} ${title}`.trim();
}

export const listKey = (uid, attr) => `router/${uid}/${attr}`;
export const addKey = (n) => `router/new:${n}/add`;
export const removeKey = (uid) => `router/${uid}/remove`;

// The keys of the router list: every setting of a router, its Remove, and Add router.
export const LIST_KEY = /^router\/[^/]+\/[A-Za-z]+$/;
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

const KINDS = [
  ['preset', 'preset', 'presets'],
  ['filterSet', 'filter set', 'filter sets'],
  ['panel', 'panel', 'panels'],
  ['tieLine', 'tie line', 'tie lines'],
];

const joinAnd = (l) => (l.length < 2 ? l.join('') : `${l.slice(0, -1).join(', ')} and ${l.at(-1)}`);

const countOf = (items, kinds) =>
  KINDS.filter(([kind]) => kinds.includes(kind))
    .map(([kind, one, many]) => {
      const n = items.filter((it) => it.kind === kind).length;
      return n ? `${n} ${n === 1 ? one : many}` : '';
    })
    .filter(Boolean);

// usesNote: what a Remove router breaks, from the routerUses answer. head is one sentence, lines name each item. Presets,
// filter sets and panels become Broken; a tie line with an end on the router goes.
export function usesNote(prefix, items) {
  if (!items.length) return { head: `Router ${prefix} is in no preset, filter set, panel or tie line.`, lines: [] };
  const broken = countOf(items, ['preset', 'filterSet', 'panel']);
  const ties = countOf(items, ['tieLine']);
  const name = (kind) => KINDS.find((k) => k[0] === kind)?.[1] || kind;
  const lines = items.map((it) => {
    const what = `${name(it.kind).replace(/^./, (c) => c.toUpperCase())} ${it.name}`;
    return it.kind === 'tieLine' ? what : `${what} (${it.owner || 'Everyone'})`;
  });
  const parts = [broken.length ? `makes ${joinAnd(broken)} Broken` : '', ties.length ? `removes ${ties[0]}` : ''].filter(Boolean);
  return { head: `Remove router ${prefix} ${parts.join(' and ')}:`, lines };
}

// The link state of a router in the Routers list, as an icon and words. A healthy link shows no color
// (docs/color-roles.md): Connected is plain text. Connecting is caution (state warn). Down and Never connected are
// failure (state error).
const LINK_STATES = {
  connected: { word: 'Connected', glyph: 'circle-check', state: '' },
  connecting: { word: 'Connecting', glyph: 'loader-circle', state: 'warn' },
  down: { word: 'Down', glyph: 'circle-x', state: 'error' },
  never: { word: 'Never connected', glyph: 'circle-x', state: 'error' },
};

// linkState: r is one entry of routerLinks (routerlinkrules.js), {link, err}. link is '' while the server link is down,
// so nothing is known and nothing shows (null). everUp: the router was connected at some time (the server's never flag is
// false). A link that connects for the first time is Connecting; one that failed is Down, or Never
// connected when it was never up.
export function linkState(r, everUp) {
  if (!r?.link) return null;
  const key = r.link === 'up' ? 'connected' : r.link === 'connecting' && !r.err ? 'connecting' : everUp ? 'down' : 'never';
  return { key, ...LINK_STATES[key] };
}
