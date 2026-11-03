// The confirm lockout: a short wait before a confirm's key or button works,
// so a stray keystroke cannot answer it. Its own module (like dialogkeys.js)
// so the lock screen can use it without the widgets.

import { S } from './core.js';

// ready() once the lockout (S.cfg.confirmLockoutMs) has passed; onReady
// then, unless cancelled first (the confirm closed).
export function lockout(onReady) {
  const ms = S.cfg.confirmLockoutMs;
  const at = performance.now() + ms;
  const t = setTimeout(onReady, ms);
  return { ready: () => performance.now() >= at, cancel: () => clearTimeout(t) };
}
