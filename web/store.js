// Never throws: if localStorage is missing, blocked or full, reads give the default and writes are dropped. null removes the key.
// Writes return whether they landed.

// The session keys: a step-in writes none of them (stepin.js), so a reload or another tab still sees the original.
const SESSION_KEYS = new Set(['sode-session', 'sode-tab-session', 'sode-last-user', 'sode-held-user', 'sode-profile-prefs']);
let keysHeld = () => false;
export const holdSessionKeys = (fn) => {
  keysHeld = fn;
};
const held = (k) => SESSION_KEYS.has(k) && keysHeld();

export const readStr = (k) => {
  try {
    return localStorage.getItem(k);
  } catch {
    return null;
  }
};
export const writeStr = (k, v) => {
  if (held(k)) return false;
  try {
    if (v == null) localStorage.removeItem(k);
    else localStorage.setItem(k, v);
    return true;
  } catch {
    return false;
  }
};
export const readJSON = (k, d = null) => {
  try {
    return JSON.parse(localStorage.getItem(k)) ?? d;
  } catch {
    return d;
  }
};
export const writeJSON = (k, v) => writeStr(k, v == null ? null : JSON.stringify(v));

export const readTab = (k) => {
  try {
    return sessionStorage.getItem(k);
  } catch {
    return null;
  }
};
export const writeTab = (k, v) => {
  if (held(k)) return;
  try {
    if (v == null) sessionStorage.removeItem(k);
    else sessionStorage.setItem(k, v);
  } catch {}
};
