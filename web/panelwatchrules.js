// Monitoring text and timing (docs/panels-design.md, Monitoring). Pure.

export const KEEPALIVE_MS = 15000;
export const DEVICE_MAX = 40;

export function durationText(secs) {
  const s = Math.max(0, Math.round(secs));
  const plural = (n, w) => `${n} ${w}${n === 1 ? '' : 's'}`;
  if (s < 60) return plural(s, 'second');
  if (s < 3600) {
    const m = Math.floor(s / 60);
    return plural(m, 'minute') + (s % 60 ? ` ${plural(s % 60, 'second')}` : '');
  }
  if (s < 86400) {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    return plural(h, 'hour') + (m ? ` ${plural(m, 'minute')}` : '');
  }
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  return plural(d, 'day') + (h ? ` ${plural(h, 'hour')}` : '');
}

// A notice names the panel, the device and how long it was missing. offline says the panel has no screen left.
export function noticeText(n) {
  const dur = durationText(n.secs);
  if (n.state === 'back') {
    return {
      kind: 'ok',
      title: n.offline ? `${n.name} is back` : `${n.name}: ${n.device} is back`,
      text: `${n.device} is back after ${dur}.`,
    };
  }
  return {
    kind: n.offline ? 'error' : 'warn',
    title: n.offline ? `${n.name} is offline` : `${n.name} lost a screen`,
    text: `${n.device} has not been heard from for ${dur}.`,
  };
}

export const badgeText = (rows) => (rows.length === 1 ? '1 panel offline' : `${rows.length} panels offline`);

export const deviceFrom = (s) =>
  String(s || '')
    .trim()
    .slice(0, DEVICE_MAX);

// A browser notification is only worth sending when the page is not in front of the person.
export const shouldNotify = (permission, hidden, focused) => permission === 'granted' && (hidden || !focused);

export const keepaliveBody = (device) => JSON.stringify({ device: deviceFrom(device) });
