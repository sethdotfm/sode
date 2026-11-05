// The settings PIN unlock, shared by Settings and Recent's Filters. The
// server unlocks a connection for the PIN entered against, until a lock, a
// PIN change or the socket dropping; one flag here follows that.

import { S, on, wsSend, sentence } from './core.js';
import { needsPin as needsPinRule } from './lockrules.js';

let seq = 0;
let waiting = null; // the unlock in flight: { id, resolve }
let isUnlocked = false;

// unlocked() is the settings PIN only. A user the server calls trusted may
// also make low-risk saves without it (needsPin); the destructive controls
// (users, clear history, changing the PIN) are refused by the server and then
// demandPin() brings the PIN prompt back.
let trustedUser = false;
let forced = false;

export const unlocked = () => isUnlocked;
export const trusted = () => trustedUser;
export const needsPin = () => needsPinRule(S.pinSet, isUnlocked, trustedUser, forced);
export const demandPin = () => { forced = true; };
// The server refused for want of the PIN: this page's unlock is gone (the popup in pinprompt.js asks again).
export const forgetUnlock = () => { isUnlocked = false; };

function settle(r) {
  const w = waiting;
  waiting = null;
  w?.resolve(r);
}

// Sends the PIN. Resolves {ok, err}; stale when overtaken (relock, or the
// socket dropping, which also says why in err), so the caller only needs
// to re-enable what it disabled. superseded too when a newer unlock took
// over: that one's caller has the button now, so leave it be.
export function unlock(pin) {
  settle({ ok: false, stale: true, superseded: true });
  const id = ++seq;
  if (!wsSend({ t: 'unlock', pin, id })) return Promise.resolve({ ok: false, err: 'Not connected to Sode.' });
  return new Promise((resolve) => { waiting = { id, resolve }; });
}

// Locks again. Also sent with an unlock still in flight: the server
// handles them in order, so the lock wins. force: send it regardless.
export function relock(force = false) {
  if (force || isUnlocked || waiting) wsSend({ t: 'lock' });
  isUnlocked = false;
  forced = false;
  settle({ ok: false, stale: true });
}

// lock.js: what the server says about this connection's user.
on('access', (m) => {
  trustedUser = !!m.trusted && !!m.user && !m.locked;
  if (!trustedUser) forced = false;
});

on('unlockResult', (m) => {
  if (!waiting || m.id !== waiting.id) return;
  if (m.ok) { isUnlocked = true; forced = false; }
  settle(m.ok ? { ok: true } : { ok: false, err: sentence(m.err) });
});
on('locked', () => { isUnlocked = false; forced = true; });
on('conn', () => {
  if (S.wsState === 'up') return;
  isUnlocked = false;
  trustedUser = false;
  forced = false;
  settle({ ok: false, stale: true, err: 'Not connected to Sode.' });
});
// With no PIN now, one set later (even while disconnected) needs entering.
on('hello', () => { if (!S.pinSet) isUnlocked = false; });
