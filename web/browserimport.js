// The values are another browser's, so only known values pass.

import { PREFS } from './prefs.js';
import { cleanBookmarks } from './bookmarkrules.js';

export function browserPrefs(file) {
  const out = {};
  for (const [name, p] of Object.entries(PREFS)) {
    const v = file?.[name];
    if (typeof v === 'string' && Object.hasOwn(p.values, v)) out[name] = v;
  }
  return out;
}

// Names match without regard to case.
export function mergeByName(local, file, replace) {
  if (replace) return file;
  const key = (x) => String(x.name).trim().toLowerCase();
  const out = local.map((x) => file.find((f) => key(f) === key(x)) || x);
  return out.concat(file.filter((f) => !local.some((x) => key(x) === key(f))));
}

// This browser's bookmarks are one list, written whole, as Everyone bookmarks on the server. A bookmark's place is its
// Alt+1-9 key, so a merge would move them. null: keep this browser's list. Add import keeps it when the file's is
// empty; Replace import empties it. The bookmark rules drop an unsafe entry.
export function browserBookmarks(file, replace) {
  if (!Array.isArray(file?.bookmarks)) return null;
  const list = cleanBookmarks(file.bookmarks);
  return list.length || replace ? list : null;
}
