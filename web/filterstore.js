// Recent filters, per browser (edited in filters.js), and the named filter sets. hidden shows
// changes made elsewhere to Settings' globally hidden destinations.

import { S, on, onMsg, emit, off, srcItem, dstItem } from './core.js';
import { filterFn, entryAllows, stackAllows } from './filter.js';
import { pref } from './device.js';
import { profile, profileName } from './profile.js';
import { readJSON, writeJSON, readStr, writeStr } from './store.js';

const FILTER_KEY = 'sode-filter';
export const FILTER_DEFAULT = Object.freeze({ src: '', dst: '', sode: true, ext: true, hidden: false });
export const normFilter = (f) => ({
  src: String(f?.src ?? ''),
  dst: String(f?.dst ?? ''),
  sode: f?.sode !== false,
  ext: f?.ext !== false,
  hidden: f?.hidden === true,
});

function loadFilter() {
  const f = readJSON(FILTER_KEY, {}) || {};
  // Old first-beta setting: 'none' or 'all' other panels.
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
let srcOk = filterFn(filter.src) || anything;
let dstOk = filterFn(filter.dst) || anything;

export const filterState = () => ({ ...filter });

export const filterCount = () =>
  ['src', 'dst'].filter((k) => filter[k].trim()).length + ['sode', 'ext', 'hidden'].filter((k) => filter[k] !== FILTER_DEFAULT[k]).length;

function applyFilter(next) {
  filter = normFilter(next);
  srcOk = filterFn(filter.src) || srcOk;
  dstOk = filterFn(filter.dst) || dstOk;
  writeJSON(FILTER_KEY, filterCount() ? filter : null);
  emit('filters');
  emit('history');
}

// Every edit of the filter goes in here: it leaves a stack of sets; a single set stays active,
// shown as edited.
export function setFilter(next) {
  if (stack.length) setActive(null);
  applyFilter(next);
}

// ---- named sets: built-in All, global (cfg.filterSets) and this browser's own. The active set is remembered with the filter it had when picked (base), so a global set changed in Settings carries browsers along unless they edited it since.

const SETS_KEY = 'sode-filter-sets';
const ACTIVE_KEY = 'sode-filter-active';

const sameFilter = (a, b) => Object.keys(FILTER_DEFAULT).every((k) => (typeof a[k] === 'string' ? a[k].trim() === b[k].trim() : a[k] === b[k]));
export const globalSets = () => (S.cfg?.filterSets || []).map((s) => ({ scope: 'global', name: s.name, f: normFilter(s) }));
export const localSets = () =>
  []
    .concat(readJSON(SETS_KEY, []))
    .filter((s) => s?.name)
    .map((s) => ({ scope: 'local', name: String(s.name), f: normFilter(s.f) }));
export const userSets = () =>
  (profileName() ? profile().filterSets : [])
    .filter((s) => s?.name)
    .map((s) => ({ scope: 'user', user: profileName(), name: String(s.name), f: normFilter(s) }));
export const allSets = () => [{ scope: 'all', name: 'All', f: normFilter(FILTER_DEFAULT) }, ...globalSets(), ...userSets(), ...localSets()];
const SET_LISTS = { global: globalSets, user: userSets, local: localSets };
export const setsIn = (scope) => (SET_LISTS[scope] || (() => []))();
// A user's set carries the user, so another user logging in never picks up a same-named set of
// theirs.
export const setRef = (scope, name) => (scope === 'user' ? { scope, user: profileName(), name } : { scope, name });
// Names are unique per scope whatever the case, so a set renamed only in case is the same set.
const findSet = (scope, name, user) =>
  allSets().find((s) => s.scope === scope && s.name.toLowerCase() === name.toLowerCase() && (scope !== 'user' || s.user === user));
const saveLocals = (list) => writeJSON(SETS_KEY, list.length ? list.map(({ name, f }) => ({ name, f })) : null);

// The active set { scope, name, base }, or null (unsaved); or a stack of two or more [{ scope,
// name }] (none `active`): `filter` is then the first one's, and filterAllows combines them via
// stackAllows. The stack has its own key, since it can't be edited.
const STACK_KEY = 'sode-filter-stack';
const isRef = (r) => r && typeof r.scope === 'string' && typeof r.name === 'string';
const sameRef = (a, b) => a.scope === b.scope && a.name.toLowerCase() === b.name.toLowerCase() && (a.user ?? '') === (b.user ?? '');
const copyRef = ({ scope, name, user }) => (scope === 'user' ? { scope, user: String(user ?? ''), name } : { scope, name });
const loadStack = () =>
  []
    .concat(readJSON(STACK_KEY, null) ?? [])
    .filter(isRef)
    .map(copyRef);
let active = null,
  stack = [];
{
  // Old first-beta list under the active key.
  const stored = [].concat(readJSON(ACTIVE_KEY, null) ?? []).filter(isRef);
  stack = stored.length > 1 ? stored.map(copyRef) : loadStack();
  if (stack.length < 2) stack = [];
  if (!stack.length && stored.length) active = { ...copyRef(stored[0]), base: normFilter(stored[0].base) };
}

// Compiled matchers, built on first use after the list changes. setActive is the only thing
// that changes it.
let stackM = null;
function setActive(a, refs = []) {
  active = a;
  stack = refs;
  stackM = null;
  writeJSON(ACTIVE_KEY, active);
  writeJSON(STACK_KEY, stack.length ? stack : null);
}

export function activeSet() {
  const s = active && findSet(active.scope, active.name, active.user);
  return s ? { ...s, edited: !sameFilter(filter, s.f) } : null;
}

export const activeStack = () => stack.map((r) => findSet(r.scope, r.name, r.user)).filter(Boolean);
export const isActive = (scope, name) => [active, ...stack].some((r) => r && sameRef(r, setRef(scope, name)));

export const activeRef = () => active && { ...active };
export function adoptFilter(f, ref = null) {
  setActive(ref);
  applyFilter(f);
}

export const snapshot = () => ({ filter: filterState(), active: activeRef(), stack: stack.map((r) => ({ ...r })) });
export function restore(snap) {
  setActive(snap.active, snap.stack);
  applyFilter(snap.filter);
}

export function pickSet(scope, name) {
  const s = findSet(scope, name, profileName());
  if (!s) return;
  setActive({ ...setRef(scope, name), base: s.f });
  applyFilter(s.f);
}

function useSets(refs) {
  const sets = refs.map((r) => findSet(r.scope, r.name, r.user)).filter(Boolean);
  if (sets.length < 2) return sets.length ? pickSet(sets[0].scope, sets[0].name) : pickSet('all', 'All');
  setActive(
    null,
    sets.map(({ scope, name, user }) => copyRef({ scope, name, user })),
  );
  applyFilter(sets[0].f);
}

export function toggleSet(scope, name) {
  const s = findSet(scope, name, profileName());
  if (!s) return;
  if (s.scope === 'all') return pickSet('all', 'All');
  const now = stack.length ? stack : active && active.scope !== 'all' ? [active] : [];
  useSets(isActive(s.scope, s.name) ? now.filter((r) => !sameRef(r, setRef(s.scope, s.name))) : [...now, setRef(s.scope, s.name)]);
}

function stackMatchers() {
  if (!stackM) {
    stackM = activeStack().map((s) => ({
      f: s.f,
      srcOk: filterFn(s.f.src) || anything,
      dstOk: filterFn(s.f.dst) || anything,
    }));
  }
  return stackM.length > 1 ? stackM : null;
}

export function saveLocalSet(name) {
  const list = localSets().filter((s) => s.name.toLowerCase() !== name.toLowerCase());
  list.push({ scope: 'local', name, f: filterState() });
  saveLocals(list);
  setActive({ scope: 'local', name, base: filterState() });
  emit('filters');
}

export function putLocalSets(list) {
  saveLocals(list);
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

// Follow a global set that changed in Settings unless this browser changed the filter since. A
// stack can't have been edited, so it follows (and loses sets that are gone).
function followGlobal() {
  if (stack.length) return useSets(stack);
  const s = active?.scope === 'global' && findSet('global', active.name);
  if (!s || sameFilter(s.f, active.base) || !sameFilter(filter, active.base)) return;
  setActive({ scope: 'global', name: s.name, base: s.f });
  setFilter(s.f);
}

// The same for the user's sets, edited on another device. A set of another user can't be used
// here: it drops out for good. `was` is who the last profile was, so a page loading before its
// profile arrives keeps what it stored.
let was = '';
function followProfile() {
  const now = profileName();
  const left = was !== now && was !== '';
  was = now;
  const mine = (r) => r.scope === 'user' && r.user === now;
  if (left && (stack.some((r) => r.scope === 'user') || active?.scope === 'user')) {
    if (stack.length) return useSets(stack.filter((r) => r.scope !== 'user' || mine(r)));
    if (!mine(active)) {
      pickSet('all', 'All');
      return;
    }
  }
  if (stack.length) return useSets(stack);
  const s = active?.scope === 'user' && mine(active) && findSet('user', active.name, now);
  if (!s || sameFilter(s.f, active.base) || !sameFilter(filter, active.base)) return;
  setActive({ ...setRef('user', s.name), base: s.f });
  setFilter(s.f);
}

export function filterSummary(f) {
  const parts = [];
  if (f.src.trim()) parts.push(`Src: ${f.src.trim()}`);
  if (f.dst.trim()) parts.push(`Dst: ${f.dst.trim()}`);
  if (!f.sode) parts.push('No Sode takes');
  if (!f.ext) parts.push('No Externals');
  else if (f.hidden) parts.push('With hidden');
  return parts.join(', ') || 'Everything';
}

// Compiled once per expression. Blank (or unparsable) hides nothing.
let hiddenText = null,
  hiddenFn = null;
function globallyHidden(num, name) {
  const t = (S.cfg?.hiddenDsts || '').trim();
  if (t !== hiddenText) {
    hiddenText = t;
    hiddenFn = t ? filterFn(t) : null;
  }
  return !!hiddenFn && hiddenFn(num, name);
}

// Quick search is history.js's. With a stack, each set's saved filter judges it and pref
// setStack says whether all must allow or any may.
export function filterAllows(e) {
  const s = srcItem(e.src, e.srcName),
    d = dstItem(e.dst, e.dstName);
  const dn = e.dst + off();
  const x = { ext: !!e.ext, sn: e.src + off(), sname: s.name, dn, dname: d.name, hiddenDst: !!e.ext && globallyHidden(dn, d.name) };
  const st = stackMatchers();
  return st ? stackAllows(pref('setStack'), st, x) : entryAllows(filter, { srcOk, dstOk }, x);
}

on('helloData', followGlobal);
on('profile', () => {
  followProfile();
  emit('filters');
  emit('history');
});
onMsg('filterSets', (m) => {
  S.cfg = { ...S.cfg, filterSets: m.sets };
  followGlobal();
  emit('filters');
  emit('history');
});
