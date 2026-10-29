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
function applyFilter(next) {
  filter = normFilter(next);
  srcOk = filterFn(filter.src) || srcOk;
  dstOk = filterFn(filter.dst) || dstOk;
  writeJSON(FILTER_KEY, filterCount() ? filter : null);
  emit('filters');
  emit('history');
}

// The way every edit of the filter goes in (editors, checkboxes, quick
// search boxes, share links): it leaves a stack of sets, so what's left is
// this filter in no set. A single set stays active, shown as edited.
export function setFilter(next) {
  if (stack.length) setActive(null);
  applyFilter(next);
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

// The active set { scope, name, base }, or null (the filter is unsaved). Or
// a stack of two or more sets, [{ scope, name }], none of them `active`;
// `filter` is then the first one's, and filterAllows combines them with
// stackAllows. The stack lives in its own key, since it can't be edited.
const STACK_KEY = 'sode-filter-stack';
const isRef = (r) => r && typeof r.scope === 'string' && typeof r.name === 'string';
const sameRef = (a, b) => a.scope === b.scope && a.name.toLowerCase() === b.name.toLowerCase();
const loadStack = () => [].concat(readJSON(STACK_KEY, null) ?? []).filter(isRef).map(({ scope, name }) => ({ scope, name }));
let active = null, stack = [];
{
  // The first beta kept a list under the active key.
  const stored = [].concat(readJSON(ACTIVE_KEY, null) ?? []).filter(isRef);
  stack = stored.length > 1 ? stored.map(({ scope, name }) => ({ scope, name })) : loadStack();
  if (stack.length < 2) stack = [];
  if (!stack.length && stored.length) active = { scope: stored[0].scope, name: stored[0].name, base: normFilter(stored[0].base) };
}

// The stack's sets with their compiled matchers, built on first use after
// the list changes. setActive is the only thing that changes it.
let stackM = null;
function setActive(a, refs = []) {
  active = a;
  stack = refs;
  stackM = null;
  writeJSON(ACTIVE_KEY, active);
  writeJSON(STACK_KEY, stack.length ? stack : null);
}

// The active set, with edited = the filter differs from it now; or null.
export function activeSet() {
  const s = active && findSet(active.scope, active.name);
  return s ? { ...s, edited: !sameFilter(filter, s.f) } : null;
}

// The sets of a stack, or [].
export const activeStack = () => stack.map((r) => findSet(r.scope, r.name)).filter(Boolean);
export const isActive = (scope, name) => [active, ...stack].some((r) => r && sameRef(r, { scope, name }));

// The active set as stored, and a filter to use with it (null: no set),
// for putting things back as they were.
export const activeRef = () => active && { ...active };
export function adoptFilter(f, ref = null) {
  setActive(ref);
  applyFilter(f);
}

// The set and stack as they are now, for restore.
export const snapshot = () => ({ filter: filterState(), active: activeRef(), stack: stack.map((r) => ({ ...r })) });
export function restore(snap) {
  setActive(snap.active, snap.stack);
  applyFilter(snap.filter);
}

export function pickSet(scope, name) {
  const s = findSet(scope, name);
  if (!s) return;
  setActive({ scope, name, base: s.f });
  applyFilter(s.f);
}

// Where some sets leave the filter: those that still exist, none being All,
// one being that set, more a stack starting from the first.
function useSets(refs) {
  const sets = refs.map((r) => findSet(r.scope, r.name)).filter(Boolean);
  if (sets.length < 2) return sets.length ? pickSet(sets[0].scope, sets[0].name) : pickSet('all', 'All');
  setActive(null, sets.map(({ scope, name }) => ({ scope, name })));
  applyFilter(sets[0].f);
}

// A chip click: All alone, or a set in or out of those active. Adding to a
// single set that was edited drops the edit; none left is All.
export function toggleSet(scope, name) {
  const s = findSet(scope, name);
  if (!s) return;
  if (s.scope === 'all') return pickSet('all', 'All');
  const now = stack.length ? stack : active && active.scope !== 'all' ? [active] : [];
  useSets(isActive(s.scope, s.name) ? now.filter((r) => !sameRef(r, s)) : [...now, s]);
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
  setActive({ scope: 'local', name, base: filterState() });
  emit('filters');
}

export function renameLocalSet(old, name) {
  saveLocals(localSets().map((s) => (s.name === old ? { ...s, name } : s)));
  const ren = (r) => (r.scope === 'local' && r.name === old ? { ...r, name } : r);
  setActive(active && ren(active), stack.map(ren));
  emit('filters');
}

export function deleteLocalSet(name) {
  saveLocals(localSets().filter((s) => s.name.toLowerCase() !== name.toLowerCase()));
  if (stack.length) useSets(stack);
  else if (active?.scope === 'local' && active.name.toLowerCase() === name.toLowerCase()) setActive(null);
  emit('filters');
}

// After a hello: follow a global set that changed in Settings, unless this
// browser changed the filter since picking it. A stack can't have been
// edited, so it simply follows (and loses sets that are gone).
function followGlobal() {
  if (stack.length) return useSets(stack);
  const s = active?.scope === 'global' && findSet('global', active.name);
  if (!s || sameFilter(s.f, active.base) || !sameFilter(filter, active.base)) return;
  setActive({ scope: 'global', name: s.name, base: s.f });
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
