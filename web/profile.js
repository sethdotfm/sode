// The logged-in user's profile (This device prefs, filter sets, presets), pushed to every connection unlocked as that user (name '' clears it).
// The ONLY module handling `profile` and `profileSaveResult` (onMsg allows one handler per type); others use the 'profile' bus event.
// Never in the hello, so empty for a locked or anonymous page.

import { on, onMsg, emit, wsSend } from './core.js';
import { findByName } from './profilesrules.js';

const EMPTY = Object.freeze({ name: '', prefs: {}, filterSets: [], presets: [], pinOrder: [] });
let cur = EMPTY;

export const profile = () => cur;
export const profileName = () => cur.name;

export function toProfile(m) {
  if (!m || typeof m.name !== 'string' || !m.name) return EMPTY;
  return {
    name: m.name,
    prefs: m.prefs && typeof m.prefs === 'object' ? m.prefs : {},
    filterSets: Array.isArray(m.filterSets) ? m.filterSets : [],
    presets: Array.isArray(m.presets) ? m.presets : [],
    pinOrder: Array.isArray(m.pinOrder) ? m.pinOrder.filter((k) => typeof k === 'string') : [],
  };
}

// What an editor aimed at one user's profile (bus 'openFilterEditor' / 'openPresetEditor') works on; null without a user.
export function editTarget(m, key) {
  const user = String(m?.user || '');
  if (!user) return null;
  const profile = toProfile({ ...m.profile, name: user });
  const lists = profile[key];
  const found = m.name ? findByName(lists, m.name) || null : null;
  return { user, profile, lists, found, old: found ? String(found.name) : '' };
}

onMsg('profile', (m) => {
  cur = toProfile(m);
  emit('profile', cur);
});

let seq = 0;
const pending = new Map(); // save id -> resolve
const reading = new Map(); // get id -> resolve
const lost = { ok: false, err: 'Connection lost.', locked: false };

onMsg('profileGetResult', (m) => {
  const done = reading.get(m.id);
  if (!done) return;
  reading.delete(m.id);
  done(
    m.ok
      ? { ok: true, err: '', locked: false, profile: toProfile({ ...m.profile, name: m.name }) }
      : { ok: false, err: m.err || 'Could not read that profile.', locked: !!m.locked, profile: EMPTY },
  );
});

onMsg('profileSaveResult', (m) => {
  const done = pending.get(m.id);
  if (!done) return;
  pending.delete(m.id);
  done({ ok: !!m.ok, err: m.err || '', locked: !!m.locked });
});

// A dropped socket never answers: settle everything waiting.
on('wsclose', () => {
  for (const done of pending.values()) done(lost);
  pending.clear();
  for (const done of reading.values()) done({ ...lost, profile: EMPTY });
  reading.clear();
});

// Saves go one at a time: the server may handle one connection's messages out of order and each save replaces whole parts.
let chain = Promise.resolve();

// Saves some parts ({prefs}, {filterSets}, {presets}, {pinOrder}); a part left out is kept, a part sent replaces the whole list/map.
// `name` defaults to the logged-in user; a trusted user may name another. No PIN is asked here: the server decides.
// The own profile's local copy takes the parts at once so the next save builds on them; a failed save puts it back.
export function saveProfile(parts, name = cur.name) {
  if (!name) return Promise.resolve({ ok: false, err: 'Not logged in.', locked: false });
  // At call time, not when the queued send runs: the next caller reads it now.
  const before = cur;
  const mine = name === cur.name;
  const next = mine ? { ...cur, ...parts } : null;
  if (mine) cur = next;
  const run = () =>
    new Promise((resolve) => {
      const id = ++seq;
      const settle = (r) => {
        if (!r.ok && mine && cur === next) cur = before;
        if (r.ok) emit('profileSaved', { name });
        resolve(r);
      };
      if (!wsSend({ t: 'profileSave', id, name, ...parts })) {
        return settle({ ok: false, err: 'Not connected to Sode.', locked: false });
      }
      pending.set(id, settle);
    });
  const p = chain.then(run);
  chain = p.catch(() => {});
  return p;
}

// Any user's profile (trusted user or settings PIN; others get locked: true).
export function fetchProfile(name) {
  return new Promise((resolve) => {
    const id = ++seq;
    if (!wsSend({ t: 'profileGet', id, name })) {
      return resolve({ ok: false, err: 'Not connected to Sode.', locked: false, profile: EMPTY });
    }
    reading.set(id, resolve);
  });
}
