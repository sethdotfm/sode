// Step in (GLOSSARY.md, docs/agents/users-locks.md): this tab acts as another user until Step out. The server holds it
// per connection. The page keeps it out of the session keys (store.js), so a reload or another tab still sees the original.
import { S, on, emit, onMsg, wsSend } from './core.js';
import { holdSessionKeys } from './store.js';

let asking = false; // a stepIn is on its way: its lockState comes before its result
let stepped = false;
let guest = false; // Step out returns to Guest
let guestLock = false; // a Lock ended a step-in from Guest: the page raises its own Lock

holdSessionKeys(() => asking || stepped);

export const steppedIn = () => stepped;

// lock.js calls this first thing on each lockState, before it writes a key.
export function noteStepState(m) {
  stepped = !!m.stepIn;
  guest = !!m.stepGuest;
}
onMsg('stepInResult', (m) => {
  asking = false;
  emit('stepInResult', m);
});
onMsg('stepEnded', (m) => {
  if (m.why === 'lock' && guest) guestLock = true;
  emit('stepEnded', m);
});
on('conn', () => {
  if (S.wsState === 'up') return;
  asking = stepped = false; // the server ends a step-in with the connection
});
// After lock.js applied the lockState that ended it.
on('access', (L) => {
  if (!guestLock || stepped || L.user) return;
  guestLock = false;
  emit('localLock');
});

export function sendStepIn(user, pin, id) {
  asking = wsSend({ t: 'stepIn', user, pin, id });
  return asking;
}

export const stepOut = () => wsSend({ t: 'stepOut' });

// The Lock while stepped in: the server ends the step-in and locks the original. True when it took the Lock.
export function stepLock() {
  if (!stepped) return false;
  wsSend({ t: 'userLock' });
  return true;
}
