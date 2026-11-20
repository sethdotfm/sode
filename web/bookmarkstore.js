// The three Bookmark lists and their saves. Everyone's is cfg.bookmarks (the `bookmarks` message, a Trusted user or an
// Administrator). Me is Profile.bookmarks (profileSave). This browser's is localStorage sode-bookmarks.
// Rules are in bookmarkrules.js. The bus event 'bookmarks' says that a list changed.

import { S, on, onMsg, emit, request } from './core.js';
import { readJSON, writeJSON } from './store.js';
import { profile, profileName, saveProfile } from './profile.js';
import { cleanBookmarks, shownList, relabel } from './bookmarkrules.js';

const KEY = 'sode-bookmarks';

export const localBookmarks = () => cleanBookmarks(readJSON(KEY, []));
export const userBookmarks = () => (profileName() ? cleanBookmarks(profile().bookmarks) : []);
export const globalBookmarks = () => cleanBookmarks(S.cfg?.bookmarks);
const LISTS = { local: localBookmarks, user: userBookmarks, global: globalBookmarks };
export const bookmarksIn = (scope) => (LISTS[scope] || (() => []))();

// The one list the strip, Alt+1-9 and the Palette show.
export const shownBookmarks = () => shownList({ local: localBookmarks(), user: userBookmarks(), global: globalBookmarks(), signedIn: !!profileName() });

// Saves a whole list. Answers { ok, err }. This browser's is written at once.
export function saveBookmarks(scope, list) {
  if (scope === 'local') {
    writeJSON(KEY, list.length ? list : null);
    emit('bookmarks');
    return Promise.resolve({ ok: true });
  }
  if (scope === 'user') {
    if (!profileName()) return Promise.resolve({ ok: false, err: 'Sign in to save to Me.' });
    return saveProfile({ bookmarks: list });
  }
  return request('bookmarksResult', { t: 'bookmarks', bookmarks: list });
}

// A rename of This browser's filter set or preset. The server does Everyone's and Me in the same save.
export function relabelLocalBookmarks(key, label) {
  const list = localBookmarks();
  if (!list.some((b) => b.key === key)) return;
  writeJSON(KEY, relabel(list, key, label));
  emit('bookmarks');
}

onMsg('bookmarks', (m) => {
  S.cfg = { ...S.cfg, bookmarks: Array.isArray(m.bookmarks) ? m.bookmarks : [] };
  emit('bookmarks');
});
on('profile', () => emit('bookmarks'));
on('helloData', () => emit('bookmarks'));
