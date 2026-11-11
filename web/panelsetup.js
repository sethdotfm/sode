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

// A Public panel's short-lived key (internal/app/panelpublic.go), or '' when Sode says the panel is not Public. Any
// other answer throws: a lockout or a lost link must not read as "no longer Public". Held in memory only.
export async function askPublic(id) {
  const res = await fetch(`api/v1/panels/${id}/public`, { method: 'POST' });
  if (res.status === 200) return '';
  if (res.status !== 201) throw Object.assign(new Error('no Public key'), { status: res.status });
  return (await res.json()).key || '';
}

// At open, a failed ask leaves the page to its session.
export const askPublicAtOpen = (id) => askPublic(id).catch(() => '');

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
// routes and every timer of the old panel, and a reload lands on the same address and key. With no key the target
// signs in a user, and a key stored earlier for that address would open it as its client instead, so it goes.
export function followAssign(msg) {
  const { seg } = assignAddress(msg, location, true);
  let stored = !msg.key;
  if (stored) forgetKey(seg);
  else {
    try {
      localStorage.setItem(storedKeyName(seg), msg.key);
      stored = localStorage.getItem(storedKeyName(seg)) === msg.key;
    } catch {
      /* the key rides the fragment instead */
    }
  }
  // A name chosen after a clash: with no ?instance= to carry it, the stored device name does.
  if (!instanceFrom(location.search)) writeStr(DEVICE_KEY, msg.instance);
  location.replace(new URL(assignAddress(msg, location, stored).href, document.baseURI).href);
}
