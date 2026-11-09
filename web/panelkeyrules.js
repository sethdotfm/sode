// Pure rules for a panel's Link with key tab and for a panel page that opens with a key or a setup link.
// No DOM, no clock, no network, so Node tests them. Server side: internal/app/panelkey.go, panelsetup.go, panelnet.go.
import { fullSetup } from './panellink.js';

export const MODES = [
  ['key', 'Standing link'],
  ['setup', 'One-time setup link'],
];

export const MODE_HINT = {
  key: 'The key is in the link, so anyone who has it can use this panel. Rotate or Revoke ends it.',
  setup: 'The link works once, for one hour. The page then keeps its own key in that browser and the link is spent.',
};

// The server answers a link relative to Sode's root. Resolving against the page's base URL keeps it right at
// /panels and /panels/ and behind a path prefix, since both pages set <base href="../">.
export const absoluteLink = (link, base) => new URL(link, base).href;

export const kioskLink = (href) => `${href}&kiosk=1`;

// The secret in the fragment, hidden until the owner asks to see it.
export const maskLink = (href) => href.replace(/(#(?:key|setup)=)[^&]+/, '$1••••••••');

// What the make button says and whether it needs a Confirm first. Making a link replaces the panel's key, so it
// asks whenever a key exists.
export function makeAction(mode, hasKey) {
  if (mode === 'setup') return { label: 'Make setup link', confirm: hasKey, verb: 'Replace the key' };
  return hasKey ? { label: 'Rotate key', confirm: true, verb: 'Rotate' } : { label: 'Make link', confirm: false, verb: 'Make' };
}

export const CONFIRM_TEXT = {
  rotate: 'Rotating ends the current key. Every open instance of this panel signs out and every old link stops working.',
  setup: 'A setup link replaces the current key. Every open instance of this panel signs out and every old link stops working.',
  revoke: 'Revoking ends the key. Every open instance of this panel signs out, and the panel stays without a key until you make one.',
};

export const confirmKind = (mode, which) => (which === 'revoke' ? 'revoke' : mode === 'setup' ? 'setup' : 'rotate');

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

// The hash without its setup part, so a reload or a copied address no longer carries the spent token.
export function withoutSetup(hash) {
  const rest = hash
    .replace(/^#/, '')
    .split('&')
    .filter((p) => p && !p.startsWith('setup='));
  return rest.length ? `#${rest.join('&')}` : '';
}

export function setupFailText(status, message) {
  if (status === 401) return 'This setup link was already used, has expired or was replaced. Ask for a new one.';
  if (status === 403 && message) return message;
  return 'The setup link could not be used. Try again, or ask for a new link.';
}

// What a polling failure says. A key the server no longer knows ends the page: only a new link helps. Without a
// key the page runs on a session, which sessionkey.js renews.
export function pollFail(status, code, message, hadKey) {
  if (status === 401 && hadKey) return { stop: true, text: "This link's key no longer works. It was rotated or revoked, so ask for a new link." };
  if (status === 403 && code === 'network_not_allowed') return { stop: true, text: message || 'This panel may not be used from this network.' };
  return { stop: false, text: 'Lost contact with Sode. Retrying.' };
}
