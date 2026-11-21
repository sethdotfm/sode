// A panel page's Offline cover, and the wait at open. A page that opened while Sode was down (the service worker's
// copy, sw.js) shows the cover and asks again every 2 s. So it goes live as soon as Sode answers, with no reload.
import { offlineOf } from './panelrules.js';

const WAIT_MS = 2000;

// o: offlineOf's words, or null for online. The grid is blurred so nobody mistakes a stale grid for a working one.
export function paintOffline(ui, o) {
  ui.main.classList.toggle('sode-panels--offline', !!o);
  ui.offline.hidden = !o;
  if (o) ui.offlineDetail.textContent = o.detail;
  // The Lock reads it: offline wins over its colors (lock.css).
  document.documentElement.dataset.conn = o ? 'down' : 'up';
}

// healthz is open to every caller and the worker never keeps it, so an answer means Sode itself is up. A proxy's 5xx
// while Sode is down is no answer.
export async function waitForSode(ui, stopped, ask = () => fetch('healthz', { cache: 'no-store' })) {
  let shown = false;
  while (!stopped()) {
    try {
      if ((await ask()).ok) break;
    } catch {
      // No contact: ask again.
    }
    if (!shown) paintOffline(ui, offlineOf({ failed: true, routerUp: true }));
    shown = true;
    await new Promise((done) => setTimeout(done, WAIT_MS));
  }
  if (shown) paintOffline(ui, null);
}
