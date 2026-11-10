// Pure rules for the address other devices use in a link made on a localhost page (see sharehost.js).

const LOOPBACK = /^(localhost|127(\.\d{1,3}){3}|\[?::1\]?|[^.]+\.localhost)$/i;

export const isLoopback = (hostname) => LOOPBACK.test(hostname || '');

// text is "192.168.1.20:8080" or a full origin; a bare host borrows the page's scheme. null when it is not an address.
export function parseShareHost(text, pageProtocol = 'http:') {
  const t = String(text || '').trim();
  if (!t) return null;
  try {
    const u = new URL(/^[a-z][a-z0-9+.-]*:\/\//i.test(t) ? t : `${pageProtocol}//${t}`);
    if (!/^https?:$/.test(u.protocol) || !u.hostname || isLoopback(u.hostname)) return null;
    return u;
  } catch {
    return null;
  }
}

// The base a link is made against: the page's own, unless it is loopback and an address was typed.
export function shareBase(base, stored) {
  const page = new URL(base);
  if (!isLoopback(page.hostname)) return page.href;
  const to = parseShareHost(stored, page.protocol);
  if (!to) return page.href;
  const out = new URL(page.pathname + page.search, to.origin);
  return out.href;
}
