// The pure parts of the Profiles dialog (profiles.js): how a profile's parts
// read as lines. No imports but prefs.js, so Node tests it.

import { PREFS } from './prefs.js';

const plural = (n, one, many = one + 's') => `${n} ${n === 1 ? one : many}`;

// This device settings as [{key, label, value}], in PREFS order; values that
// aren't valid for their setting are left out (the browser ignores them too).
const LABELS = { theme: 'Theme', layout: 'Layout', clock: 'Clock', flip: 'Destination before Source', scrollTop: 'Scroll to top', setStack: 'Stacked sets', nextMode: 'Next chip', searchFilters: 'Quick search follows filters' };
export function prefRows(prefs) {
  const out = [];
  for (const [key, def] of Object.entries(PREFS)) {
    let v = prefs?.[key];
    if (typeof v === 'boolean') v = v ? 'on' : 'off';
    if (typeof v !== 'string' || !Object.hasOwn(def.values, v)) continue;
    out.push({ key, label: LABELS[key] || key, value: def.values[v] });
  }
  return out;
}

// The prefs map without one setting (what a Remove saves).
export function withoutPref(prefs, key) {
  const rest = { ...(prefs || {}) };
  delete rest[key];
  return rest;
}

// A filter set's short summary: what it shows and hides.
export function setSummary(s) {
  const bits = [];
  if (s?.src) bits.push(`sources ${s.src}`);
  if (s?.dst) bits.push(`destinations ${s.dst}`);
  const kinds = [s?.sode === false && 'Sode takes', s?.ext === false && 'changes made elsewhere'].filter(Boolean);
  if (kinds.length) bits.push(`hides ${kinds.join(' and ')}`);
  if (s?.hidden === true) bits.push('shows hidden destinations');
  return bits.length ? bits.join('; ') : 'Shows everything';
}

// A preset's short summary: its takes and destinations.
export function presetSummary(p) {
  const items = Array.isArray(p?.items) ? p.items : [];
  const dsts = items.reduce((n, it) => n + (Array.isArray(it?.dsts) ? it.dsts.length : 0), 0);
  return `${plural(items.length, 'take')}, ${plural(dsts, 'destination')}`;
}

// The list without entry i (what a Remove saves).
export const without = (list, i) => (Array.isArray(list) ? list.filter((_, j) => j !== i) : []);

// Whether a users-editor row's Profile… button is off: a new row (no saved
// name) or one whose name was edited has no profile to open yet.
export const profileButtonOff = (was, typed) => !was || String(typed).trim() !== was;

// Whether the Profiles button is offered: users exist and the viewer is a
// trusted user or holds the settings PIN.
export const canSeeProfiles = (names, trusted, unlocked) => (names?.length || 0) > 0 && !!(trusted || unlocked);

// ---- editing a profile

// Every This device setting as a select: [{key, label, value, options}] in
// PREFS order. value is the saved one ('' = Not set, also for a value that
// isn't valid); options are [value, text] pairs with Not set first.
export function prefFields(prefs) {
  return Object.entries(PREFS).map(([key, def]) => {
    let v = prefs?.[key];
    if (typeof v === 'boolean') v = v ? 'on' : 'off';
    const ok = typeof v === 'string' && Object.hasOwn(def.values, v);
    return {
      key,
      label: LABELS[key] || key,
      value: ok ? v : '',
      options: [['', 'Not set'], ...Object.entries(def.values)],
    };
  });
}

// The prefs map with one setting changed: '' (Not set) removes it, a value
// that isn't allowed for that setting changes nothing. Other keys are kept.
export function prefsWith(prefs, key, value) {
  const def = PREFS[key];
  const next = { ...(prefs || {}) };
  if (!def) return next;
  if (value === '' || value == null) delete next[key];
  else if (Object.hasOwn(def.values, value)) next[key] = value;
  return next;
}

// The list with `entry` added, or put in place of `old` (matched ignoring
// case). The name is checked by the caller (core.js nameProblem).
export function upsertEntry(list, entry, old = '') {
  const items = Array.isArray(list) ? list : [];
  const at = old ? items.findIndex((x) => String(x?.name).toLowerCase() === old.toLowerCase()) : -1;
  const next = items.slice();
  if (at >= 0) next[at] = entry; else next.push(entry);
  return next;
}

// A filter set as saved in a profile: its name and the filter's own fields.
export const filterEntry = (name, f) => ({ name: String(name).trim(), ...f });

// The entry named `name` (ignoring case), or undefined.
export const findByName = (list, name) => (Array.isArray(list) ? list : [])
  .find((x) => String(x?.name).toLowerCase() === String(name).toLowerCase());

// The profile with `parts` ({prefs}, {filterSets}, {presets}) put in. The
// dialog applies this when a change is made, not when the save returns, so a
// second change before the first answer builds on the first.
export const withParts = (prof, parts) => ({ ...(prof || {}), ...parts });
