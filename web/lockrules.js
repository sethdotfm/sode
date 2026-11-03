// The lock screen's pure rules (#151): PIN entry, when to ask the server,
// idle time, the logo's flight and who made a take. No imports, so Node
// tests them.

// The PIN length the pad stops at: the longest in use, never past maxLen
// (the configured longest PIN; 0 = none given, 12 is the hard limit).
export const padCap = (lens, maxLen = 0) => Math.min(Math.max(...lens, 0), maxLen > 0 ? maxLen : 12);

// The PIN after a pad or keyboard key: a digit, 'back' or 'clear'. lens are
// the users' PIN lengths; nothing longer than the longest can be right.
export function padKey(pin, key, lens, maxLen = 0) {
  if (key === 'clear') return '';
  if (key === 'back') return pin.slice(0, -1);
  if (/^[0-9]$/.test(key) && pin.length < padCap(lens, maxLen)) return pin + key;
  return pin;
}

// Whether pin is a length some user has, so it is worth asking the server.
// The pad's check key and Enter send whatever is typed if this holds.
export const sendable = (pin, lens) => lens.includes(pin.length);

// How long to wait before sending pin on its own, or null for not at all.
// The pad submits by itself only at the LONGEST length in use: a shorter PIN
// may be the start of a longer one, and a pause after four digits must never
// send the 4-digit PIN of someone whose PIN has five. Shorter PINs go with
// the check key or Enter (sendable).
export function sendDelay(pin, lens) {
  if (!lens.length) return null;
  return pin.length === Math.max(...lens) ? 0 : null;
}

// The configured user a history entry's `client` names, or '' (an address,
// an ext/found entry, or a user since removed). Never returns an address.
export function actorUser(client, users) {
  if (!client || !users?.length) return '';
  const u = users.find((x) => x.name === client);
  return u ? u.name : '';
}

// One name for a folder's entries when they all share it, else ''.
export function folderActor(entries, users) {
  const names = new Set(entries.map((e) => actorUser(e.client, users)));
  return names.size === 1 ? [...names][0] : '';
}

// Whether the idle time has run out; 0 minutes = never.
export const idleDue = (lastActive, now, minutes) => minutes > 0 && now - lastActive >= minutes * 60_000;

// Whether the idle timer should lock this browser now. s is the lock state
// ({enforced, locked, user, persistent}). A persistent ("never log out") user
// is skipped: the server says so, and it skips ONLY the timer (Ctrl+L and the
// Lock button always work).
export const idleLock = (s, lastActive, now, minutes) =>
  !!(s.enforced && !s.locked && s.user) && !s.persistent && idleDue(lastActive, now, minutes);

// Whether a low-risk settings save (the Settings form, filter sets, salvos)
// must ask for the settings PIN first. A trusted user's connection needs none
// unless the server just refused one (forced: it wanted the PIN after all).
// The server decides; this only hides the prompt.
export const needsPin = (pinSet, unlocked, trusted, forced) =>
  !!pinSet && !unlocked && !(trusted && !forced);

// Which kind of page this is, by its path.
export function pageType(pathname) {
  if (/\/log(\.html)?\/?$/.test(pathname)) return 'log';
  return 'main';
}

// The transform that puts the box `to` (where the logo ends up) over the box
// `from` (where the page's own logo is): translate its centre, scale its width.
export function liftFrom(from, to) {
  if (!from || !from.width || !to || !to.width) return { x: 0, y: 0, scale: 1 };
  return {
    x: from.left + from.width / 2 - (to.left + to.width / 2),
    y: from.top + from.height / 2 - (to.top + to.height / 2),
    scale: from.width / to.width,
  };
}

// Whether the logo flies between the page's own and the middle. Not when the
// page is scrolled or the logo isn't fully in view: the overlay then just
// fades, since a flight from or to somewhere off screen would look broken.
export function shouldFly(rect, scrollY, innerHeight) {
  if (!rect || !rect.width || scrollY > 0) return false;
  return rect.top >= 0 && rect.bottom <= innerHeight;
}

// The brand's keyframes for one run, new objects every call so a run never
// shares anything with the last. dir 'in' = locking, 'out' = unlocking; t is
// liftFrom's result; fly false = a plain fade. Out holds full opacity for the
// first part and fades over the rest, so nothing snaps off at the end.
export function brandFrames(dir, fly, t) {
  const at = `translate(${t.x}px, ${t.y}px) scale(${t.scale})`;
  if (!fly) return dir === 'in' ? [{ opacity: 0 }, { opacity: 1 }] : [{ opacity: 1 }, { opacity: 0 }];
  return dir === 'in'
    ? [{ transform: at, opacity: 1 }, { transform: 'none', opacity: 1 }]
    : [{ transform: 'none', opacity: 1, offset: 0 }, { opacity: 1, offset: 0.55 }, { transform: at, opacity: 0, offset: 1 }];
}

// The way out takes this many times as long as the way in, so it ends gently.
export const OUT_FACTOR = 1.5;

// Dots for the PIN typed so far.
export const dots = (pin) => '●'.repeat(pin.length);
