// Device features a panel can switch on: keep the screen awake (Screen Wake Lock) and haptics (Vibration).
// Both are missing in some browsers (iOS has no vibrate), so the editor asks featureSupport and hides or disables
// the switch, and the panel page does nothing for a switch that is on anyway.

const HAPTIC_MS = 15;

export const featureSupport = (nav) => ({ wake: !!nav?.wakeLock?.request, haptics: typeof nav?.vibrate === 'function' });

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
