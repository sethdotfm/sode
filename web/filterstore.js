// History's filter, per browser (filters.js edits it), and the named filter sets. hidden: show Externals on the destinations that Settings hides.

import { S, on, onMsg, emit, off, srcItem, dstItem } from './core.js';
import { filterFn, entryAllows, stackAllows } from './filter.js';
import { pref } from './device.js';
import { profile, profileName } from './profile.js';
import { readJSON, writeJSON, readStr, writeStr } from './store.js';
import { newPresetId } from './presetref.js';
import { typedEnds } from './tiehistoryrules.js';

const FILTER_KEY = 'sode-filter';
export const FILTER_DEFAULT = Object.freeze({ src: '', dst: '', sode: true, ext: true, hidden: false });
// router: the UID whose numbers the number and range terms mean.
export const normFilter = (f) => ({
  ...(f?.router && typeof f.router === 'string' ? { router: f.router } : {}),
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
let srcOk = filterFn(filter.src, { router: filter.router }) || anything;
let dstOk = filterFn(filter.dst, { router: filter.router }) || anything;

export const filterState = () => ({ ...filter });

export const filterCount = () =>
  ['src', 'dst'].filter((k) => filter[k].trim()).length + ['sode', 'ext', 'hidden'].filter((k) => filter[k] !== FILTER_DEFAULT[k]).length;

function applyFilter(next) {
  filter = normFilter(next);
  srcOk = filterFn(filter.src, { router: filter.router }) || srcOk;
  dstOk = filterFn(filter.dst, { router: filter.router }) || dstOk;
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

// Named sets: the built-in All, global (cfg.filterSets) and this browser's own. The active set keeps the filter it had when picked (base).
// So a global set that changes in Settings moves each browser that did not edit it.
// Each set has a hidden id, as a preset has (config.FilterSet). The active set, a stack and bookmarks name a set by
// it, so a rename moves nothing. The server makes the id of an Everyone or Me set sent without one.

const SETS_KEY = 'sode-filter-sets';
const ACTIVE_KEY = 'sode-filter-active';
const ALL_ID = 'All';

const sameFilter = (a, b) => Object.keys(FILTER_DEFAULT).every((k) => (typeof a[k] === 'string' ? a[k].trim() === b[k].trim() : a[k] === b[k]));
// broken: the server's marks (brokenrules.js), kept out of f so a save never sends them.
const marks = (s) => (Array.isArray(s?.broken) && s.broken.length ? { broken: s.broken } : {});
const idOf = (s) => (typeof s?.id === 'string' ? s.id.trim() : '');
export const globalSets = () => (S.cfg?.filterSets || []).map((s) => ({ scope: 'global', id: idOf(s), name: s.name, f: normFilter(s), ...marks(s) }));
export const localSets = () =>
  []
    .concat(readJSON(SETS_KEY, []))
    .filter((s) => s?.name)
    .map((s) => ({ scope: 'local', id: idOf(s), name: String(s.name), f: normFilter(s.f) }));
export const userSets = () =>
  (profileName() ? profile().filterSets : [])
    .filter((s) => s?.name)
    .map((s) => ({ scope: 'user', user: profileName(), id: idOf(s), name: String(s.name), f: normFilter(s), ...marks(s) }));
export const allSets = () => [{ scope: 'all', id: ALL_ID, name: 'All', f: normFilter(FILTER_DEFAULT) }, ...globalSets(), ...userSets(), ...localSets()];
const SET_LISTS = { global: globalSets, user: userSets, local: localSets };
export const setsIn = (scope) => (SET_LISTS[scope] || (() => []))();
// A user's set carries the user, so another user who signs in never picks up a set of theirs.
export const setRef = (scope, id) => (scope === 'user' ? { scope, user: profileName(), id } : { scope, id });
const findSet = (scope, id, user) => allSets().find((s) => s.scope === scope && s.id === id && (scope !== 'user' || s.user === user));
const sameName = (a, b) => a.toLowerCase() === b.toLowerCase();
// As the server's WithFilterSetExtras: a set without an id gets the id of this browser's set of its name, else a new
// one. So a Replace import keeps the bookmarks on it.
function saveLocals(list) {
  const was = localSets();
  const taken = new Set();
  const out = list.map(({ id, name, f }) => {
    let key = id && !taken.has(id) ? id : '';
    if (!key) key = was.find((x) => sameName(x.name, String(name).trim()) && x.id && !taken.has(x.id))?.id || newPresetId();
    taken.add(key);
    return { id: key, name, f };
  });
  writeJSON(SETS_KEY, out.length ? out : null);
}

// The active set { scope, id, base }, or null (unsaved). Or a stack of two or more [{ scope,
// id }] (none `active`): `filter` is then the first one's, and filterAllows combines them with
// stackAllows. The stack has its own key, since it cannot be edited.
const STACK_KEY = 'sode-filter-stack';
const isRef = (r) => r && typeof r.scope === 'string' && typeof r.id === 'string';
const sameRef = (a, b) => a.scope === b.scope && a.id === b.id && (a.user ?? '') === (b.user ?? '');
const copyRef = ({ scope, id, user }) => (scope === 'user' ? { scope, user: String(user ?? ''), id } : { scope, id });
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
  const s = active && findSet(active.scope, active.id, active.user);
  return s ? { ...s, edited: !sameFilter(filter, s.f) } : null;
}

export const activeStack = () => stack.map((r) => findSet(r.scope, r.id, r.user)).filter(Boolean);
export const isActive = (scope, id) => [active, ...stack].some((r) => r && sameRef(r, setRef(scope, id)));

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

export function pickSet(scope, id) {
  const s = findSet(scope, id, profileName());
  if (!s) return;
  setActive({ ...setRef(scope, id), base: s.f });
  applyFilter(s.f);
  if (scope !== 'all') emit('itemUsed', { kind: 'filterSet', scope, id, name: s.name });
}

function useSets(refs) {
  const sets = refs.map((r) => findSet(r.scope, r.id, r.user)).filter(Boolean);
  if (sets.length < 2) return sets.length ? pickSet(sets[0].scope, sets[0].id) : pickSet('all', ALL_ID);
  setActive(
    null,
    sets.map(({ scope, id, user }) => copyRef({ scope, id, user })),
  );
  applyFilter(sets[0].f);
}

export function toggleSet(scope, id) {
  const s = findSet(scope, id, profileName());
  if (!s) return;
  if (s.scope === 'all') return pickSet('all', ALL_ID);
  const now = stack.length ? stack : active && active.scope !== 'all' ? [active] : [];
  const on = !isActive(s.scope, s.id);
  useSets(on ? [...now, setRef(s.scope, s.id)] : now.filter((r) => !sameRef(r, setRef(s.scope, s.id))));
  if (on) emit('itemUsed', { kind: 'filterSet', scope: s.scope, id: s.id, name: s.name });
}

function stackMatchers() {
  if (!stackM) {
    stackM = activeStack().map((s) => ({
      f: s.f,
      srcOk: filterFn(s.f.src, { router: s.f.router }) || anything,
      dstOk: filterFn(s.f.dst, { router: s.f.router }) || anything,
    }));
  }
  return stackM.length > 1 ? stackM : null;
}

// Saves the filter as this browser's set and makes it active. id: the set to save over, which a new name renames. With
// no id, a set of the same name keeps its id, else the set is new. Gives the set's id.
export function saveLocalSet(name, id = '') {
  const was = localSets();
  const key = id || was.find((s) => sameName(s.name, name))?.id || newPresetId();
  const next = { scope: 'local', id: key, name, f: filterState() };
  const list = was.filter((s) => s.id === key || !sameName(s.name, name)).map((s) => (s.id === key ? next : s));
  if (!list.includes(next)) list.push(next);
  saveLocals(list);
  setActive({ scope: 'local', id: key, base: filterState() });
  emit('filters');
  return key;
}

export function putLocalSets(list) {
  saveLocals(list);
  emit('filters');
}

// Copy this browser to My user: moves [{ from, to }] leave this browser, and the active set or stack follows them to Me.
// The Me set keeps the moved set's id, unless Me had a set of that name (Use new), which keeps its own.
export function moveLocalSets(moves) {
  const to = new Map(moves.map((m) => [m.from.toLowerCase(), m.to]));
  const was = localSets();
  saveLocals(was.filter((s) => !to.has(s.name.toLowerCase())));
  const meId = (s) => userSets().find((u) => sameName(u.name, to.get(s.name.toLowerCase())))?.id || s.id;
  const ids = new Map(was.filter((s) => to.has(s.name.toLowerCase())).map((s) => [s.id, meId(s)]));
  const re = (r) => (r.scope === 'local' && ids.has(r.id) ? { ...r, ...setRef('user', ids.get(r.id)) } : r);
  setActive(active && re(active), stack.map(re));
  emit('filters');
}

export function deleteLocalSet(id) {
  saveLocals(localSets().filter((s) => s.id !== id));
  if (stack.length) useSets(stack);
  else if (active?.scope === 'local' && active.id === id) setActive(null);
  emit('filters');
}

// Follow a global set that changed in Settings unless this browser changed the filter since. A
// stack cannot have been edited, so it follows (and loses sets that are gone).
function followGlobal() {
  if (stack.length) return useSets(stack);
  const s = active?.scope === 'global' && findSet('global', active.id);
  if (!s || sameFilter(s.f, active.base) || !sameFilter(filter, active.base)) return;
  setActive({ scope: 'global', id: s.id, base: s.f });
  setFilter(s.f);
}

// The same for the user's sets, edited on another device. A set of another user cannot be used
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
      pickSet('all', ALL_ID);
      return;
    }
  }
  if (stack.length) return useSets(stack);
  const s = active?.scope === 'user' && mine(active) && findSet('user', active.id, now);
  if (!s || sameFilter(s.f, active.base) || !sameFilter(filter, active.base)) return;
  setActive({ ...setRef('user', s.id), base: s.f });
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

// Compiled one time per expression. Blank (or unparsable) hides nothing.
let hiddenText = null,
  hiddenFn = null;
function globallyHidden(num, name, at) {
  const t = (S.cfg?.hiddenDsts || '').trim();
  const router = S.cfg?.hiddenDstsRouter || '';
  if (`${router} ${t}` !== hiddenText) {
    hiddenText = `${router} ${t}`;
    hiddenFn = t ? filterFn(t, { router }) : null;
  }
  return !!hiddenFn && hiddenFn(num, name, at);
}

// Quick search is history.js's. With a stack, each set's saved filter judges it and pref
// setStack says whether all must allow, or one is enough.
// A tie take matches by its typed ends, each on its own router, and a hop entry never matches (tiehistoryrules.js).
export function filterAllows(e) {
  const ends = typedEnds(e);
  if (!ends) return false;
  const s = srcItem(ends.src.n, ends.src.name, ends.src.router),
    d = dstItem(ends.dst.n, ends.dst.name, ends.dst.router);
  const dn = ends.dst.n + off();
  const router = ends.dst.router || S.routerUid;
  const srcRouter = ends.src.router || S.routerUid;
  const x = {
    ext: !!e.ext,
    sn: ends.src.n + off(),
    sname: s.name,
    srcRouter,
    dn,
    dname: d.name,
    router,
    hiddenDst: !!e.ext && globallyHidden(dn, d.name, router),
  };
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
  // A rename changes the stored label of Everyone's bookmarks on the set in the same save, so the message carries them.
  S.cfg = { ...S.cfg, filterSets: m.sets, ...(Array.isArray(m.bookmarks) ? { bookmarks: m.bookmarks } : {}) };
  followGlobal();
  emit('filters');
  emit('history');
});
