// Pure parts of the Profiles dialog (profiles.js). No imports but prefs.js.

import { PREFS } from './prefs.js';

const plural = (n, one, many = one + 's') => `${n} ${n === 1 ? one : many}`;

const LABELS = {
  theme: 'Theme',
  layout: 'Layout',
  clock: 'Clock',
  flip: 'Destination before Source',
  scrollTop: 'Scroll to top',
  setStack: 'Stacked sets',
  nextMode: 'Next chip',
  searchFilters: 'Quick search uses filters',
};
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

export function withoutPref(prefs, key) {
  const rest = { ...(prefs || {}) };
  delete rest[key];
  return rest;
}

export function setSummary(s) {
  const bits = [];
  if (s?.src) bits.push(`sources ${s.src}`);
  if (s?.dst) bits.push(`destinations ${s.dst}`);
  const kinds = [s?.sode === false && 'Sode takes', s?.ext === false && 'changes made elsewhere'].filter(Boolean);
  if (kinds.length) bits.push(`hides ${kinds.join(' and ')}`);
  if (s?.hidden === true) bits.push('shows hidden destinations');
  return bits.length ? bits.join('; ') : 'Shows everything';
}

export function presetSummary(p) {
  const items = Array.isArray(p?.items) ? p.items : [];
  const dsts = items.reduce((n, it) => n + (Array.isArray(it?.dsts) ? it.dsts.length : 0), 0);
  return `${plural(items.length, 'take')}, ${plural(dsts, 'destination')}`;
}

export const without = (list, i) => (Array.isArray(list) ? list.filter((_, j) => j !== i) : []);

export const profileButtonOff = (was, typed) => !was || String(typed).trim() !== was;

export const canSeeProfiles = (names, trusted, unlocked) => (names?.length || 0) > 0 && !!(trusted || unlocked);

// ---- editing a profile

// value '' = Not set (also for an invalid one); options are [value, text] pairs, Not set first.
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

// '' (Not set) removes the key; a value not allowed for that setting changes nothing.
export function prefsWith(prefs, key, value) {
  const def = PREFS[key];
  const next = { ...(prefs || {}) };
  if (!def) return next;
  if (value === '' || value == null) delete next[key];
  else if (Object.hasOwn(def.values, value)) next[key] = value;
  return next;
}

export function upsertEntry(list, entry, old = '') {
  const items = Array.isArray(list) ? list : [];
  const at = old ? items.findIndex((x) => String(x?.name).toLowerCase() === old.toLowerCase()) : -1;
  const next = items.slice();
  if (at >= 0) next[at] = entry;
  else next.push(entry);
  return next;
}

export const filterEntry = (name, f) => ({ name: String(name).trim(), ...f });

export const findByName = (list, name) => (Array.isArray(list) ? list : []).find((x) => String(x?.name).toLowerCase() === String(name).toLowerCase());

// Applied when a change is made, not when the save returns, so a second change builds on the first.
export const withParts = (prof, parts) => ({ ...(prof || {}), ...parts });

// Server caps a whole profile at 48 KiB (socket read limit is 64).
export const PROFILE_MAX_BYTES = 48 * 1024;

export function profileProblem(p, parts) {
  const next = { prefs: p.prefs, filterSets: p.filterSets, presets: p.presets, pinOrder: p.pinOrder, ...parts };
  // UTF-8 bytes as the server counts them: Go's json.Marshal also writes < > & as 6-byte escapes.
  const json = JSON.stringify(next);
  const bytes = new TextEncoder().encode(json).length + 5 * (json.match(/[<>&]/g) || []).length;
  return bytes > PROFILE_MAX_BYTES
    ? `Your profile is too big to save (${Math.ceil(bytes / 1024)} KiB, at most ${PROFILE_MAX_BYTES / 1024} KiB). Delete something first.`
    : '';
}
