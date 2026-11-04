// The logged-in user's profile (Beta 1.6, #161): their saved This device
// settings, filter sets and presets, kept on the server and pushed to every
// connection unlocked as that user (`profile` message; name '' clears it).
// This is the ONLY module that handles `profile` and `profileSaveResult`
// (onMsg allows one handler per type); everything else reads it from here
// and listens for the 'profile' bus event. Never in the hello, so it is empty
// for a locked or anonymous page.

import { on, onMsg, emit, wsSend } from './core.js';

const EMPTY = Object.freeze({ name: '', prefs: {}, filterSets: [], presets: [] });
let cur = EMPTY;

// { name, prefs, filterSets, presets }; name '' means nobody / no users.
export const profile = () => cur;
export const profileName = () => cur.name;

// What the server sent, as a profile (a clear, or junk, is EMPTY).
export function toProfile(m) {
  if (!m || typeof m.name !== 'string' || !m.name) return EMPTY;
  return {
    name: m.name,
    prefs: m.prefs && typeof m.prefs === 'object' ? m.prefs : {},
    filterSets: Array.isArray(m.filterSets) ? m.filterSets : [],
    presets: Array.isArray(m.presets) ? m.presets : [],
  };
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
  done(m.ok ? { ok: true, err: '', locked: false, profile: toProfile({ ...m.profile, name: m.name }) }
    : { ok: false, err: m.err || 'Could not read that profile.', locked: !!m.locked, profile: EMPTY });
});

onMsg('profileSaveResult', (m) => {
  const done = pending.get(m.id);
  if (!done) return;
  pending.delete(m.id);
  done({ ok: !!m.ok, err: m.err || '', locked: !!m.locked });
});

// A dropped socket never answers: settle everything waiting so a button or
// dialog isn't stuck for ever.
on('wsclose', () => {
  for (const done of pending.values()) done(lost);
  pending.clear();
  for (const done of reading.values()) done({ ...lost, profile: EMPTY });
  reading.clear();
});

// Saves go one at a time, each after the last one's answer: the server may
// handle messages of one connection out of order, and each save replaces
// whole parts, so two in flight could lose one.
let chain = Promise.resolve();

// Saves some parts of a profile ({prefs}, {filterSets}, {presets}; a part
// left out is kept; a part sent replaces the whole list/map). `name` is the
// owner: the logged-in user by default; a trusted user may name another.
// Resolves { ok, err, locked }. No PIN is asked for here: the server decides
// (own profile: no PIN; someone else's: trusted user or settings PIN).
// For the own profile the local copy takes the new parts at once, so the next
// save is built from them (the server's `profile` push confirms it); a failed
// save puts it back.
export function saveProfile(parts, name = cur.name) {
  if (!name) return Promise.resolve({ ok: false, err: 'Not logged in.', locked: false });
  // At call time, not when the queued send runs: the next caller reads it now.
  const before = cur;
  const mine = name === cur.name;
  const next = mine ? { ...cur, ...parts } : null;
  if (mine) cur = next;
  const run = () => new Promise((resolve) => {
    const id = ++seq;
    const settle = (r) => {
      if (!r.ok && mine && cur === next) cur = before;
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

// Reads any user's profile (a trusted user or the settings PIN; the server
// refuses others with locked: true). Resolves { ok, err, locked, profile }.
export function fetchProfile(name) {
  return new Promise((resolve) => {
    const id = ++seq;
    if (!wsSend({ t: 'profileGet', id, name })) {
      return resolve({ ok: false, err: 'Not connected to Sode.', locked: false, profile: EMPTY });
    }
    reading.set(id, resolve);
  });
}
