// The one place a panel link is built (the editor's Copy link and the Share modal's Link with key tab). Pure.
// Shape: panels/<slug or id>[?instance=<name>]#[key=<k>|setup=<t>][&kiosk=1]. The secret stays in the fragment, which
// never reaches a request line or a proxy log; the name is no secret, so it may ride the query.

export const KEY_PREFIX = 'sode_';
export const SETUP_PREFIX = 'sode_setup_';

// The server's secrets all start with a fixed prefix that adds nothing to a link, and the page puts it back.
// A panel key's body never starts with the prefix (newShortSecret), so a short and a full form cannot be confused.
export const shortSecret = (s) => (s.startsWith(SETUP_PREFIX) ? s.slice(SETUP_PREFIX.length) : s.startsWith(KEY_PREFIX) ? s.slice(KEY_PREFIX.length) : s);
export const fullKey = (s) => (s && !s.startsWith(KEY_PREFIX) ? KEY_PREFIX + s : s);
export const fullSetup = (s) => (s && !s.startsWith(SETUP_PREFIX) ? SETUP_PREFIX + s : s);

// The short name when the panel has one, else the id. A slug change breaks the old link, so a link is built fresh.
export const pathFor = (panel) => `panels/${panel.slug || panel.id}`;

// opts: kiosk, instance (device name), key or setup (the secret as the server sent it), base (document.baseURI, to
// make the link absolute).
export function linkFor(panel, { kiosk = false, instance = '', key = '', setup = '', base = '' } = {}) {
  const name = String(instance || '').trim();
  const frag = [key && `key=${shortSecret(key)}`, !key && setup && `setup=${shortSecret(setup)}`, kiosk && 'kiosk=1'].filter(Boolean).join('&');
  const link = `${pathFor(panel)}${name ? `?instance=${encodeURIComponent(name)}` : ''}${frag ? `#${frag}` : ''}`;
  return base ? new URL(link, base).href : link;
}

// ?instance=Studio%20A, or '' when the link names no session.
export const instanceFrom = (search) => new URLSearchParams(search).get('instance')?.trim() || '';
