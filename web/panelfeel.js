// Wake lock is missing in some browsers and over plain http, so the editor disables the switch and the page ignores it.

export const featureSupport = (nav) => ({ wake: !!nav?.wakeLock?.request });

// Wake lock exists only in a secure context, so a LAN panel over plain http loses it silently.
export function featureNotes(look, nav, secure) {
  const out = [];
  if (look.wake && !featureSupport(nav).wake) out.push(secure ? 'This browser cannot keep the screen awake.' : 'Screen wake needs https.');
  return out;
}

// A browser drops the lock when the page hides, so the page asks for it again when it shows.
export function createWake(nav, doc) {
  let sentinel = null;
  let wanted = false;
  const request = async () => {
    if (!wanted || sentinel || doc.visibilityState !== 'visible' || !featureSupport(nav).wake) return;
    try {
      const s = await nav.wakeLock.request('screen');
      s.addEventListener?.('release', () => {
        if (sentinel === s) sentinel = null;
      });
      sentinel = s;
    } catch {
      sentinel = null; // refused (battery saver, no permission): the panel still works
    }
  };
  return {
    held: () => !!sentinel,
    // Safari can refuse a request made without a touch, so a press asks again.
    retry: request,
    async on() {
      if (wanted) return;
      wanted = true;
      doc.addEventListener('visibilitychange', request);
      await request();
    },
    async off() {
      wanted = false;
      doc.removeEventListener('visibilitychange', request);
      const s = sentinel;
      sentinel = null;
      await s?.release();
    },
  };
}

// Chrome locks the turn only in fullscreen, so landscape on makes the fullscreen switch moot.
export const wantsFullscreen = (look) => !!(look?.fullscreen || look?.landscape);

export const LANDSCAPE_HINT = 'A press also enters fullscreen, since browsers lock the turn only there.';

export const fullscreenNote = (s) =>
  s?.landscape === true && s?.fullscreen !== true ? 'Enforce landscape orientation is on, so a press still enters fullscreen.' : '';

// Browsers allow fullscreen only from a user gesture. The turn lock follows fullscreen: iOS never allows it.
export function enterFullscreen(doc, landscape = false, scr = globalThis.screen) {
  const root = doc?.documentElement;
  if (doc?.fullscreenElement || !doc?.fullscreenEnabled || !root?.requestFullscreen) return false;
  const lock = () => landscape && scr?.orientation?.lock?.('landscape')?.catch?.(() => {});
  root.requestFullscreen().then(lock, () => {}); // refused (iOS Safari, an embedded frame): the panel still works
  return true;
}

export function exitFullscreen(doc) {
  if (!doc?.fullscreenElement) return false;
  doc.exitFullscreen().catch(() => {});
  return true;
}
