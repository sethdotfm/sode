// Recent filters, per browser (edited in filters.js), and the named filter
// sets: which kinds of entry show, and filter expressions (filter.js) for
// sources and destinations. hidden shows changes made elsewhere to
// Settings' globally hidden destinations.

import { S, on, onMsg, emit, off, srcItem, dstItem } from './core.js';
import { filterFn, entryAllows, stackAllows } from './filter.js';
import { pref } from './device.js';
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

// Puts a filter to work, whatever the sets say.
function put(next) {
  filter = normFilter(next);
  srcOk = filterFn(filter.src) || srcOk;
  dstOk = filterFn(filter.dst) || dstOk;
  writeJSON(FILTER_KEY, filterCount() ? filter : null);
  emit('filters');
  emit('history');
}

// Any change to the filter itself leaves a stack of sets: what's left is
// this filter, in no set. Only pickSet, toggleSet and adoptFilter set the
// list, so every editor, checkbox and share link is covered here.
export function setFilter(next) {
  if (active.length > 1) setActive([]);
  put(next);
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

// The active sets, each { scope, name, base }. Empty: no set (the filter
// is unsaved). One: that set, edited or not. Two or more: a stack, whose
// `filter` is the first one's; filterAllows combines them (stackAllows).
// Older versions kept just one object.
export const normActive = (raw) => [].concat(raw ?? [])
  .filter((r) => r && typeof r.scope === 'string' && typeof r.name === 'string')
  .map((r) => ({ scope: r.scope, name: r.name, base: normFilter(r.base) }));
let active = normActive(readJSON(ACTIVE_KEY, null));

// The stack's compiled matchers, built when first needed and dropped
// whenever the list or the sets might have changed.
let stackM = null;
function setActive(list) {
  active = list;
  writeJSON(ACTIVE_KEY, active.length ? active : null);
  stackM = null;
}

// The active set when there is just one, with edited = the filter differs
// from it now; or null.
export function activeSet() {
  const s = active.length === 1 && findSet(active[0].scope, active[0].name);
  return s ? { ...s, edited: !sameFilter(filter, s.f) } : null;
}

// The sets of a stack (two or more), or [].
export function activeStack() {
  if (active.length < 2) return [];
  return active.map((r) => findSet(r.scope, r.name)).filter(Boolean);
}
export const isActive = (scope, name) => active.some((r) => r.scope === scope && r.name.toLowerCase() === name.toLowerCase());

// The active set as stored (activeRefs: all of them), for putting things
// back as they were with adoptFilter.
export const activeRef = () => (active.length === 1 ? { ...active[0] } : null);
export const activeRefs = () => active.map((r) => ({ ...r }));
export function adoptFilter(f, ref = null) {
  setActive(normActive(ref));
  put(f);
}

export function pickSet(scope, name) {
  const s = findSet(scope, name);
  if (!s) return;
  setActive([{ scope, name, base: s.f }]);
  put(s.f);
}

// Where a list of sets leaves the filter: none is All, one is that set, a
// stack starts from its first.
function useSets(list) {
  const sets = list.map((r) => findSet(r.scope, r.name));
  if (!list.length || sets.some((s) => !s)) return pickSet('all', 'All');
  setActive(list);
  put(sets[0].f);
}

// A chip click: All alone, or a set in or out of the stack. Adding to a
// single set that was edited drops the edit; none left is All.
export function toggleSet(scope, name) {
  const s = findSet(scope, name);
  if (!s) return;
  if (s.scope === 'all') return pickSet('all', 'All');
  const named = active.filter((r) => r.scope !== 'all');
  if (isActive(s.scope, s.name)) {
    return useSets(named.filter((r) => !(r.scope === s.scope && r.name.toLowerCase() === s.name.toLowerCase())));
  }
  useSets([...named, { scope: s.scope, name: s.name, base: s.f }]);
}

// The stack's matchers, one per set; null unless there are two or more.
function stackMatchers() {
  if (!stackM) {
    stackM = activeStack().map((s) => ({
      f: s.f, srcOk: filterFn(s.f.src) || anything, dstOk: filterFn(s.f.dst) || anything,
    }));
  }
  return stackM.length > 1 ? stackM : null;
}

// The filter now, as this browser's set `name` (replacing one of that name,
// any case), which becomes the active one.
export function saveLocalSet(name) {
  const list = localSets().filter((s) => s.name.toLowerCase() !== name.toLowerCase());
  list.push({ scope: 'local', name, f: filterState() });
  saveLocals(list);
  setActive([{ scope: 'local', name, base: filterState() }]);
  emit('filters');
}

export function renameLocalSet(old, name) {
  saveLocals(localSets().map((s) => (s.name === old ? { ...s, name } : s)));
  setActive(active.map((r) => (r.scope === 'local' && r.name === old ? { ...r, name } : r)));
  emit('filters');
}

export function deleteLocalSet(name) {
  saveLocals(localSets().filter((s) => s.name.toLowerCase() !== name.toLowerCase()));
  const gone = (r) => r.scope === 'local' && r.name.toLowerCase() === name.toLowerCase();
  if (active.length > 1 && active.some(gone)) useSets(active.filter((r) => !gone(r)));
  else if (active.some(gone)) setActive([]);
  stackM = null;
  emit('filters');
}

// After a hello: follow a global set that changed in Settings, unless this
// browser changed the filter since picking it.
function followGlobal() {
  stackM = null;
  if (active.length > 1) {
    // A stack can't have been edited: global sets gone are dropped, the
    // rest follow what changed.
    const list = active.filter((r) => r.scope !== 'global' || findSet('global', r.name)).map((r) => {
      const s = r.scope === 'global' && findSet('global', r.name);
      return s ? { scope: 'global', name: s.name, base: s.f } : r;
    });
    if (JSON.stringify(list) !== JSON.stringify(active)) useSets(list);
    return;
  }
  const r = active[0];
  const s = r?.scope === 'global' && findSet('global', r.name);
  if (!s || sameFilter(s.f, r.base) || !sameFilter(filter, r.base)) return;
  setActive([{ scope: 'global', name: s.name, base: s.f }]);
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
// With a stack of sets, each one's saved filter judges it and pref
// setStack says whether all must allow or any may.
export function filterAllows(e) {
  const s = srcItem(e.src, e.srcName), d = dstItem(e.dst, e.dstName);
  const dn = e.dst + off();
  const x = { ext: !!e.ext, sn: e.src + off(), sname: s.name, dn, dname: d.name, hiddenDst: !!e.ext && globallyHidden(dn, d.name) };
  const st = stackMatchers();
  return st ? stackAllows(pref('setStack'), st, x) : entryAllows(filter, { srcOk, dstOk }, x);
}

on('helloData', followGlobal);
// The sets for everyone changed: just those, not a fresh snapshot.
onMsg('filterSets', (m) => {
  S.cfg = { ...S.cfg, filterSets: m.sets };
  followGlobal();
  emit('filters');
  emit('history');
});
