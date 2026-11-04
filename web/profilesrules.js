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
