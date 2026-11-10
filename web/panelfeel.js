// Device features a panel can switch on: keep the screen awake (Screen Wake Lock) and haptics (Vibration).
// Both are missing in some browsers (iOS has no vibrate), so the editor asks featureSupport and hides or disables
// the switch, and the panel page does nothing for a switch that is on anyway.

const HAPTIC_MS = 15;

export const featureSupport = (nav) => ({ wake: !!nav?.wakeLock?.request, haptics: typeof nav?.vibrate === 'function' });

// What a switched-on feature cannot do here and why, for the note on the panel page. Wake lock exists only in a
// secure context, so a LAN panel over plain http loses it silently.
export function featureNotes(look, nav, secure) {
  const f = featureSupport(nav);
  const out = [];
  if (look.wake && !f.wake) out.push(secure ? 'This browser cannot keep the screen awake.' : 'The screen cannot be kept awake: the browser allows it only over https, and this page is open over http.');
  if (look.haptics && !f.haptics) out.push('This browser has no vibration, so presses will not buzz (iPhone and iPad browsers never do).');
  return out;
}

export function buzz(nav) {
  return featureSupport(nav).haptics ? nav.vibrate(HAPTIC_MS) !== false : false;
}

// A browser drops the lock when the page hides, so it is asked for again when the page shows.
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

// Browsers only allow fullscreen from a user gesture, so a panel with the setting on asks on its first press.
// landscape asks for the turn lock once fullscreen starts: Chrome only allows it in fullscreen, and iOS never does.
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
