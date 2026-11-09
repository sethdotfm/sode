// When an open panel takes a changed copy of itself. The server moves the panel's rev on every stored
// change (a saved edit, a Push); the page polls it and asks this module what to do. No DOM and no
// clock, so the rules test directly (test/panelupdate.test.mjs). Every function returns a new state.
export const COUNT_MS = 10000;
export const SNOOZE_MS = 60000;

// seen: the rev on screen. since: when a newer one was first noticed (0 = none). holdUntil: the end of a snooze.
export const createUpdate = (rev) => ({ seen: rev, since: 0, holdUntil: 0 });

// A server without rev sends none: nothing ever differs then.
export function noticeRev(u, rev, now) {
  if (typeof rev !== 'number' || rev === u.seen) return u;
  return u.since ? u : { ...u, since: now };
}

// The countdown starts again at the end of the snooze, so a held press is not cut off the moment it ends.
export const snooze = (u, now) => ({ ...u, since: now + SNOOZE_MS, holdUntil: now + SNOOZE_MS });

export const applied = (u, rev) => createUpdate(rev);

// state: { staged: routes are staged, busy: a take is in flight }. Staged routes belong to the old copy and are
// lost on apply, so they hold it past the countdown. A viewed destination alone is carried over (panelmodel.carryView).
// none, snoozed (no banner), banner (leftMs on the countdown, wait names what holds it at 0) or apply.
export function updateStep(u, state, now) {
  if (!u.since) return { step: 'none' };
  if (now < u.holdUntil) return { step: 'snoozed' };
  const leftMs = Math.max(0, u.since + COUNT_MS - now);
  const wait = state.busy ? 'busy' : state.staged ? 'staged' : '';
  if (leftMs > 0) return { step: 'banner', leftMs, wait: '' };
  return wait ? { step: 'banner', leftMs: 0, wait } : { step: 'apply' };
}

// The banner's line for a banner step.
export function bannerLine(s) {
  if (s.wait === 'busy') return 'It refreshes after the take.';
  if (s.wait === 'staged') return 'It refreshes once you take or clear your staged routes.';
  return `Refreshing in ${Math.ceil(s.leftMs / 1000)} s.`;
}
