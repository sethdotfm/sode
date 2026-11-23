// Swap source and Swap destination (GLOSSARY.md, Swap): the list by owner, Only Broken and the value that the one
// Save's `swap` key or swapMine sends. Pure, no imports (Node-tested). The server is internal/app/swap.go.

export const EVERYONE = 'Everyone';

const ownerOf = (it) => it.owner || EVERYONE;

// [{owner, items}], Everyone first, then owners by name. onlyBroken keeps the items where the old one is Broken.
export function groupByOwner(items, onlyBroken) {
  const by = new Map();
  for (const it of items) {
    if (onlyBroken && !it.broken) continue;
    const o = ownerOf(it);
    if (!by.has(o)) by.set(o, []);
    by.get(o).push(it);
  }
  const names = [...by.keys()].filter((o) => o !== EVERYONE).sort((a, b) => a.localeCompare(b));
  return (by.has(EVERYONE) ? [EVERYONE, ...names] : names).map((owner) => ({ owner, items: by.get(owner) }));
}

// The ids ticked when a list comes. A tie line is a cable, so Sode never moves its end by itself: the Administrator
// ticks it (docs/tielines-design.md, Other stores).
export const firstTicked = (items) => items.filter((it) => it.kind !== 'tieLine').map((it) => it.id);

// The ids that are ticked and shown. null when none is, so nothing is staged or sent.
export function swapValue({ router, side, from, to }, items, ticked, onlyBroken) {
  const ids = items.filter((it) => ticked.has(it.id) && (!onlyBroken || it.broken)).map((it) => it.id);
  return ids.length ? { router, side, from, to, items: ids } : null;
}

// A number as typed (shown numbers start at the offset) as the router holds it, or null.
export function parseShown(text, offset) {
  const t = String(text).trim();
  if (!/^\d+$/.test(t)) return null;
  const n = Number(t) - offset;
  return n >= 0 && n <= 0xffff ? n : null;
}

export const swapLabel = (v, offset) => `Swap ${v.side} ${v.from + offset} for ${v.to + offset}`;

// "3 items: Everyone 1, Ana 1, Bo 1". items is the list the value was made from.
export function swapShow(v, items) {
  if (!v) return 'No Swap';
  const ids = new Set(v.items);
  const groups = groupByOwner(
    items.filter((it) => ids.has(it.id)),
    false,
  );
  const n = v.items.length;
  return `${n} ${n === 1 ? 'item' : 'items'}: ${groups.map((g) => `${g.owner} ${g.items.length}`).join(', ')}`;
}
