// Device features a panel can switch on: keep the screen awake (Screen Wake Lock) and haptics (Vibration).
// Both are missing in some browsers (iOS has no vibrate), so the editor asks featureSupport and hides or disables
// the switch, and the panel page does nothing for a switch that is on anyway.

// Android motors often cannot start under about 30 ms, so a shorter pulse is dropped silently.
const HAPTIC_MS = 40;

// iOS has no vibrate, but Safari 17.4 and up ticks when a switch input is toggled by a touch, so that stands in.
const iosTouch = (nav) => /iP(hone|ad|od)/.test(nav?.userAgent || '') || (nav?.platform === 'MacIntel' && nav?.maxTouchPoints > 1);

export const featureSupport = (nav) => ({ wake: !!nav?.wakeLock?.request, haptics: typeof nav?.vibrate === 'function' || iosTouch(nav) });

// What a switched-on feature cannot do here and why, for the note on the panel page. Wake lock exists only in a
// secure context, so a LAN panel over plain http loses it silently.
export function featureNotes(look, nav, secure) {
  const f = featureSupport(nav);
  const out = [];
  if (look.wake && !f.wake) out.push(secure ? 'This browser cannot keep the screen awake.' : 'The screen cannot be kept awake: the browser allows it only over https, and this page is open over http.');
  if (look.haptics && !f.haptics) out.push('This browser has no vibration, so presses will not buzz.');
  return out;
}

function tick(doc) {
  if (!doc?.body) return false;
  const label = doc.createElement('label');
  label.setAttribute('aria-hidden', 'true');
  label.style.cssText = 'position:fixed;left:0;top:0;width:1px;height:1px;opacity:0;pointer-events:none';
  const input = doc.createElement('input');
  input.type = 'checkbox';
  input.setAttribute('switch', '');
  input.tabIndex = -1;
  label.append(input);
  doc.body.append(label);
  label.click(); // must run inside the press that asked for it, or Safari gives no tick
  label.remove();
  return true;
}

export function buzz(nav, doc = globalThis.document) {
  if (typeof nav?.vibrate === 'function') return nav.vibrate(HAPTIC_MS) !== false;
  return iosTouch(nav) ? tick(doc) : false;
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
    // Safari may refuse a request made without a touch, so a press asks again.
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
