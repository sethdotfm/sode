// API key lists and requests, pure (no imports) so Node tests them. A key record is what keyserve.go sends:
// {id, owner: {kind: 'user'|'client', name}, name (the key name), created, lastUsed, count, expires}.
// Go sends the zero time (year 1) for "never", so any date before 1970 means unset.

const UNSET = (iso) => !iso || Date.parse(iso) < 0;

const UNITS = [
  [86_400_000, 'day'],
  [3_600_000, 'hour'],
  [60_000, 'minute'],
];
const span = (ms) => {
  for (const [size, word] of UNITS) {
    const n = Math.floor(ms / size);
    if (n >= 1) return `${n} ${word}${n === 1 ? '' : 's'}`;
  }
  return '';
};

export function ago(ms, now) {
  const s = span(now - ms);
  return s ? `${s} ago` : 'just now';
}

export const whenText = (iso, now, never) => (UNSET(iso) ? never : ago(Date.parse(iso), now));

export function keyFacts(k, now) {
  const exp = UNSET(k.expires) ? 0 : Date.parse(k.expires);
  const expired = exp > 0 && exp <= now;
  const n = k.count || 0;
  return {
    name: k.name,
    created: whenText(k.created, now, 'unknown'),
    used: UNSET(k.lastUsed) ? 'never used' : `used ${whenText(k.lastUsed, now, '')} (${n} ${n === 1 ? 'request' : 'requests'})`,
    expires: !exp ? 'never expires' : expired ? 'expired' : `expires in ${span(exp - now) || 'under a minute'}`,
    expired,
  };
}

// The date field gives yyyy-mm-dd; the key ends at the close of that day, local time.
export function expiryFrom(text, now) {
  if (!text) return { ms: 0 };
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(text);
  if (!m) return { error: 'Pick the expiry date again.' };
  const ms = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 23, 59, 0).getTime();
  return ms > now ? { ms } : { error: 'The expiry date is already past.' };
}

export const keyNameProblem = (name) => (name.trim() ? '' : 'Give the key a key name, such as "Panel 21".');

export const createReq = ({ name, pin, expires, id }) => ({ t: 'keyCreate', id, keyName: name.trim(), pin, expires });

export const ownerText = (o) => `${o.name} (${o.kind === 'client' ? 'API client' : 'user'})`;

export function keysByOwner(keys) {
  const groups = new Map();
  for (const k of keys) {
    const id = `${k.owner.kind}:${k.owner.name.toLowerCase()}`;
    if (!groups.has(id)) groups.set(id, { owner: k.owner, keys: [] });
    groups.get(id).keys.push(k);
  }
  return [...groups.values()];
}
