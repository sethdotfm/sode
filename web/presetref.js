// Stored preset items hold router references { router, n, name } (internal/config/presets.go).
// Pages work on numbers: an item in memory is { src, dsts, levels, router, srcName, dstNames }.
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

// Reads an item in memory (plain numbers) or a reference. A stored item goes through storedItemNums. Names follow
// the first reference to a number.
export function itemNums(it) {
  const dstNames = { ...(it?.dstNames || {}) };
  for (const d of [].concat(it?.dsts ?? [])) {
    const n = num(d);
    if (Number.isInteger(n) && nameOf(d) && !dstNames[n]) dstNames[n] = nameOf(d);
  }
  const src = num(it?.src);
  return {
    src: Number.isInteger(src) ? src : -1,
    dsts: [...new Set(ints(it?.dsts))],
    levels: [...new Set(ints(it?.levels))].sort((a, b) => a - b),
    router: routerOf(it?.src) || (typeof it?.router === 'string' ? it.router : ''),
    srcName: nameOf(it?.src) || (typeof it?.srcName === 'string' ? it.srcName : ''),
    dstNames,
  };
}

// A stored item: only references count, so a bare number reads as no source and is left out of the destinations.
const refsOnly = (a) => [].concat(a ?? []).filter((x) => x && typeof x === 'object');
export const storedItemNums = (it) => itemNums({ ...it, src: it?.src && typeof it.src === 'object' ? it.src : undefined, dsts: refsOnly(it?.dsts) });

// Plain numbers only, for code that never stores (panels, plans).
export const itemPlain = (it) => {
  const { src, dsts, levels } = itemNums(it);
  return { src, dsts, levels };
};

// The stored form. router: the item's own, else this install's router. live(kind, n) is the
// router's name now, else the name the item came with.
export function itemRefs(it, router, live = () => '') {
  const r = it.router || router || '';
  const ref = (n, kept, kind) => {
    const name = live(kind, n) || kept || '';
    return name ? { router: r, n, name } : { router: r, n };
  };
  return {
    src: ref(it.src, it.srcName, 'src'),
    dsts: it.dsts.map((n) => ref(n, it.dstNames?.[n], 'dst')),
    levels: it.levels,
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
