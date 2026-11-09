// Session tokens and the `hi` handshake, shared by every page that signs in (main, /log and the
// three panel pages). lock.js owns the pad and the Lock; this owns what survives a reload.
import { S, on, onMsg, wsSend } from './core.js';
import { readStr, writeStr, readTab, writeTab } from './store.js';
import { hiToken, tokenHome } from './lockrules.js';

// localStorage rather than a cookie: the socket's `hi` carries it, no HTTP endpoint is needed,
// and a cookie would be JS-readable anyway (see docs/agents/users-locks.md).
const TOKEN_KEY = 'sode-session';
// A no-lock sign-in's token: this tab only (sessionStorage), so closing the tab signs out while a
// reload stays in.
const TAB_KEY = 'sode-tab-session';
// Whose Lock this tab shows: this tab's sessionStorage, so a reload still names them. The
// username box's prefill is the browser's (localStorage); no name ever comes from the server.
const HELD_KEY = 'sode-held-user';
const LAST_KEY = 'sode-last-user';

// Drop keys of the removed admin-browser sign-in.
writeStr('sode-lock-token', null);
writeStr('sode-lock-never', null);

export const heldUserKept = () => readTab(HELD_KEY) || '';
export const keepHeldUser = (name) => writeTab(HELD_KEY, name || null);
export const lastUser = () => readStr(LAST_KEY);
export const keepLastUser = (name) => writeStr(LAST_KEY, name || null);

// Sends `hi` once per socket, as the page type says.
export function startHandshake(page) {
  let hiFor = null;
  on('conn', () => {
    if (S.wsState !== 'up' || S.ws === hiFor) return;
    hiFor = S.ws;
    wsSend({ t: 'hi', page, token: hiToken(readTab(TAB_KEY), readStr(TOKEN_KEY)) });
  });
}

// A token handed out with a sign-in answer lands where the server says (tab or browser).
export function keepSignInToken(m) {
  if (!m.token) return;
  if (tokenHome(m) === 'tab') writeTab(TAB_KEY, m.token);
  else writeStr(TOKEN_KEY, m.token);
}

export function forgetTokens() {
  if (readStr(TOKEN_KEY)) writeStr(TOKEN_KEY, null);
  if (readTab(TAB_KEY)) writeTab(TAB_KEY, null);
}

// Sign out: tokens, the remembered name and the held-Lock name all go.
export function forgetSignIn() {
  writeStr(TOKEN_KEY, null);
  writeStr(LAST_KEY, null);
  writeTab(TAB_KEY, null);
  writeTab(HELD_KEY, null);
}

// A token sent outside an unlock (user already signed in when Never sign out was ticked, or
// changed their own PIN). signedIn: this page has a user, so the token is for it.
export function onSessionToken(signedIn) {
  onMsg('session', (m) => {
    // A held Lock's tab token arrives after the page already raised the Lock, so signedIn() is false.
    if (m.token && m.tab) writeTab(TAB_KEY, m.token);
    else if (m.token && signedIn()) writeStr(TOKEN_KEY, m.token);
  });
}
