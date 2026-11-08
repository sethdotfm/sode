// The lock screen's pure rules. No imports, so Node tests them.

// maxLen: the configured longest PIN (0 = none given, 12 is the hard limit).
export const padCap = (lens, maxLen = 0) => Math.min(Math.max(...lens, 0), maxLen > 0 ? maxLen : 12);

export function padKey(pin, key, lens, maxLen = 0) {
  if (key === 'clear') return '';
  if (key === 'back') return pin.slice(0, -1);
  if (/^[0-9]$/.test(key) && pin.length < padCap(lens, maxLen)) return pin + key;
  return pin;
}

export const sendable = (pin, lens) => lens.includes(pin.length);

// The pad auto-sends only at the LONGEST length in use: a shorter PIN may be the start of a
// longer one, and a pause after four digits must never send the 4-digit PIN of someone whose
// PIN has five. Shorter PINs go with the check key or Enter (sendable).
export function sendDelay(pin, lens) {
  if (!lens.length) return null;
  return pin.length === Math.max(...lens) ? 0 : null;
}

// '' for an address, an ext/found entry, or a removed user. Never returns an address.
export function actorUser(client, names) {
  return client && names?.includes(client) ? client : '';
}

export function groupActor(entries, users) {
  const who = new Set(entries.map((e) => actorUser(e.client, users)));
  return who.size === 1 ? [...who][0] : '';
}

export const idleDue = (lastActive, now, minutes) => minutes > 0 && now - lastActive >= minutes * 60_000;

// s is the lock state ({locked, user}). Anyone signed in gets the server-held Lock, persistent
// users and tab sign-ins included; it never signs them out. A user marked No auto-lock is skipped
// (the server tells only that user, in lockState); the Lock button still works for them.
export const idleLock = (s, lastActive, now, minutes) => !!(!s.locked && s.user && !s.noAutoLock) && idleDue(lastActive, now, minutes);

// A held Lock has no user in the browser's state but is not a letter Lock: the pad stays.
export const localIdleLock = (s, letterUp, lastActive, now, minutes) => !s.enforced && !s.locked && !s.user && !letterUp && idleDue(lastActive, now, minutes);

// Mirrors the Go `guard:"router"` and `guard:"pin"` tags in internal/config/config.go: keep in
// step.
const GUARDED = ['router', 'matrix', 'levels', 'levelWireOffset', 'nameLength', 'nameRequests', 'fireMode', 'salvoGroup', 'pinMaxTries', 'autoLockMinutes', 'requireHttps'];
export const guardedEdited = (next, cur) =>
  !!cur &&
  (GUARDED.some((k) => next[k] !== cur[k]) ||
    !!next.locks?.main !== !!cur.locks?.main ||
    !!next.locks?.log !== !!cur.locks?.log ||
    (next.trustedProxies || []).join(',') !== (cur.trustedProxies || []).join(','));

// A low-risk settings save asks for the settings PIN first, except on a trusted user's
// connection. The server decides; this only hides the prompt.
export const needsPin = (pinSet, unlocked, trusted) => !!pinSet && !unlocked && !trusted;

export function pageType(pathname) {
  if (/\/log(\.html)?\/?$/.test(pathname)) return 'log';
  return 'main';
}

// Puts box `to` over box `from`: translate its centre, scale its width.
export function liftFrom(from, to) {
  if (!from || !from.width || !to || !to.width) return { x: 0, y: 0, scale: 1 };
  return {
    x: from.left + from.width / 2 - (to.left + to.width / 2),
    y: from.top + from.height / 2 - (to.top + to.height / 2),
    scale: from.width / to.width,
  };
}

// Not when the page is scrolled or the logo isn't fully in view: a flight from or to off screen
// would look broken, so the overlay just fades.
export function shouldFly(rect, scrollY, innerHeight) {
  if (!rect || !rect.width || scrollY > 0) return false;
  return rect.top >= 0 && rect.bottom <= innerHeight;
}

// New objects every call so a run never shares anything with the last. t is liftFrom's result;
// fly false = plain fade. 'out' holds full opacity first so nothing snaps off at the end.
export function brandFrames(dir, fly, t) {
  const at = `translate(${t.x}px, ${t.y}px) scale(${t.scale})`;
  if (!fly) return dir === 'in' ? [{ opacity: 0 }, { opacity: 1 }] : [{ opacity: 1 }, { opacity: 0 }];
  return dir === 'in'
    ? [
        { transform: at, opacity: 1 },
        { transform: 'none', opacity: 1 },
      ]
    : [
        { transform: 'none', opacity: 1, offset: 0 },
        { opacity: 1, offset: 0.55 },
        { transform: at, opacity: 0, offset: 1 },
      ];
}

export const OUT_FACTOR = 1.5;

export const WRONG_MS = 1000;
export const WELCOME_MS = 600;

const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);

// Plain wrong PIN: "Incorrect" for WRONG_MS, then the dots; with a wait ("try again in 30s")
// the wait is `note` under the field. Any other refusal stays whole in the field (ms 0 = until
// the next key). Always text, never colour alone.
export function wrongFeedback(err) {
  const e = String(err || '').trim();
  const m = /^wrong pin(?:[;.]\s*(.*))?$/i.exec(e);
  if (!e || m) return { text: 'Incorrect', ms: WRONG_MS, note: m?.[1] ? cap(m[1]) : '' };
  return { text: cap(e), ms: 0, note: '' };
}

export const dots = (pin) => '●'.repeat(pin.length);

// Random key never equal to the one before, so the answer can't be a habit. No i or l (easy to
// confuse), no o.
const CHURN_KEYS = 'abcdefghjkmnpqrstuvwxyz';
export function churnKey(last = '', rnd = Math.random) {
  let key;
  do key = CHURN_KEYS[Math.floor(rnd() * CHURN_KEYS.length)];
  while (key === last);
  return key;
}

// Tab sign-in: users exist, this page has no PIN lock, nobody is signed in. The pad it opens can
// be dismissed.
export const signInOffered = (s) => !s.enforced && !s.user && (s.lens?.length || 0) > 0;

// The server then keeps the user in for this tab only. A page with a lock never asks; its idle
// timer stays in charge.
export const wantsTab = (s) => !s.enforced;

// This tab's own (sessionStorage) wins over the browser's persistent one (localStorage); either
// may be empty.
export const hiToken = (tab, persistent) => tab || persistent || '';

export const tokenHome = (m) => (m.tab ? 'tab' : 'browser');

// Username and PIN mode asks for the username too, except where the held Lock of a signed-in
// session resumes: the server knows whose it is and a `user` naming someone else is a wrong try.
export const asksUsername = (s) => s.mode === 'username' && !s.held;

export const canSend = (s, pin, username) => sendable(pin, s.lens || []) && (!asksUsername(s) || username.trim() !== '');

// A pad that auto-sends waits for the username, so a full PIN typed first is not lost as a wrong try.
export const padDelay = (s, pin, username) => (asksUsername(s) && username.trim() === '' ? null : sendDelay(pin, s.lens || []));

export const signInMsg = (s, { pin, id, username }) => ({
  t: 'userUnlock',
  ...(asksUsername(s) ? { user: username.trim() } : {}),
  pin,
  id,
  tab: wantsTab(s),
});

export const signInAsOffered = (s) => !s.enforced && (s.lens?.length || 0) > 0;

// One value {mode, back}. mode is one of MODE: PAGE (a lock from the server may still cover
// it, see viewBody), LOCK (Lock with nobody signed in) or SIGN_IN (pad by request, dismissable). back: where
// Cancel / Esc of a sign-in returns to ('page' or 'lock'). Pure; lock.js does the DOM.
export const MODE = Object.freeze({ PAGE: 'page', LOCK: 'lock', SIGN_IN: 'signIn' });
export const PAGE_VIEW = Object.freeze({ mode: MODE.PAGE, back: 'page' });

export const viewBody = (v, covered) => (v.mode === MODE.LOCK ? 'letter' : covered || v.mode === MODE.SIGN_IN ? 'pad' : '');

// The server covers the page: a PIN lock with nobody signed in, or the held Lock of a
// signed-in session.
export const lockCovers = (s) => !!s.locked && !!(s.enforced || s.held);

// The Lock of a signed-in session keeps the session (ADR 0001). Until the server's state
// arrives the page already shows the pad.
export const raiseLock = (s) => ({ ...s, locked: true, held: true, user: '', trusted: false, persistent: false, noAutoLock: false });

export const signedOut = (s) => ({ ...s, locked: !!s.enforced, held: false, user: '', trusted: false, persistent: false, noAutoLock: false });

// Only the answer to our hi says whether the token was honored. A held Lock is the session
// still alive and waiting for its PIN, so its token stays.
export const tokenStale = (m, user) => !!m.hi && !user && !m.held;

// Nothing with an enforced lock on; wins over a sign-in pad.
export const lockView = (v, enforced) => (enforced ? v : { mode: MODE.LOCK, back: 'page' });

export const lockDismissed = () => PAGE_VIEW;

export const startSignIn = (v, fromLock) =>
  fromLock ? (v.mode === MODE.LOCK ? { mode: MODE.SIGN_IN, back: 'lock' } : v) : v.mode === MODE.PAGE ? { mode: MODE.SIGN_IN, back: 'page' } : v;

// The view stays LOCK so the page stays covered; "Sign in as…" remains when users exist.
export const signOutOffered = (s) => !s.enforced && !!s.user;

export const signInCancelled = (v) => (v.mode === MODE.SIGN_IN ? PAGE_VIEW : v);

// covered: a lock from the server is up (lockCovers).
export const lockArrived = (v, covered) => (covered && v.mode === MODE.LOCK ? PAGE_VIEW : v);

// A sign-in ends only with an enforced lock or a real unlock; any other state (someone else saved
// settings, a token catch-up) says nothing about the pad.
export const signInWon = (v, { enforced, unlocked, user }) => (v.mode === MODE.SIGN_IN && (enforced || (unlocked && user)) ? PAGE_VIEW : v);

// heldUser: whose held Lock this tab shows (the server's held state carries no name).
export const chipShown = ({ covered, letter, user, heldUser = '' }) => (letter ? !!user : !covered || !!heldUser);

// The server fills `user` only for a session that is signed in and not locked, so a held Lock that
// ends with no user ended by sign out, a removed user or a changed PIN; with a user it was resumed.
export const heldEnded = (prev, next) => !!prev.held && !next.held && !next.user;

// This tab's own memory of whose Lock is up: from the user it had when the Lock came, kept
// across reloads in sessionStorage, and forgotten once the Lock is gone.
export const heldWho = (kept, prev, next) => (next.held ? prev.user || kept : '');

// A username is kept for the next sign-in unless a browser-wide token already signs them in.
export const remembers = (m) => !m.token || !!m.tab;

export const prefillName = (s, last) => (asksUsername(s) ? last || '' : '');

// The pad takes the keys once the box already holds a name.
export const focusName = (s, name) => asksUsername(s) && !name.trim();
