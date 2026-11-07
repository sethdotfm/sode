// Settings PIN unlock shared by Settings and Filters; one flag follows the server's per-connection unlock (until lock, PIN change or socket drop).

import { S, on, wsSend, sentence } from './core.js';
import { needsPin as needsPinRule } from './lockrules.js';

let seq = 0;
let waiting = null; // the unlock in flight: { id, resolve }
let isUnlocked = false;

// unlocked() is the settings PIN only; a trusted user may make low-risk saves without it (needsPin).
// Destructive controls are PIN-only: ask via pinprompt.js before, or after a refusal.
let trustedUser = false;

export const unlocked = () => isUnlocked;
export const trusted = () => trustedUser;
export const needsPin = () => needsPinRule(S.pinSet, isUnlocked, trustedUser);
export const lostUnlock = () => { isUnlocked = false; };

function settle(r) {
  const w = waiting;
  waiting = null;
  w?.resolve(r);
}

// Resolves {ok, err}; stale when overtaken (relock, socket drop with why in err); superseded when a newer unlock's caller owns the button.
export function unlock(pin) {
  settle({ ok: false, stale: true, superseded: true });
  const id = ++seq;
  if (!wsSend({ t: 'unlock', pin, id })) return Promise.resolve({ ok: false, err: 'Not connected to Sode.' });
  return new Promise((resolve) => { waiting = { id, resolve }; });
}

// Also sent with an unlock in flight: the server handles them in order, so the lock wins.
export function relock(force = false) {
  if (force || isUnlocked || waiting) wsSend({ t: 'lock' });
  isUnlocked = false;
  settle({ ok: false, stale: true });
}

on('access', (m) => {
  trustedUser = !!m.trusted && !!m.user && !m.locked;
});

on('unlockResult', (m) => {
  if (!waiting || m.id !== waiting.id) return;
  if (m.ok) isUnlocked = true;
  settle(m.ok ? { ok: true } : { ok: false, err: sentence(m.err) });
});
on('locked', () => { isUnlocked = false; });
on('conn', () => {
  if (S.wsState === 'up') return;
  isUnlocked = false;
  trustedUser = false;
  settle({ ok: false, stale: true, err: 'Not connected to Sode.' });
});
// With no PIN now, one set later (even while disconnected) needs entering.
on('hello', () => { if (!S.pinSet) isUnlocked = false; });
