// The /log uptime graph (#139): turns the server's record of the router link
// into coloured slots. Pure, no imports, so Node tests it. The server sends
// the state at the range's start, the changes in it and, per slot, how many
// messages went badly; this works out each slot's colour.

const MIN = 60_000, HOUR = 60 * MIN, DAY = 24 * HOUR;
// Slot lengths to choose from, shortest first.
const SLOTS = [MIN, 5 * MIN, 15 * MIN, 30 * MIN, HOUR, 3 * HOUR, 6 * HOUR, 12 * HOUR, DAY, 7 * DAY];
// The most slots a row has: 24h is 96 × 15 min, 7 days 168 × 1 h.
export const MAX_SLOTS = 200;
// The toggles for common ranges, shortest first.
export const PRESETS = [
  { id: '1h', label: '1 hour', span: HOUR },
  { id: '24h', label: '24 hours', span: DAY },
  { id: '7d', label: '7 days', span: 7 * DAY },
  { id: '30d', label: '30 days', span: 30 * DAY },
];
export const DEFAULT_PRESET = '24h';
export const presetId = (id) => (PRESETS.some((p) => p.id === id) ? id : DEFAULT_PRESET);

// A slot length as words: "1 min", "15 min", "1 hour", "6 hours".
export function slotLabel(ms) {
  const [n, unit] = ms % HOUR === 0 ? [ms / HOUR, 'hour'] : ms % MIN === 0 ? [ms / MIN, 'min'] : [ms / 1000, 'sec'];
  return `${n} ${unit}${unit === 'hour' && n !== 1 ? 's' : ''}`;
}

// A preset's chip text: the range, then how long each bar is.
export const presetText = (p) => `${p.label} (${slotLabel(slotFor(0, p.span))})`;

// The shortest slot that keeps a range within MAX_SLOTS (ms).
export function slotFor(from, to) {
  const span = Math.max(to - from, MIN);
  return SLOTS.find((s) => span / s <= MAX_SLOTS) || Math.ceil(span / MAX_SLOTS / DAY) * DAY;
}

// Slot colours are statuses: ok (green), warn (yellow), error (red);
// none is no record, future is time that hasn't come.
// data: { start, trans: [{t, s}], counts: [{i, bad, slow}], from, to, slot, now }
export function slots(data) {
  const { trans = [], counts = [], from, to, slot, now } = data;
  const n = Math.max(0, Math.ceil((to - from) / slot));
  const byIndex = new Map(counts.map((c) => [c.i, c]));
  const out = [];
  let k = 0; // next transition to apply
  let state = data.start || '';
  for (let i = 0; i < n; i++) {
    const a = from + i * slot;
    const b = Math.min(a + slot, to);
    const end = Math.min(b, now);
    const c = byIndex.get(i) || { bad: 0, slow: 0 };
    const s = { from: a, to: b, upMs: 0, knownMs: 0, bad: c.bad, slow: c.slow, colour: 'future' };
    if (end > a) {
      while (k < trans.length && trans[k].t <= a) state = trans[k++].s;
      let at = a, cur = state, j = k;
      const span = (until) => {
        if (cur) s.knownMs += until - at;
        if (cur === 'up') s.upMs += until - at;
        at = until;
      };
      for (; j < trans.length && trans[j].t < end; j++) {
        span(trans[j].t);
        cur = trans[j].s;
      }
      span(end);
      s.colour = !s.knownMs ? 'none' : !s.upMs ? 'error' : s.upMs < s.knownMs || s.bad || s.slow ? 'warn' : 'ok';
    }
    out.push(s);
  }
  return out;
}

// How many times the link went from up to not up in the range.
export function drops(data) {
  let prev = data.start || '';
  let n = 0;
  for (const t of data.trans || []) {
    if (prev === 'up' && t.s !== 'up') n++;
    prev = t.s;
  }
  return n;
}

// "99.8% up · 2 drops · 5 failed · 3 slow": status always has text. The
// percentage rounds down, so it only reads 100% when nothing was down.
export function summary(data) {
  const ss = slots(data);
  const known = ss.reduce((t, s) => t + s.knownMs, 0);
  if (!known) return 'No record for this time';
  const up = ss.reduce((t, s) => t + s.upMs, 0);
  const pct = up === known ? '100' : (Math.floor((up / known) * 1000) / 10).toFixed(1);
  const d = drops(data), bad = ss.reduce((t, s) => t + s.bad, 0), slow = ss.reduce((t, s) => t + s.slow, 0);
  const parts = [`${pct}% up`, `${d} ${d === 1 ? 'drop' : 'drops'}`];
  if (bad) parts.push(`${bad} failed`);
  if (slow) parts.push(`${slow} slow`);
  return parts.join(' · ');
}

// What one slot says, for its tooltip and screen readers: when, and why that colour.
export function slotText(s, fmt) {
  const when = `${fmt(s.from)} – ${fmt(s.to)}`;
  if (s.colour === 'future') return `${when}: not yet`;
  if (s.colour === 'none') return `${when}: no record`;
  const pct = Math.floor((s.upMs / s.knownMs) * 1000) / 10;
  const bits = [s.upMs === s.knownMs ? 'router up' : s.upMs ? `router up ${pct}% of the time` : 'router down'];
  if (s.bad) bits.push(`${s.bad} ${s.bad === 1 ? 'message' : 'messages'} failed or needed a retry`);
  if (s.slow) bits.push(`${s.slow} slow ${s.slow === 1 ? 'ACK' : 'ACKs'}`);
  return `${when}: ${bits.join(', ')}`;
}

// The range to draw: the times chosen with From and To if any (From alone
// runs to now; To alone starts a week before it), else the preset back from
// now. null if From is after To.
export function rangeFor(preset, rangeFrom, rangeTo, now) {
  if (rangeFrom == null && rangeTo == null) return { custom: false, from: now - PRESETS.find((p) => p.id === presetId(preset)).span, to: now };
  const to = rangeTo ?? now;
  const from = rangeFrom ?? to - 7 * DAY;
  return from < to ? { custom: true, from, to } : null;
}
