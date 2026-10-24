// The settings PIN unlock, shared by Settings and Recent's Filters. The
// server unlocks a connection for the PIN entered against, until a lock, a
// PIN change or the socket dropping; one flag here follows that.

import { S, on, wsSend, sentence } from './core.js';

let seq = 0;
let waiting = null; // the unlock in flight: { id, resolve }
let isUnlocked = false;

export const unlocked = () => isUnlocked;

function settle(r) {
  const w = waiting;
  waiting = null;
  w?.resolve(r);
}

// Sends the PIN. Resolves {ok, err}; stale when overtaken (a newer unlock,
// relock, or the socket dropping, which also says why in err), so the
// caller only needs to re-enable what it disabled.
export function unlock(pin) {
  settle({ ok: false, stale: true });
  const id = ++seq;
  if (!wsSend({ t: 'unlock', pin, id })) return Promise.resolve({ ok: false, err: 'Not connected to Sode.' });
  return new Promise((resolve) => { waiting = { id, resolve }; });
}

// Locks again. Also sent with an unlock still in flight: the server
// handles them in order, so the lock wins. force: send it regardless.
export function relock(force = false) {
  if (force || isUnlocked || waiting) wsSend({ t: 'lock' });
  isUnlocked = false;
  settle({ ok: false, stale: true });
}

on('unlockResult', (m) => {
  if (!waiting || m.id !== waiting.id) return;
  if (m.ok) isUnlocked = true;
  settle(m.ok ? { ok: true } : { ok: false, err: sentence(m.err) });
});
on('locked', () => { isUnlocked = false; });
on('conn', () => {
  if (S.wsState === 'up') return;
  isUnlocked = false;
  settle({ ok: false, stale: true, err: 'Not connected to Sode.' });
});
// With no PIN now, one set later (even while disconnected) needs entering.
on('hello', () => { if (!S.pinSet) isUnlocked = false; });
