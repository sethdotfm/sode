// Confirm lockout; its own module so the lock screen can use it without widgets.

import { S } from './core.js';

export function lockout(onReady) {
  const ms = S.cfg.confirmLockoutMs;
  const at = performance.now() + ms;
  const t = setTimeout(onReady, ms);
  return { ready: () => performance.now() >= at, cancel: () => clearTimeout(t) };
}
