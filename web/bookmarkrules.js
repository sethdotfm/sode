// Bookmark rules (GLOSSARY.md, Bookmark): pure, so Node tests load them. A bookmark opens a Palette row (its key, as
// palettecatalogue.js builds it) or an outside web address. The server checks the same things (internal/config/
// bookmarks.go); web/test/bookmark_cases.json holds the cases that both read.
//
// The stored shape: { key } or { url }, plus label, newTab and inPalette (Show in Palette).

// Keep in step with MaxBookmarks and MaxBookmarkLabel in internal/config/bookmarks.go.
export const BOOKMARKS_MAX = 30;
export const LABEL_MAX = 40;
const URL_MAX = 2000;
const KEY_MAX = 120; // MaxPaletteKey
const KINDS = ['action', 'preset', 'page', 'panel', 'filterSet', 'setting'];

// Why Sode refuses this web address, or ''. Only http: and https:, and never a secret: each reader of a bookmark gets
// what it holds.
export function urlProblem(raw) {
  const s = String(raw ?? '');
  if (s.length > URL_MAX) return `a web address has at most ${URL_MAX} characters`;
  let u;
  try {
    u = new URL(s);
  } catch {
    u = null;
  }
  if (!u || (u.protocol !== 'http:' && u.protocol !== 'https:') || !u.host) return 'use a web address that starts with http:// or https://';
  if (u.username || u.password) return 'a web address in a bookmark cannot hold a user name or password';
  for (const part of u.hash.replace(/^#/, '').split('&')) {
    const name = part.split('=')[0].toLowerCase();
    if (name === 'key') return 'a bookmark cannot hold a panel key';
    if (name === 'setup') return 'a bookmark cannot hold a setup link';
    if (name === 'share') return 'a bookmark cannot hold a #share= link';
  }
  for (const name of u.searchParams.keys()) {
    const n = name.toLowerCase();
    if (n === 'key' || n === 'token') return 'a bookmark cannot hold a panel key';
    if (n === 'setup') return 'a bookmark cannot hold a setup link';
  }
  return '';
}

// Why Sode refuses this bookmark, or ''. The same words as the server.
export function bookmarkProblem(b) {
  const label = String(b?.label ?? '').trim();
  const key = String(b?.key ?? '').trim();
  const url = String(b?.url ?? '').trim();
  if (!label) return 'a bookmark needs a name';
  if ([...label].length > LABEL_MAX) return `a bookmark name has at most ${LABEL_MAX} characters`;
  if (/\p{C}/u.test(label)) return 'a bookmark name has only characters you can see';
  if (key && url) return 'a bookmark opens a Palette row or a web address, not both';
  if (key)
    return KINDS.includes(key.split(':')[0]) && key.includes(':') && [...key].length <= KEY_MAX ? '' : 'a bookmark names a Palette row that Sode does not know';
  if (url) return urlProblem(url);
  return 'a bookmark needs a Palette row or a web address';
}

// A stored list as the page uses it: bad entries out, at most BOOKMARKS_MAX.
export function cleanBookmarks(v) {
  if (!Array.isArray(v)) return [];
  return v
    .filter((b) => b && typeof b === 'object' && !bookmarkProblem(b))
    .map((b) => ({
      ...(b.key ? { key: String(b.key).trim() } : { url: String(b.url).trim() }),
      label: String(b.label).trim(),
      newTab: !!b.newTab,
      inPalette: b.inPalette !== false,
    }))
    .slice(0, BOOKMARKS_MAX);
}

// The list the strip shows: This browser's if it has one, else Me (signed in), else Everyone. Never merged.
export function shownList({ local = [], user = [], global = [], signedIn = false }) {
  if (local.length) return { scope: 'local', list: local };
  if (signedIn && user.length) return { scope: 'user', list: user };
  return { scope: 'global', list: global };
}

// Where a Palette row's Bookmark action adds: Me for a signed-in user, else This browser.
export const ownScope = (signedIn) => (signedIn ? 'user' : 'local');

// Everyone's list needs a Trusted user or an Administrator. With no users, every browser acts as an Administrator.
export const canEditEveryone = ({ role = '', noUsers = false } = {}) => noUsers || role === 'trusted' || role === 'administrator';

export const sameTarget = (a, b) => (a.key ? a.key === b.key : !!a.url && a.url === b.url);

// The list with b at the end, or { err } when it is full or holds the same target.
export function addBookmark(list, b) {
  const why = bookmarkProblem(b);
  if (why) return { err: why };
  if (list.some((x) => sameTarget(x, b))) return { err: `${b.label} is already a bookmark here` };
  if (list.length >= BOOKMARKS_MAX) return { err: `A list holds at most ${BOOKMARKS_MAX} bookmarks` };
  return { list: [...list, b] };
}

// A Palette row as a new bookmark: Show in Palette is on, a new tab is off.
export const fromRow = (row) => ({ key: row.key, label: String(row.label).slice(0, LABEL_MAX), newTab: false, inPalette: true });

// A web address as a new bookmark: it opens in a new tab unless the person turns that off.
export function fromUrl(url, label = '') {
  const u = String(url).trim();
  let name = String(label).trim();
  if (!name) {
    try {
      name = new URL(u).host;
    } catch {
      name = u;
    }
  }
  return { url: u, label: name.slice(0, LABEL_MAX), newTab: true, inPalette: true };
}

// "Open in a new tab" is for pages, panels and web addresses. Settings runs on Home, so it has no new tab.
export const canNewTab = (b, row) => !!b.url || (!!row && (row.kind === 'page' || row.kind === 'panel') && row.key !== 'page:settings');

// What a bookmark shows. rows: the catalogue, or null while it is not read yet (then no Missing).
// state: 'ok', 'missing' (the target is gone), 'noAccess' (outside Rights or Role) or 'signIn' (a locked page).
export function bookmarkView(b, rows) {
  if (b.url) return { state: 'ok', label: b.label, row: null };
  if (!rows) return { state: 'ok', label: b.label, row: null };
  const row = rows.find((r) => r.key === b.key) || null;
  if (!row) return { state: 'missing', label: b.label, row: null };
  const label = row.label || b.label;
  if (!row.access.ok) return { state: 'noAccess', label, row };
  if (row.access.lock) return { state: 'signIn', label, row };
  return { state: 'ok', label, row };
}

export const STATE_TEXT = { missing: 'Missing', noAccess: 'No access', signIn: 'Sign in' };

// The bookmarks that show first in the Palette's empty query, as rows. A web address becomes a row of kind 'url'.
export function paletteMarks(list, rows) {
  const out = [];
  for (const b of list) {
    if (!b.inPalette) continue;
    if (b.url) {
      out.push({
        key: `url:${b.url}`,
        kind: 'url',
        cat: '',
        label: b.label,
        hint: 'Web address',
        icon: 'external-link',
        words: [],
        home: false,
        target: { url: b.url, newTab: b.newTab },
        access: { ok: true, lock: false, why: '', offer: [] },
      });
      continue;
    }
    const row = rows.find((r) => r.key === b.key);
    if (row) out.push(row);
  }
  return out;
}

// A rename in the same scope: the bookmarks on from point at to, with the new name.
export const renameKey = (list, from, to, label) => list.map((b) => (b.key === from ? { ...b, key: to, label: label || b.label } : b));

// Move: { by } (Alt+arrows, buttons) or { before } (a drag onto another row's index). null when nothing moves.
export function moveBookmark(list, i, where) {
  if (i < 0 || i >= list.length) return null;
  let to;
  if ('by' in where) {
    to = i + where.by;
    if (to < 0 || to >= list.length) return null;
  } else {
    to = where.before > i ? where.before - 1 : where.before;
    if (to === i || to < 0 || to >= list.length) return null;
  }
  const next = list.slice();
  const [b] = next.splice(i, 1);
  next.splice(to, 0, b);
  return next;
}
