// Keys the panel page keeps in this browser: the one a setup link trades for, and a link key that worked.
// Server side: POST /api/v1/panels/{id}/setup (internal/app/panelsetup.go). The stored name is in docs/persisted-names.md.
import { setupFailText, storedKeyName, withoutKey, withoutSetup } from './panelkeyrules.js';
import { assignAddress } from './panelassignrules.js';
import { instanceFrom } from './panellink.js';
import { writeStr } from './store.js';
import { DEVICE_KEY } from './panelkeepalive.js';

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

// An Assign: keep the target's key under the address the page will open, then open it. A full load drops the staged
// routes and every timer of the old panel, and a reload lands on the same address and key.
export function followAssign(msg) {
  const { seg } = assignAddress(msg, location, true);
  let stored = false;
  try {
    localStorage.setItem(storedKeyName(seg), msg.key);
    stored = localStorage.getItem(storedKeyName(seg)) === msg.key;
  } catch {
    /* the key rides the fragment instead */
  }
  // A name chosen after a clash: with no ?instance= to carry it, the stored device name does.
  if (!instanceFrom(location.search)) writeStr(DEVICE_KEY, msg.instance);
  location.replace(new URL(assignAddress(msg, location, stored).href, document.baseURI).href);
}
