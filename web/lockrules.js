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

export function folderActor(entries, users) {
  const who = new Set(entries.map((e) => actorUser(e.client, users)));
  return who.size === 1 ? [...who][0] : '';
}

export const idleDue = (lastActive, now, minutes) => minutes > 0 && now - lastActive >= minutes * 60_000;

// s is the lock state ({enforced, locked, user, persistent}). A persistent user is skipped by
// the idle timer ONLY (Ctrl+L and the Lock button always work).
export const idleLock = (s, lastActive, now, minutes) => !!(s.enforced && !s.locked && s.user) && !s.persistent && idleDue(lastActive, now, minutes);

export const softIdleLock = (s, handLocked, lastActive, now, minutes) => !s.enforced && !handLocked && idleDue(lastActive, now, minutes);

// Mirrors the Go `guard:"router"` and `guard:"pin"` tags in internal/config/config.go: keep in
// step.
const GUARDED = ['router', 'matrix', 'levels', 'levelWireOffset', 'nameLength', 'nameRequests', 'fireMode', 'salvoGroup', 'pinMaxTries', 'autoLockMinutes'];
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

// Tab login: users exist, this page has no PIN lock, nobody is logged in. The pad it opens can
// be dismissed.
export const loginOffered = (s) => !s.enforced && !s.user && (s.lens?.length || 0) > 0;

// The server then keeps the user in for this tab only. A page with a lock never asks; its idle
// timer stays in charge.
export const wantsTab = (s) => !s.enforced;

// This tab's own (sessionStorage) wins over the browser's persistent one (localStorage); either
// may be empty.
export const hiToken = (tab, persistent) => tab || persistent || '';

export const tokenHome = (m) => (m.tab ? 'tab' : 'browser');

export const loginAsOffered = (s) => !s.enforced && (s.lens?.length || 0) > 0;

// One value {mode, back}. mode is one of MODE: PAGE (an enforced lock may still cover it, see
// viewBody), SOFT (hand lock), LOGIN (pad by request, dismissable) or OWN (logged-in user's
// lock, no PIN lock: pad, not dismissable). back: where Cancel / Esc of a login returns to
// ('page' or 'soft'). Pure; lock.js does the DOM.
export const MODE = Object.freeze({ PAGE: 'page', SOFT: 'soft', LOGIN: 'login', OWN: 'userlock' });
export const PAGE_VIEW = Object.freeze({ mode: MODE.PAGE, back: 'page' });

export const viewBody = (v, enforcedLocked) => (v.mode === MODE.SOFT ? 'soft' : enforcedLocked || v.mode === MODE.LOGIN || v.mode === MODE.OWN ? 'pad' : '');

// Locking while logged in with no PIN lock: the PIN pad with no Cancel, and the session ended
// on the server. The hand lock is only for a page nobody is logged in on.
export const userLock = (v, enforced) => (enforced ? v : { mode: MODE.OWN, back: 'page' });

// Nothing with an enforced lock on; wins over a login pad.
export const softLock = (v, enforced) => (enforced ? v : { mode: MODE.SOFT, back: 'page' });

export const softUnlock = () => PAGE_VIEW;

export const startLogin = (v, fromSoft) =>
  fromSoft ? (v.mode === MODE.SOFT ? { mode: MODE.LOGIN, back: 'soft' } : v) : v.mode === MODE.PAGE ? { mode: MODE.LOGIN, back: 'page' } : v;

// The view stays SOFT so the page stays covered; "Log in as..." remains when users exist.
export const logOutOffered = (s) => !s.enforced && !!s.user;

export const cancelLogin = (v) => (v.mode === MODE.LOGIN ? PAGE_VIEW : v);

export const lockArrived = (v, enforced) => (enforced && (v.mode === MODE.SOFT || v.mode === MODE.OWN) ? PAGE_VIEW : v);

// A login ends only with an enforced lock or a real unlock; any other state (someone else saved
// settings, a token catch-up) says nothing about the pad.
export const loginWon = (v, { enforced, unlocked, user }) =>
  (v.mode === MODE.LOGIN || v.mode === MODE.OWN) && (enforced || (unlocked && user)) ? PAGE_VIEW : v;

export const chipShown = ({ covered, soft, user }) => (soft ? !!user : !covered);
