// Secure mode on the panel page: trade a one-time setup link for a key and keep it in this browser.
// Server side: POST /api/v1/panels/{id}/setup (internal/app/panelsetup.go). The stored name is in docs/persisted-names.md.
import { setupFailText, storedKeyName, withoutSetup } from './panelkeyrules.js';

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

// A Public panel's short-lived key (internal/app/panelpublic.go), or '' for any other panel or a failed ask, which
// leaves the page to its session. Held in memory only: a reload asks again.
export async function askPublic(id) {
  try {
    const res = await fetch(`api/v1/panels/${id}/public`, { method: 'POST' });
    if (res.status !== 201) return '';
    return (await res.json()).key || '';
  } catch {
    return '';
  }
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
