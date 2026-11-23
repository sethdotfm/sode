// The person's Palette pins (GLOSSARY.md, Palette pin). A signed-in user's are Me (Profile.palettePins, profileSave
// `palettePins`). A Guest's are This browser's (localStorage sode-palette-pins). A signed-in user sees only Me: This
// browser's pins are hidden, not merged, and come back after sign-out. There is no Everyone list. Rules are in
// pinrules.js. The bus event 'pins' says that the list changed.

import { on, emit } from './core.js';
import { readJSON, writeJSON } from './store.js';
import { profile, profileName, saveProfile } from './profile.js';
import { cleanPins } from './pinrules.js';

export const PINS_KEY = 'sode-palette-pins';

export const pinScope = () => (profileName() ? 'user' : 'local');
export const ownPins = () => cleanPins(profileName() ? profile().palettePins : readJSON(PINS_KEY, []));
export const localPins = () => cleanPins(readJSON(PINS_KEY, []));

// Saves the whole list. Answers { ok, err }. This browser's is written at once.
export function savePins(list) {
  if (!profileName()) return putLocalPins(list);
  const r = saveProfile({ palettePins: list });
  emit('pins');
  return r.then((res) => {
    if (!res.ok) emit('pins');
    return res;
  });
}

// This browser's list, also from an import while a user is signed in.
export function putLocalPins(list) {
  writeJSON(PINS_KEY, list.length ? list : null);
  emit('pins');
  return Promise.resolve({ ok: true });
}

on('profile', () => emit('pins'));
