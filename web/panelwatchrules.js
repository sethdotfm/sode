// Monitoring text and timing (docs/panels-design.md, Monitoring). Pure.

export const KEEPALIVE_MS = 15000;
export const RETRY_MIN_MS = 1000;
export const RETRY_MAX_MS = 10000;

// The held connection that lets Sode see a closed page at once (the keepalive stays as the fallback).
export const streamPath = (id, device) => `api/v1/panels/${id}/instances/stream?device=${encodeURIComponent(deviceFrom(device))}`;

// Wait before reopening it after n failed or ended tries in a row.
export const retryMs = (n) => Math.min(RETRY_MAX_MS, RETRY_MIN_MS * 2 ** Math.max(0, n));
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
    text: lostText(n.device, n.secs),
  };
}

// The line of a lost notice; the toast calls it again every second with the time since the notice came.
export const lostText = (device, secs) => `${device} has not been heard from for ${durationText(secs)}.`;

// An alarm row with an `instance` is a watched screen, not a whole panel.
const screens = (rows) => rows.length > 0 && rows.every((r) => r.instance);
export const badgeText = (rows) => {
  const noun = screens(rows) ? 'screen' : 'panel';
  if (rows.some((r) => r.instance) && !screens(rows)) return `${rows.length} offline`;
  return rows.length === 1 ? `1 ${noun} offline` : `${rows.length} ${noun}s offline`;
};

export const deviceFrom = (s) =>
  String(s || '')
    .trim()
    .slice(0, DEVICE_MAX);

// Sent even with the page in front, so the person gets the same alert on every screen they watch from.
export const shouldNotify = (permission) => permission === 'granted';

export const keepaliveBody = (device) => JSON.stringify({ device: deviceFrom(device) });

// Headers that tie a page's /api/v1 call to its panel and screen for Communication Log rows (docs/agents/comm-log.md).
// The server trusts them only as the page's own word; a panel's own key proves the panel by itself.
export function fromPanel(init = {}, id, screen) {
  if (!id) return init;
  const headers = { ...init.headers, 'X-Sode-Panel': id };
  if (screen) headers['X-Sode-Instance'] = deviceFrom(screen);
  return { ...init, headers };
}

// What the browser allows. The Notification API is missing in some browsers and over plain HTTP.
export function permissionText(permission) {
  if (permission === 'granted') return { state: 'on', text: 'Browser notifications are on.' };
  if (permission === 'denied') {
    return {
      state: 'blocked',
      text: 'Browser notifications are blocked for this site. Allow them in the browser site settings to get alerts outside this tab.',
    };
  }
  if (permission === 'default') return { state: 'ask', text: 'Browser notifications are not allowed yet.' };
  return { state: 'none', text: 'This browser cannot show notifications here, so alerts show in Sode only.' };
}

// Words for the Clear menu: one panel or several.
export const clearNote = (rows) => ({ stop: rows.length === 1 ? 'Stop monitoring this panel' : 'Stop monitoring these panels' });

// Banner line for the panels that are offline and not cleared.
export function bannerText(rows) {
  if (rows.length === 1) {
    const [r] = rows;
    return { title: r.instance ? `${r.name} lost ${r.device}` : `${r.name} is offline`, text: `${r.device} stopped responding.` };
  }
  const label = (r) => (r.instance ? `${r.name}: ${r.device}` : r.name);
  const noun = screens(rows) ? 'screens' : rows.some((r) => r.instance) ? 'watched panels and screens' : 'panels';
  return { title: `${rows.length} ${noun} are offline`, text: rows.map(label).join(', ') };
}

// What a rename asks for: the new name, or empty when there is nothing to change (blank or the same name).
export function renameTo(current, input) {
  const name = deviceFrom(input);
  return name && name !== current ? name : '';
}
