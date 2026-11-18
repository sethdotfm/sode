// Confirm lockout; its own module so the lock screen can use it without widgets.

import { S } from './core.js';
import { lockoutMs } from './lockrules.js';

// cfg: a page with no socket hello passes the settings it read over /api/v1.
export function lockout(onReady, cfg = S.cfg) {
  const ms = lockoutMs(cfg);
  const at = performance.now() + ms;
  const t = setTimeout(onReady, ms);
  return { ready: () => performance.now() >= at, cancel: () => clearTimeout(t) };
}
