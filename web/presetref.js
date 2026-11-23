// Stored preset items hold router references { router, n, name } (internal/config/presets.go).
// Pages work on numbers: an item in memory is { src, dsts, levels, router, srcName, dstNames }. router is the router of
// the source. An item across routers (docs/tielines-design.md, Presets) also has dstRouter, the router of its
// destinations, and ties, the ids of its tie lines, upstream first. Other items have neither.
// Pure, no imports: panel pages read /api/v1/presets through it too.

const num = (x) => (x && typeof x === 'object' ? Number(x.n) : Number(x));
const nameOf = (x) => (x && typeof x === 'object' && typeof x.name === 'string' ? x.name.trim() : '');
const routerOf = (x) => (x && typeof x === 'object' && typeof x.router === 'string' ? x.router : '');

// Out-of-range values stay so presetProblem can name them rather than quietly change the preset.
const ints = (a) =>
  []
    .concat(a ?? [])
    .map(num)
    .filter((n) => Number.isInteger(n));

const strings = (a) =>
  []
    .concat(a ?? [])
    .filter((x) => typeof x === 'string' && x.trim() !== '')
    .map((x) => x.trim());

// Reads an item in memory (plain numbers) or a reference. A stored item goes through storedItemNums. Names follow
// the first reference to a number.
export function itemNums(it) {
  const dstNames = { ...(it?.dstNames || {}) };
  for (const d of [].concat(it?.dsts ?? [])) {
    const n = num(d);
    if (Number.isInteger(n) && nameOf(d) && !dstNames[n]) dstNames[n] = nameOf(d);
  }
  const src = num(it?.src);
  const router = routerOf(it?.src) || (typeof it?.router === 'string' ? it.router : '');
  const first = [].concat(it?.dsts ?? [])[0];
  const dstRouter = routerOf(first) || (typeof it?.dstRouter === 'string' ? it.dstRouter : '');
  const ties = strings(it?.ties);
  const across = !!dstRouter && dstRouter !== router;
  return {
    src: Number.isInteger(src) ? src : -1,
    dsts: [...new Set(ints(it?.dsts))],
    levels: [...new Set(ints(it?.levels))].sort((a, b) => a - b),
    router,
    srcName: nameOf(it?.src) || (typeof it?.srcName === 'string' ? it.srcName : ''),
    dstNames,
    ...(across ? { dstRouter } : {}),
    ...(across || ties.length ? { ties } : {}),
  };
}

// across: the item's destinations are on another router than its source, so it goes through tie lines.
export const across = (it) => !!it?.dstRouter && it.dstRouter !== it.router;

// A stored item: only references count, so a bare number reads as no source and is left out of the destinations.
const refsOnly = (a) => [].concat(a ?? []).filter((x) => x && typeof x === 'object');
export const storedItemNums = (it) => itemNums({ ...it, src: it?.src && typeof it.src === 'object' ? it.src : undefined, dsts: refsOnly(it?.dsts) });

// Plain numbers only, for code that never stores (panels, plans). ties stays, so a panel Preset cell sends the stored
// tie lines.
export const itemPlain = (it) => {
  const { src, dsts, levels, ties } = itemNums(it);
  return ties?.length ? { src, dsts, levels, ties } : { src, dsts, levels };
};

// The stored form. router: the item's own, else this install's router. live(kind, n, router) is the
// router's name now, else the name the item came with.
export function itemRefs(it, router, live = () => '') {
  const r = it.router || router || '';
  const d = it.dstRouter || r;
  const ref = (n, kept, kind, at) => {
    const name = live(kind, n, at) || kept || '';
    return name ? { router: at, n, name } : { router: at, n };
  };
  return {
    src: ref(it.src, it.srcName, 'src', r),
    dsts: it.dsts.map((n) => ref(n, it.dstNames?.[n], 'dst', d)),
    levels: it.levels,
    ...(d !== r ? { ties: [...(it.ties || [])] } : {}),
  };
}

// Names the editor cannot see in its rows: those of the item's numbers in the preset it loaded.
export function carryNames(items, from) {
  const src = new Map(),
    dst = new Map();
  for (const it of from || []) {
    if (it.srcName) src.set(it.src, it.srcName);
    for (const [n, name] of Object.entries(it.dstNames || {})) dst.set(Number(n), name);
  }
  return items.map((it) => ({
    ...it,
    srcName: it.srcName || src.get(it.src) || '',
    dstNames: Object.fromEntries(it.dsts.filter((n) => it.dstNames?.[n] || dst.has(n)).map((n) => [n, it.dstNames?.[n] || dst.get(n)])),
  }));
}

// A hidden id for a This browser preset or filter set; the server makes the others.
export function newPresetId() {
  const b = new Uint8Array(8);
  globalThis.crypto.getRandomValues(b);
  return [...b].map((x) => x.toString(16).padStart(2, '0')).join('');
}
