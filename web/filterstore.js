// Recent filters, per browser (edited in filters.js), and the named filter
// sets: which kinds of entry show, and filter expressions (filter.js) for
// sources and destinations. hidden shows changes made elsewhere to
// Settings' globally hidden destinations.

import { S, on, onMsg, emit, off, srcItem, dstItem } from './core.js';
import { filterFn } from './filter.js';
import { readJSON, writeJSON, readStr, writeStr } from './store.js';

const FILTER_KEY = 'sode-filter';
export const FILTER_DEFAULT = Object.freeze({ src: '', dst: '', sode: true, ext: true, hidden: false });
export const normFilter = (f) => ({
  src: String(f?.src ?? ''), dst: String(f?.dst ?? ''),
  sode: f?.sode !== false, ext: f?.ext !== false, hidden: f?.hidden === true,
});

function loadFilter() {
  const f = readJSON(FILTER_KEY, {}) || {};
  // The first beta's one setting: 'none' or 'all' other panels.
  const old = readStr('sode-ext');
  if (old) {
    if (old === 'none') f.ext = false;
    if (old === 'all') f.hidden = true;
    writeStr('sode-ext', null);
    writeJSON(FILTER_KEY, f);
  }
  return normFilter(f);
}

const anything = () => true;
let filter = loadFilter();
// Matchers for the expressions; one that doesn't parse keeps the last good one.
let srcOk = filterFn(filter.src) || anything;
let dstOk = filterFn(filter.dst) || anything;

export const filterState = () => ({ ...filter });

// How many parts of the filter differ from the default, for the button.
export const filterCount = () => ['src', 'dst'].filter((k) => filter[k].trim()).length +
  ['sode', 'ext', 'hidden'].filter((k) => filter[k] !== FILTER_DEFAULT[k]).length;

export function setFilter(next) {
  filter = normFilter(next);
  srcOk = filterFn(filter.src) || srcOk;
  dstOk = filterFn(filter.dst) || dstOk;
  writeJSON(FILTER_KEY, filterCount() ? filter : null);
  emit('filters');
  emit('history');
}

// ---- named filter sets: the built-in All, global ones from Settings
// (cfg.filterSets) and this browser's own. The active set is remembered
// with the filter it had when picked (base), so a global set changed in
// Settings carries browsers along, unless they've edited it since.

const SETS_KEY = 'sode-filter-sets';
const ACTIVE_KEY = 'sode-filter-active';

export const sameFilter = (a, b) => Object.keys(FILTER_DEFAULT)
  .every((k) => (typeof a[k] === 'string' ? a[k].trim() === b[k].trim() : a[k] === b[k]));
export const globalSets = () => (S.cfg?.filterSets || []).map((s) => ({ scope: 'global', name: s.name, f: normFilter(s) }));
export const localSets = () => [].concat(readJSON(SETS_KEY, [])).filter((s) => s?.name)
  .map((s) => ({ scope: 'local', name: String(s.name), f: normFilter(s.f) }));
export const allSets = () => [{ scope: 'all', name: 'All', f: normFilter(FILTER_DEFAULT) }, ...globalSets(), ...localSets()];
// Names are unique per scope whatever the case, so a set renamed only in
// case is still the same set.
const findSet = (scope, name) => allSets().find((s) => s.scope === scope && s.name.toLowerCase() === name.toLowerCase());
const saveLocals = (list) => writeJSON(SETS_KEY, list.length ? list.map(({ name, f }) => ({ name, f })) : null);

let active = readJSON(ACTIVE_KEY, null); // { scope, name, base }

// The active set, with edited = the filter differs from it now; or null.
export function activeSet() {
  const s = active && findSet(active.scope, active.name);
  return s ? { ...s, edited: !sameFilter(filter, s.f) } : null;
}

// The active set as stored, and a filter to use with it (null: no set),
// for putting things back as they were.
export const activeRef = () => active && { ...active };
export function adoptFilter(f, ref = null) {
  active = ref;
  writeJSON(ACTIVE_KEY, active);
  setFilter(f);
}

export function pickSet(scope, name) {
  const s = findSet(scope, name);
  if (!s) return;
  active = { scope, name, base: s.f };
  writeJSON(ACTIVE_KEY, active);
  setFilter(s.f);
}

// The filter now, as this browser's set `name` (replacing one of that name,
// any case), which becomes the active one.
export function saveLocalSet(name) {
  const list = localSets().filter((s) => s.name.toLowerCase() !== name.toLowerCase());
  list.push({ scope: 'local', name, f: filterState() });
  saveLocals(list);
  active = { scope: 'local', name, base: filterState() };
  writeJSON(ACTIVE_KEY, active);
  emit('filters');
}

export function renameLocalSet(old, name) {
  saveLocals(localSets().map((s) => (s.name === old ? { ...s, name } : s)));
  if (active?.scope === 'local' && active.name === old) {
    active = { ...active, name };
    writeJSON(ACTIVE_KEY, active);
  }
  emit('filters');
}

export function deleteLocalSet(name) {
  saveLocals(localSets().filter((s) => s.name.toLowerCase() !== name.toLowerCase()));
  if (active?.scope === 'local' && active.name.toLowerCase() === name.toLowerCase()) {
    active = null;
    writeJSON(ACTIVE_KEY, null);
  }
  emit('filters');
}

// After a hello: follow a global set that changed in Settings, unless this
// browser changed the filter since picking it.
function followGlobal() {
  const s = active?.scope === 'global' && findSet('global', active.name);
  if (!s || sameFilter(s.f, active.base) || !sameFilter(filter, active.base)) return;
  active.base = s.f;
  active.name = s.name;
  writeJSON(ACTIVE_KEY, active);
  setFilter(s.f);
}

// "Dst: MV, !20-60 · No Sode takes", for titles and lists.
export function filterSummary(f) {
  const parts = [];
  if (f.src.trim()) parts.push(`Src: ${f.src.trim()}`);
  if (f.dst.trim()) parts.push(`Dst: ${f.dst.trim()}`);
  if (!f.sode) parts.push('No Sode takes');
  if (!f.ext) parts.push('No other panels');
  else if (f.hidden) parts.push('With hidden');
  return parts.join(' · ') || 'Everything';
}

// The globally hidden destinations, compiled once per expression. Blank
// (or one that doesn't parse) hides nothing.
let hiddenText = null, hiddenFn = null;
export function globallyHidden(num, name) {
  const t = (S.cfg?.hiddenDsts || '').trim();
  if (t !== hiddenText) { hiddenText = t; hiddenFn = t ? filterFn(t) : null; }
  return !!hiddenFn && hiddenFn(num, name);
}

// Whether this browser's filter (kinds, hidden destinations, Src and Dst
// expressions) lets a history entry show. Quick search is history.js's.
export function filterAllows(e) {
  if (e.ext ? !filter.ext : !filter.sode) return false;
  const s = srcItem(e.src, e.srcName), d = dstItem(e.dst, e.dstName);
  const sn = e.src + off(), dn = e.dst + off();
  if (e.ext && !filter.hidden && globallyHidden(dn, d.name)) return false;
  return srcOk(sn, s.name) && dstOk(dn, d.name);
}

on('helloData', followGlobal);
// The sets for everyone changed: just those, not a fresh snapshot.
onMsg('filterSets', (m) => {
  S.cfg = { ...S.cfg, filterSets: m.sets };
  followGlobal();
  emit('filters');
  emit('history');
});
