// Keys the panel page keeps in this browser: the one a setup link trades for, and a link key that worked.
// Server side: POST /api/v1/panels/{id}/setup (internal/app/panelsetup.go). The stored name is in docs/persisted-names.md.
import { setupFailText, storedKeyName, withoutKey, withoutSetup } from './panelkeyrules.js';

export function storedKey(id) {
  try {
    return localStorage.getItem(storedKeyName(id)) || '';
  } catch {
    return '';
  }
}

export function forgetKey(id) {
  try {
    localStorage.removeItem(storedKeyName(id));
  } catch {
    /* nothing was kept */
  }
}

// A link key that worked: keep it and drop it from the address. Read back before the address loses it, since
// with storage blocked or full a reload has nothing else to use. Answers whether it was kept.
export function keepLinkKey(id, key) {
  try {
    localStorage.setItem(storedKeyName(id), key);
    if (localStorage.getItem(storedKeyName(id)) !== key) return false;
  } catch {
    return false;
  }
  history.replaceState(null, '', `${location.pathname}${location.search}${withoutKey(location.hash)}`);
  return true;
}

// {key} or {error}. The spent token leaves the address either way, so a reload does not try it again.
export async function exchangeSetup(id, token) {
  history.replaceState(null, '', `${location.pathname}${location.search}${withoutSetup(location.hash)}`);
  let res;
  try {
    res = await fetch(`api/v1/panels/${id}/setup`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ setup: token }) });
  } catch {
    return { error: 'Sode did not answer. Try the link again.' };
  }
  const body = await res.json().catch(() => ({}));
  if (!res.ok) return { error: setupFailText(res.status, body.error?.message) };
  try {
    localStorage.setItem(storedKeyName(id), body.key);
  } catch {
    /* the key serves this page only, and a reload needs a new link */
  }
  return { key: body.key };
}
