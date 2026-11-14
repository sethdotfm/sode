// Export documents, share links and import matching. Pure: no imports, no DOM, so Node tests it
// (dialogs: transfer.js, reviewsheet.js). Export documents carry numbers with names beside them
// so an import on another router can match by name. A share link holds ONE item as base64url
// JSON in the URL fragment (#share=), which never reaches the server or a proxy.

export const SOCKET_LIMIT = 64 * 1024;
const PROFILE_LIMIT = 48 * 1024;
export const NAME_MAX = 32;

export const PREF_LABELS = {
  theme: 'Theme',
  layout: 'Layout',
  clock: 'Clock',
  flip: 'Destination before Source',
  scrollTop: 'Scroll to top on focus',
  searchFilters: 'Search respects filters',
  setStack: 'Stacked filter sets',
  nextMode: 'Next means',
  compact: 'Compact view',
};

const FILTER_DEFAULT = { src: '', dst: '', sode: true, ext: true, hidden: false };
const FILTER_KEYS = Object.keys(FILTER_DEFAULT);

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

const isObj = (v) => !!v && typeof v === 'object' && !Array.isArray(v);
const isNum = (v) => Number.isInteger(v) && v >= 0;
const goodName = (v) => typeof v === 'string' && v.trim() !== '' && runes(v) <= NAME_MAX && !/[\p{Cc}]/u.test(v);

// Only shape and version: the saves check the content (config.Validate).
export function checkDoc(doc) {
  if (!isObj(doc)) return 'Not a Sode export: the file is not a mapping.';
  if (doc.sode == null) return 'Not a Sode export: it has no "sode" version.';
  if (doc.sode !== 1) return `Unsupported export version ${typeof doc.sode === 'object' ? '?' : doc.sode} (this Sode reads version 1).`;
  if (
    doc.exported != null &&
    (typeof doc.exported !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?(Z|[+-]\d\d:\d\d)$/.test(doc.exported) || isNaN(Date.parse(doc.exported)))
  ) {
    return '"exported" must be an RFC 3339 time.';
  }
  if (doc.prefs != null && !isObj(doc.prefs)) return '"prefs" must be a mapping.';
  for (const k of ['filterSets', 'presets']) {
    if (doc[k] != null && !Array.isArray(doc[k])) return `"${k}" must be a list.`;
  }
  for (const f of doc.filterSets || []) {
    if (!isObj(f)) return 'Every filter set must be a mapping.';
    if (!goodName(f.name)) return `A filter set needs a printable name of at most ${NAME_MAX} characters.`;
  }
  for (const p of doc.presets || []) {
    if (!isObj(p)) return 'Every preset must be a mapping.';
    if (!goodName(p.name)) return `A preset needs a printable name of at most ${NAME_MAX} characters.`;
    if (!Array.isArray(p.items)) return `Preset “${p.name}” needs a list of items.`;
    for (const it of p.items) {
      if (!isObj(it) || !isNum(it.src)) return `Preset “${p.name}”: every item needs a source number of 0 or more.`;
      if (!Array.isArray(it.dsts) || !it.dsts.every((d) => isObj(d) && isNum(d.n))) return `Preset “${p.name}”: every destination needs a number of 0 or more.`;
      if (it.levels != null && !(Array.isArray(it.levels) && it.levels.every((l) => Number.isInteger(l) && l >= 1)))
        return `Preset “${p.name}”: levels must be whole numbers from 1.`;
    }
  }
  return '';
}

const clean = (s) =>
  String(s)
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^A-Za-z0-9._-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^[-.]+|[-.]+$/g, '')
    .slice(0, 40);

const two = (n) => String(n).padStart(2, '0');
export const dateStamp = (d) => `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;

export function defaultFileName({ now = new Date(), site = '', user = '' } = {}) {
  const parts = [dateStamp(new Date(now)), 'Sode', 'Export', clean(site), clean(user)].filter(Boolean);
  return parts.join('-') + '.yaml';
}

export function fileNameFor(typed, dflt) {
  const base = clean(
    String(typed || '')
      .trim()
      .replace(/\.ya?ml$/i, ''),
  );
  return base ? `${base}.yaml` : dflt;
}

// ---- matching an imported item against this router

// Names compare ignoring case and outer spaces. Statuses, best to worst: same, byName (one live
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

// picks[i]: { choice, as } (as: name for Keep both, else automatic). Replace swaps in place;
// the rest append. Names stay unique whatever the case. Returns { list, added, replaced, kept,
// skipped }.
export function applyChoices(existing, incoming, picks) {
  const list = existing.map((x) => ({ ...x }));
  const out = { list, added: 0, replaced: 0, kept: 0, skipped: 0 };
  incoming.forEach((it, i) => {
    const p = picks[i] || { choice: 'import' };
    const taken = () => list.map((x) => x.name);
    if (p.choice === 'skip') {
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
  if (conflict(n, taken)) return `“${n}” is taken.`;
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

const b64 = (s) => {
  const bytes = new TextEncoder().encode(s);
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
};
const unb64 = (s) => new TextDecoder().decode(Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0)));

export function presetShare(p, srcName = () => '', dstName = () => '') {
  return { kind: 'preset', name: p.name, items: p.items.map((it) => itemOut(it, srcName, dstName)) };
}
export function filterShare(name, f) {
  const out = { kind: 'filter', name: String(name || '') };
  const n = filterOf(f);
  for (const k of FILTER_KEYS) if (n[k] !== FILTER_DEFAULT[k]) out[k] = n[k];
  return out;
}

export const encodeShare = (doc) => b64(JSON.stringify(doc));

export const shareUrl = (base, doc) => `${base}#share=${encodeShare(doc)}`;

export function decodeShare(text) {
  let d;
  try {
    d = JSON.parse(unb64(String(text)));
  } catch {
    throw new Error('The link is damaged.');
  }
  if (!d || typeof d !== 'object' || Array.isArray(d)) throw new Error('The link is damaged.');
  const name = String(d.name ?? '').trim();
  if (runes(name) > NAME_MAX) throw new Error('The link is damaged.');
  if (d.kind === 'filter') return { kind: 'filter', name, ...filterOf(d) };
  if (d.kind === 'preset') {
    if (!name) throw new Error('The preset in the link has no name.');
    if (!Array.isArray(d.items) || !d.items.length || d.items.length > 100) throw new Error('The preset in the link has no takes.');
    return { kind: 'preset', name, items: d.items.map(itemIn) };
  }
  throw new Error('The link is not a Sode share link.');
}

export function decodeLegacy(text) {
  let d;
  try {
    d = JSON.parse(unb64(String(text)));
  } catch {
    throw new Error('The link is damaged.');
  }
  if (!d || typeof d !== 'object' || Array.isArray(d)) throw new Error('The link is damaged.');
  return {
    kind: 'filter',
    name: String(d.name ?? '')
      .trim()
      .slice(0, NAME_MAX),
    ...filterOf(d),
  };
}

// Returns { doc, error, search, hash } (doc null and error '' when there is no link);
// search/hash: the parts to put back with our parameters removed.
export function readLink(search, hash) {
  const out = { doc: null, error: '', search, hash };
  const h = new URLSearchParams(String(hash).replace(/^#/, ''));
  const s = new URLSearchParams(search);
  const share = h.get('share');
  const legacy = s.get('f');
  if (share == null && legacy == null) return out;
  try {
    out.doc = share != null ? decodeShare(share) : decodeLegacy(legacy);
  } catch (e) {
    out.error = e.message;
  }
  if (share != null) {
    h.delete('share');
    const t = h.toString();
    out.hash = t ? `#${t}` : '';
  }
  if (legacy != null) {
    s.delete('f');
    const t = s.toString();
    out.search = t ? `?${t}` : '';
  }
  return out;
}

// Only once the router's names are in, the lock state is known (`L` from lock.js's `access`;
// null until then) and the page is not covered (PIN lock or hand lock). A link waits, in the
// address, until then.
export const mayOpenSheet = (L, pageLocked = false, namesLoaded = true) =>
  !!L && namesLoaded && !(L.locked && (L.enforced || L.held)) && !(pageLocked && !L.enforced);

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

export function docOfShare(d) {
  if (d.kind === 'preset') return { sode: 1, presets: [{ name: d.name, items: d.items }] };
  return { sode: 1, filterSets: [{ name: d.name || 'Shared filter', ...filterOf(d) }] };
}
