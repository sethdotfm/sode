// Pure, so Node tests it. Numbers carry names so an import on another router can match by name.

export const SOCKET_LIMIT = 64 * 1024;
const PROFILE_LIMIT = 48 * 1024;
export const NAME_MAX = 32;

const runes = (s) => [...String(s)].length;
const same = (a, b) => String(a).trim().toLowerCase() === String(b).trim().toLowerCase();
const ints = (a) =>
  []
    .concat(a ?? [])
    .map(Number)
    .filter((n) => Number.isInteger(n));

export const filterOf = (s) => {
  const f = s?.f || s || {};
  return {
    src: String(f.src ?? ''),
    dst: String(f.dst ?? ''),
    sode: f.sode !== false,
    ext: f.ext !== false,
    hidden: f.hidden === true,
  };
};

// prefs: { name: value }; filterSets: [{ name, f }] or flat; presets use internal numbers;
// srcName(n) / dstName(n): the router's name now ('' if unknown); now: Date or ms. site and
// user are left out when blank.
export function buildExport({ prefs, filterSets, presets, site, user, now = Date.now(), srcName = () => '', dstName = () => '' }) {
  const doc = { sode: 1 };
  if (site) doc.site = String(site);
  if (user) doc.user = String(user);
  doc.exported = new Date(now).toISOString().replace(/\.\d+Z$/, 'Z');
  if (prefs && Object.keys(prefs).length) doc.prefs = { ...prefs };
  if (filterSets?.length) doc.filterSets = filterSets.map((s) => ({ name: s.name, ...filterOf(s) }));
  if (presets?.length) doc.presets = presets.map((p) => ({ name: p.name, items: p.items.map((it) => itemOut(it, srcName, dstName)) }));
  return doc;
}

function itemOut(it, srcName, dstName) {
  const out = { src: it.src };
  const sn = srcName(it.src);
  if (sn) out.srcName = sn;
  out.dsts = it.dsts.map((n) => {
    const d = { n };
    const dn = dstName(n);
    if (dn) d.name = dn;
    return d;
  });
  if (it.levels?.length) out.levels = [...it.levels];
  return out;
}

export const itemIn = (it) => ({
  src: Number.isInteger(+it?.src) ? +it.src : -1,
  srcName: String(it?.srcName ?? ''),
  dsts: [].concat(it?.dsts ?? []).map((d) => ({ n: Number.isInteger(+d?.n) ? +d.n : -1, name: String(d?.name ?? '') })),
  levels: [...new Set(ints(it?.levels))].sort((a, b) => a - b),
});

// Names compare without regard to case and outer spaces. Statuses, best to worst: same, byName (one live
// item has that name: number remapped), ambiguous (several do: never the first), unmatched.
const STATUS = ['same', 'byName', 'ambiguous', 'unmatched'];
export const worst = (a, b) => (STATUS.indexOf(a) >= STATUS.indexOf(b) ? a : b);

export function matchEnd(n, name, list) {
  const at = list.find((x) => x.n === n);
  if (!String(name).trim()) return at ? { status: 'same', n } : { status: 'unmatched', n };
  if (at && same(at.name, name)) return { status: 'same', n };
  const hits = list.filter((x) => same(x.name, name));
  if (hits.length === 1) return { status: 'byName', n: hits[0].n, was: n };
  if (hits.length > 1) return { status: 'ambiguous', n, count: hits.length };
  return { status: 'unmatched', n };
}

// Result: each end's match, the worst status, the item as it would be saved (remapped where
// matched by name) and levels above the router's count.
export function matchItem(it, live) {
  const src = matchEnd(it.src, it.srcName, live.srcs);
  const dsts = it.dsts.map((d) => matchEnd(d.n, d.name, live.dsts));
  let status = src.status;
  for (const d of dsts) status = worst(status, d.status);
  const levelsOver = live.levelCount ? it.levels.filter((l) => l > live.levelCount) : [];
  return {
    src,
    dsts,
    status,
    levelsOver,
    saved: { src: src.n, dsts: [...new Set(dsts.map((d) => d.n))], levels: [...it.levels] },
  };
}

export function matchPreset(p, live) {
  const items = p.items.map((it) => matchItem(itemIn(it), live));
  let status = 'same';
  for (const m of items) status = worst(status, m.status);
  return {
    items,
    status,
    levelsOver: [...new Set(items.flatMap((m) => m.levelsOver))].sort((a, b) => a - b),
    count: {
      ambiguous: items.filter((m) => m.status === 'ambiguous').length,
      unmatched: items.filter((m) => m.status === 'unmatched').length,
      byName: items.filter((m) => m.status === 'byName').length,
    },
    saved: { name: p.name, items: items.map((m) => m.saved) },
  };
}

export const conflict = (name, existing) => existing.find((x) => same(x, name)) ?? null;

export function renameFor(name, taken) {
  const base = String(name).trim();
  for (let i = 2; ; i++) {
    const suffix = ` ${i}`;
    const stem = [...base]
      .slice(0, NAME_MAX - suffix.length)
      .join('')
      .trimEnd();
    const cand = stem + suffix;
    if (!conflict(cand, taken)) return cand;
  }
}

export const defaultChoice = (clash) => (clash ? 'both' : 'import');

// Use new ('replace') swaps in place; Use current and Skip leave the item out. Names stay unique without regard to case.
export function applyChoices(existing, incoming, picks) {
  const list = existing.map((x) => ({ ...x }));
  const out = { list, added: 0, replaced: 0, kept: 0, skipped: 0 };
  incoming.forEach((it, i) => {
    const p = picks[i] || { choice: 'import' };
    const taken = () => list.map((x) => x.name);
    if (p.choice === 'skip' || p.choice === 'current') {
      out.skipped++;
      return;
    }
    const at = list.findIndex((x) => same(x.name, it.name));
    if (p.choice === 'replace' && at >= 0) {
      list[at] = { ...it };
      out.replaced++;
      return;
    }
    let name = it.name;
    if (at >= 0 || p.choice === 'both') {
      name = at >= 0 ? (p.as && !conflict(p.as, taken()) ? p.as.trim() : renameFor(it.name, taken())) : it.name;
      if (at >= 0) out.kept++;
    }
    list.push({ ...it, name });
    out.added++;
  });
  return out;
}

export function renameProblem(name, taken) {
  const n = String(name).trim();
  if (!n) return 'Name required.';
  if (runes(n) > NAME_MAX) return `Names are at most ${NAME_MAX} characters.`;
  if (conflict(n, taken)) return `“${n}” is already in use.`;
  return '';
}

export const jsonBytes = (v) => new TextEncoder().encode(JSON.stringify(v)).length;

// Over the socket's read limit; a profile also over its own. '' when fine.
export function sizeProblem(scope, parts, what = 'import') {
  const kib = (b) => `${Math.ceil(b / 1024)} KiB`;
  if (scope === 'user') {
    const b = jsonBytes(parts);
    if (b > PROFILE_LIMIT)
      return `Too big for one save (${kib(b)}, a profile holds at most ${PROFILE_LIMIT / 1024} KiB): ${what} fewer filter sets or presets.`;
  }
  for (const [k, v] of Object.entries(parts)) {
    if (!Array.isArray(v)) continue;
    const b = jsonBytes({ t: 'save', id: 999999, [k]: v });
    if (b > SOCKET_LIMIT)
      return `Too big for one save (${kib(b)}, at most ${SOCKET_LIMIT / 1024} KiB): ${what} fewer ${k === 'presets' ? 'presets' : 'filter sets'}.`;
  }
  return '';
}

// "Name 2", or "Name (mine)" for a copy of an Everyone item; '' with nothing to copy. At most
// 32 characters.
export function saveAsName(name, fromScope = '') {
  const base = String(name || '').trim();
  if (!base) return '';
  const suffix = fromScope === 'global' ? ' (mine)' : ' 2';
  return (
    [...base]
      .slice(0, NAME_MAX - suffix.length)
      .join('')
      .trimEnd() + suffix
  );
}
