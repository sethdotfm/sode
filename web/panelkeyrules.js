// Pure rules for a panel's Link with key tab and for a panel page that opens with a key or a setup link.
// No DOM, no clock, no network, so Node tests them. Server side: internal/app/panelkey.go, panelsetup.go, panelnet.go.
import { fullSetup } from './panellink.js';
import { whenText } from './keyrules.js';
import { hasClient, isPublic } from './panelsrules.js';
import { renewable } from './sessionkeyrules.js';
import { KEY_ENDED, SIGN_IN_ENDED } from './panelrules.js';

// Fixed and free of any reason or name: anyone can read a Public panel's page.
export const PUBLIC_STOPPED = 'Not Public now. Reconnects when Public again.';

export const MODES = [
  ['key', 'Link for one instance'],
  ['setup', 'Setup link (1 hour)'],
];

// The instance name becomes the device name (the page's ?instance=), so it has the monitor's limits.
export const INSTANCE_MAX = 40; // characters, as monDeviceMax in internal/app/panelmon.go
export const INSTANCE_HOLD = 'Name the instance.';

// Names ignore case. max is the server's cap.
export function instanceCheck(text, keys, max) {
  const name = String(text || '').trim();
  const held = keys.find((k) => k.name.toLowerCase() === name.toLowerCase()) || null;
  if (!name) return { name, held, error: INSTANCE_HOLD };
  if (!held && keys.length >= max) return { name, held, error: `At most ${max} instances. Revoke one first.` };
  return { name, held, error: '' };
}

// The server answers relative to Sode's root; both pages set <base href="../">, so this holds at /panels, /panels/ and behind a path prefix.
export const absoluteLink = (link, base) => new URL(link, base).href;

export const maskLink = (href) => href.replace(/(#(?:key|setup)=)[^&]+/, '$1••••••••');

// A new link for a name that holds a key or a pending link ends that instance's, so it needs a Confirm.
export function makeAction(mode, held) {
  if (mode === 'setup') return { label: held ? 'Replace with setup link' : 'Make setup link', confirm: !!held, verb: 'Replace the key' };
  return held ? { label: 'Rotate this instance', confirm: true, verb: 'Rotate' } : { label: 'Make link', confirm: false, verb: 'Make' };
}

export const CONFIRM_TEXT = "Ends this instance's key. Its page signs out and its old link stops.";

export const confirmKind = (mode, which) => (which === 'revoke' ? 'revoke' : mode === 'setup' ? 'setup' : 'rotate');

function whenUntil(iso, now) {
  const m = Math.max(0, Math.round((Date.parse(iso) - now) / 60000));
  return m < 1 ? 'in under a minute' : `in ${m} minute${m === 1 ? '' : 's'}`;
}

export function keyRow(k, now) {
  const state = k.setup ? { kind: 'idle', text: 'Setup link waiting' } : k.online ? { kind: 'ok', text: 'Online' } : { kind: 'idle', text: 'Offline' };
  const used = k.setup ? `Expires ${whenUntil(k.expires, now)}` : whenText(k.lastUsed, now, '') ? `Last used ${whenText(k.lastUsed, now, '')}` : 'Never used';
  return { id: k.id, name: k.name, state, created: `Made ${whenText(k.created, now, 'less than a minute ago')}`, used };
}

// The Link with key tab on a panel that acts as the signed-in user. No key exists, and a link opened in another
// tab asks for a sign-in. Said plainly, and the key actions are not offered.
export const SESSION_WARNING =
  'This panel acts as the signed-in user, so it has no key. A link to it asks for a sign-in in any tab or device not already signed in to Sode.';
export const SESSION_NEXT =
  'For a link that opens without a sign-in, set Authentication mode to API, Both or Public on the Access tab. That saves the panel immediately.';

// A Public panel has a client, but its plain link already opens it, so keyed links add nothing.
export const keyTabMuted = (panel) => !hasClient(panel) || isPublic(panel);

export const PUBLIC_KEY_NOTE = 'This is a Public panel, so it needs no key. Anyone with the link below can use it, with no sign-in.';
export const PUBLIC_NEXT = 'To limit who can open it, set Authentication mode to User, Both or API on the Access tab.';

// The Share modal's first tab: the one that fits how the panel acts.
export const firstShareTab = (panel) => (keyTabMuted(panel) ? 'people' : 'key');

// One address or CIDR per line or comma. The server validates and canonicalizes.
export const parseNetworks = (text) =>
  text
    .split(/[\s,;]+/)
    .map((s) => s.trim())
    .filter(Boolean);

export const networksText = (list) => (list || []).join('\n');

export const networksChanged = (saved, text) => JSON.stringify(parseNetworks(networksText(saved))) !== JSON.stringify(parseNetworks(text));

// The page trades a setup link for a key and drops the fragment.
export const setupFromHash = (hash) => fullSetup(/(?:^#|&)setup=([^&]+)/.exec(hash)?.[1] || '');

// A key the page keeps after a setup link. localStorage name, kept per panel; docs/persisted-names.md.
export const storedKeyName = (id) => `sode-panel-key-${id}`;

function withoutPart(hash, name) {
  const rest = hash
    .replace(/^#/, '')
    .split('&')
    .filter((p) => p && !p.startsWith(`${name}=`));
  return rest.length ? `#${rest.join('&')}` : '';
}

// So a reload or a copied address no longer carries the spent token.
export const withoutSetup = (hash) => withoutPart(hash, 'setup');

// A stored key, or a stale one on a Public panel, which then runs on its own short-lived key.
export const withoutKey = (hash) => withoutPart(hash, 'key');

export function setupFailText(status, message) {
  if (status === 401) return 'Setup link used, expired or replaced. Ask for a new one.';
  if (status === 403 && message) return message;
  return 'Setup link failed. Try again, or ask for a new link.';
}

// A dead key ends the page: only a new link helps. Without a key the page runs on a session that sessionkey.js renews, so an
// ended answer here means the renewal could not help. A 429 is not lost contact: the fetcher waits out Retry-After.
// forget: only a 401 ends a stored key; a refused network leaves it, since the key works again once the list is fixed.
export function pollFail(status, code, message, hadKey, isPublic = false) {
  // Sode did not renew the Public key: the panel left Public, or Public panels allowed went off.
  if (status === 401 && isPublic) return { stop: true, forget: false, text: PUBLIC_STOPPED };
  if (status === 401 && hadKey) return { stop: true, forget: true, text: KEY_ENDED };
  if (status === 401 && (code === 'key_revoked' || code === 'key_expired')) return { stop: true, forget: true, text: SIGN_IN_ENDED };
  if (status === 403 && code === 'network_not_allowed') return { stop: true, forget: false, text: message || 'This panel is not allowed on this network.' };
  if (status === 429) return { stop: false, text: 'Sode is busy. Retrying.' };
  return { stop: false, text: 'Lost contact with Sode. Retrying.' };
}

const codeOf = (res) =>
  res
    .clone()
    .json()
    .then((b) => b?.error?.code)
    .catch(() => undefined);

// A Public panel's page key lives 15 minutes (internal/app/panelpublic.go). ask() gets a new key, '' when the panel is no
// longer Public. A call refused as ended asks once and goes again; calls refused together share that ask.
export function publicFetch(first, { ask, send }) {
  let key = first;
  let asking = null;
  return async (path, init) => {
    const used = key;
    const res = await send(path, init, used);
    if (res.status !== 401 || !renewable(res.status, await codeOf(res))) return res;
    if (key === used) {
      asking ||= ask().finally(() => (asking = null));
      const next = await asking;
      if (!next) return res;
      if (key === used) key = next;
    }
    return send(path, init, key);
  };
}
