// Idle is per browser: activity in any tab resets every tab's timer, or a background tab would lock the
// shared session (the server holds the Lock for all of it) under someone who is typing. channel is a
// BroadcastChannel or null; tests pass a fake.

// Garbage from the channel is ignored.
export const seenActive = (last, seen) => (Number.isFinite(seen) && seen > last ? seen : last);

// Pointer moves fire constantly: one message a second is plenty against a minutes-long timer.
export const shouldShare = (lastShared, now) => now - lastShared >= 1000;

export function createIdleShare(channel, now = Date.now) {
  let lastActive = now();
  let lastShared = 0;
  if (channel) channel.onmessage = (e) => (lastActive = seenActive(lastActive, e.data));
  return {
    lastActive: () => lastActive,
    touch() {
      const t = now();
      lastActive = t;
      if (channel && shouldShare(lastShared, t)) {
        lastShared = t;
        channel.postMessage(t);
      }
    },
  };
}
