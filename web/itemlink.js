// The link holds ids, not a copy, so it fits a QR code and opens nothing on another install. Pure.

export const OTHER_INSTALL = 'Item link from another Sode install.';
export const MISSING_PRESET = 'Preset not found, or outside your Rights.';

const params = (hash) => new URLSearchParams(String(hash || '').replace(/^#/, ''));

export const presetLink = (base, id, install) => `${base}#${new URLSearchParams({ preset: id, install })}`;

export function panelLink(link, install) {
  const at = link.indexOf('#');
  return `${link}${at < 0 ? '#' : '&'}${new URLSearchParams({ install })}`;
}

export const linkInstall = (hash) => params(hash).get('install')?.trim() || '';

// { preset, install, hash } for a preset link, or null; hash is the fragment with the link's parts removed.
export function readItemLink(hash) {
  const h = params(hash);
  const preset = h.get('preset')?.trim();
  if (!preset) return null;
  const install = h.get('install')?.trim() || '';
  h.delete('preset');
  h.delete('install');
  const rest = h.toString();
  return { preset, install, hash: rest ? `#${rest}` : '' };
}

// The page must know its install, the lock state (`access`, null until lock.js says) and the names. No lock
// can cover it. profileWait: a user is signed in and the profile with the Me presets has not come yet.
export function itemLinkStep(link, { install, access: L, pageLocked, namesIn, profileWait, presets }) {
  const covered = !L || (L.locked && (L.enforced || L.held)) || (pageLocked && !L.enforced);
  if (!install || !namesIn || covered) return { step: 'wait' };
  if (link.install !== install) return { step: 'other' };
  const preset = presets.find((p) => p.id === link.preset && (p.scope === 'global' || p.scope === 'user'));
  if (preset) return { step: 'open', preset };
  return profileWait ? { step: 'wait' } : { step: 'missing' };
}
