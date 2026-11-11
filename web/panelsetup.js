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
