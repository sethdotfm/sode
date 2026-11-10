// When an open panel takes a changed copy of itself. The server moves the panel's rev on every stored
// change (a saved edit, a Push); the page polls it and asks this module what to do. No DOM and no
// clock, so the rules test directly (test/panelupdate.test.mjs). Every function returns a new state.
export const COUNT_MS = 10000;
export const SNOOZE_MS = 60000;

// seen: the rev on screen. since: when a newer one was first noticed (0 = none). from and deadline bound the
// countdown that is running. force: Snooze or Apply now was pressed, so staged routes no longer hold the copy.
// snoozed: the one Snooze this copy gets was used.
export const createUpdate = (rev) => ({ seen: rev, since: 0, from: 0, deadline: 0, force: false, snoozed: false });

// What the page compares: the panel's rev and the Default artwork its read carries, since a changed default moves no
// panel rev. undefined for a server without rev.
export const revOf = (p) => (typeof p?.rev === 'number' ? `${p.rev}|${JSON.stringify(p.artDefaults || [])}` : undefined);

// A server without rev sends none: nothing ever differs then.
export function noticeRev(u, rev, now) {
  if ((typeof rev !== 'number' && typeof rev !== 'string') || rev === u.seen) return u;
  return u.since ? u : { ...u, since: now, from: now, deadline: now + COUNT_MS };
}

// Snooze leaves the banner up and counts 60 s down. Pressing it is the person's word that the copy may come
// then, staged routes or not. An open page snoozes once; applying starts a fresh state, so the next update gets one again.
export const canSnooze = (u) => !u.snoozed;
export const snooze = (u, now) => (u.snoozed ? u : { ...u, from: now, deadline: now + SNOOZE_MS, force: true, snoozed: true });

export const applyNow = (u, now) => ({ ...u, from: now, deadline: now, force: true });

export const applied = (u, rev) => createUpdate(rev);

// state: { staged: routes are staged, busy: a take is in flight }. Staged routes belong to the old copy and are
// lost on apply, so the plain countdown holds for them; a forced one does not. A take in flight is never cut
// short: the copy comes once it answers. A viewed destination alone is carried over (panelmodel.carryView).
// Steps: none, banner (leftMs and totalMs for the bar, deadline keys the bar, wait names what holds it at 0,
// drops says staged routes go when it applies) or apply.
export function updateStep(u, state, now) {
  if (!u.since) return { step: 'none' };
  const leftMs = Math.max(0, u.deadline - now);
  const banner = (wait) => ({ step: 'banner', leftMs, totalMs: u.deadline - u.from, deadline: u.deadline, wait, drops: u.force && !!state.staged });
  if (leftMs > 0) return banner('');
  if (state.busy) return banner('busy');
  if (state.staged && !u.force) return banner('staged');
  return { step: 'apply' };
}

// The banner's line for a banner step.
export function bannerLine(s) {
  if (s.wait === 'busy') return 'It refreshes after the take.';
  if (s.wait === 'staged') return 'It refreshes once you take or clear your staged routes.';
  const secs = Math.ceil(s.leftMs / 1000);
  return s.drops ? `Refreshing in ${secs} s, dropping the staged routes.` : `Refreshing in ${secs} s.`;
}
