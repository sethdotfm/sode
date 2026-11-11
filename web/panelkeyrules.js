// Pure rules for a panel's Link with key tab and for a panel page that opens with a key or a setup link.
// No DOM, no clock, no network, so Node tests them. Server side: internal/app/panelkey.go, panelsetup.go, panelnet.go.
import { fullSetup } from './panellink.js';
import { whenText } from './keyrules.js';
import { hasClient, isPublic } from './panelsrules.js';
import { renewable } from './sessionkeyrules.js';

export const MODES = [
  ['key', 'Named instance link'],
  ['setup', 'One-time setup link'],
];

export const MODE_HINT = {
  key: 'Makes a key for this instance. The link shows once, so copy it when it appears. Anyone with the link can use the panel as this instance. To get a link again, rotate the instance.',
  setup:
    "Makes a link that works once, for one hour. The device opens it and keeps its own key, so the link cannot be reused. This instance's old key ends now.",
};

// The instance name is required and becomes the device name (the page's ?instance=), so it has the monitor's limits.
export const INSTANCE_MAX = 40; // characters, as monDeviceMax in internal/app/panelmon.go
export const INSTANCE_HOLD = 'Name the instance, such as Lobby tablet.';

// What a typed name means against the instances the panel already holds (names ignore case). max is the server's cap.
export function instanceCheck(text, keys, max) {
  const name = String(text || '').trim();
  const held = keys.find((k) => k.name.toLowerCase() === name.toLowerCase()) || null;
  if (!name) return { name, held, error: INSTANCE_HOLD };
  if ([...name].length > INSTANCE_MAX) return { name, held, error: `Use at most ${INSTANCE_MAX} characters.` };
  if (!held && keys.length >= max) return { name, held, error: `A panel has at most ${max} instances. Revoke one first.` };
  return { name, held, error: '' };
}

// The server answers a link relative to Sode's root. Resolving against the page's base URL keeps it right at
// /panels and /panels/ and behind a path prefix, since both pages set <base href="../">.
export const absoluteLink = (link, base) => new URL(link, base).href;

// The secret in the fragment, hidden until the owner asks to see it.
export const maskLink = (href) => href.replace(/(#(?:key|setup)=)[^&]+/, '$1••••••••');

// What the make button says and whether it needs a Confirm first. Making a link for a name that holds a key or a
// pending link ends that instance's, so it asks then.
export function makeAction(mode, held) {
  if (mode === 'setup') return { label: held ? 'Replace with setup link' : 'Make setup link', confirm: !!held, verb: 'Replace the key' };
  return held ? { label: 'Rotate this instance', confirm: true, verb: 'Rotate' } : { label: 'Make link', confirm: false, verb: 'Make' };
}

export const CONFIRM_TEXT = {
  rotate: "Rotating ends this instance's key. That instance signs out and its old link stops working. Other instances are not affected.",
  setup: "A setup link replaces this instance's key. That instance signs out and its old link stops working. Other instances are not affected.",
  revoke: 'Revoking ends this instance. It signs out and its link stops working. Other instances are not affected.',
};

export const confirmKind = (mode, which) => (which === 'revoke' ? 'revoke' : mode === 'setup' ? 'setup' : 'rotate');

function whenUntil(iso, now) {
  const m = Math.max(0, Math.round((Date.parse(iso) - now) / 60000));
  return m < 1 ? 'in under a minute' : `in ${m} minute${m === 1 ? '' : 's'}`;
}

// One row of the instance list. state drives the dot and its word: a status always has text.
export function keyRow(k, now) {
  const state = k.setup ? { kind: 'idle', text: 'Setup link waiting' } : k.online ? { kind: 'ok', text: 'Online' } : { kind: 'idle', text: 'Offline' };
  const used = k.setup ? `Expires ${whenUntil(k.expires, now)}` : `Last used ${whenText(k.lastUsed, now, 'never')}`;
  return { id: k.id, name: k.name, state, created: `Made ${whenText(k.created, now, 'just now')}`, used };
}

// The Link with key tab on a panel that acts as the signed-in user: no key exists, and a link opened in another
// tab asks for a sign-in. Said plainly, and the key actions are not offered.
export const SESSION_WARNING =
  'This panel acts as the signed-in user, so it has no key. A link to it asks for a sign-in in any tab or on any device that is not already signed in to Sode.';
export const SESSION_NEXT =
  'For a link that opens without a sign-in, set Authentication mode to API, Both or Public on the Access tab. That saves the panel at once.';

// A Public panel has a client, but its plain link already opens it, so keyed links add nothing.
export const keyTabMuted = (panel) => !hasClient(panel) || isPublic(panel);

export const PUBLIC_KEY_NOTE = 'This is a Public panel, so it needs no key. Anyone with the link below can use it, with no sign-in.';
export const PUBLIC_NEXT = 'To limit who may open it, set Authentication mode to User, Both or API on the Access tab.';

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

// ---- the panel page

// A one-time setup link in the fragment; the page trades it for a key and drops the fragment.
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

// The hash without its setup part, so a reload or a copied address no longer carries the spent token.
export const withoutSetup = (hash) => withoutPart(hash, 'setup');

// The hash without its link key: one the browser stored, or a stale one on a Public panel, which then runs on its own
// short-lived key. The address, history and a copied URL hold no key.
export const withoutKey = (hash) => withoutPart(hash, 'key');

export function setupFailText(status, message) {
  if (status === 401) return 'This setup link was already used, has expired or was replaced. Ask for a new one.';
  if (status === 403 && message) return message;
  return 'The setup link could not be used. Try again, or ask for a new link.';
}

// What a polling failure says. A key the server no longer knows ends the page: only a new link helps. Without a
// key the page runs on a session, which sessionkey.js renews; an ended answer that still reaches the page means
// the renewal could not help. A 429 is not lost contact: the fetcher already waits out Retry-After.
export function pollFail(status, code, message, hadKey) {
  if (status === 401 && hadKey) return { stop: true, text: "This link's key no longer works. It was rotated or revoked, so ask for a new link." };
  if (status === 401 && (code === 'key_revoked' || code === 'key_expired'))
    return { stop: true, text: "This page's sign-in key was ended. Reload the page to sign in again." };
  if (status === 403 && code === 'network_not_allowed') return { stop: true, text: message || 'This panel may not be used from this network.' };
  if (status === 429) return { stop: false, text: 'Sode asked this page to wait before it tries again.' };
  return { stop: false, text: 'Lost contact with Sode. Retrying.' };
}

const codeOf = (res) =>
  res
    .clone()
    .json()
    .then((b) => b?.error?.code)
    .catch(() => undefined);

// A Public panel's page key lives 15 minutes (internal/app/panelpublic.go). send(path, init, key) makes the call and
// ask() gets a new key, '' when the panel is no longer Public. A call refused as ended asks once and goes again;
// calls refused together share that one ask, and a call that went out with an old key just uses the new one.
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
