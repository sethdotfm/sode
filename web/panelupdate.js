// When an open panel takes a changed copy of itself. The server moves the panel's rev on every stored
// change (a saved edit, later a Push); the page polls it and asks this module what to do. No DOM and no
// clock, so the rules test directly (test/panelupdate.test.mjs). Every function returns a new state.
export const FORCE_MS = 10000;
export const SNOOZE_MS = 60000;

// seen: the rev on screen. since: when a newer one was first noticed (0 = none). holdUntil: the end of a snooze.
export const createUpdate = (rev) => ({ seen: rev, since: 0, holdUntil: 0 });

// A server without rev sends none: nothing ever differs then.
export function noticeRev(u, rev, now) {
  if (typeof rev !== 'number' || rev === u.seen) return u;
  return u.since ? u : { ...u, since: now };
}

// The 10 seconds start again at the end of the snooze, so a held press is not cut off the moment it ends.
export const snooze = (u, now) => ({ ...u, since: now + SNOOZE_MS, holdUntil: now + SNOOZE_MS });

export const applied = (u, rev) => createUpdate(rev);

// state: { viewed: a destination is viewed or staged, busy: a take is in flight }.
// none, snoozed (no banner), banner (waiting, leftMs until it goes anyway) or apply.
export function updateStep(u, state, now) {
  if (!u.since) return { step: 'none' };
  if (now < u.holdUntil) return { step: 'snoozed' };
  const leftMs = Math.max(0, u.since + FORCE_MS - now);
  if (state.busy || (state.viewed && leftMs > 0)) return { step: 'banner', leftMs };
  return { step: 'apply' };
}
