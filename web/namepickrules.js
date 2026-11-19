// The sign-in name box's pure rules. users is the hello's list, [{name, len}].
import { fuzzyScore } from './fuzzy.js';

export function nameMatches(users, typed) {
  const list = users || [];
  const q = (typed || '').trim().toLowerCase();
  if (!q) return list.slice();
  return list
    .map((u, i) => ({ u, i, s: fuzzyScore({ num: '', lname: u.name.toLowerCase() }, q) }))
    .filter((m) => m.s >= 0)
    .sort((a, b) => b.s - a.s || a.i - b.i)
    .map((m) => m.u);
}

// Names are unique ignoring case (the server's rule too).
export function listedUser(users, typed) {
  const q = (typed || '').trim().toLowerCase();
  return (q && (users || []).find((u) => u.name.toLowerCase() === q)) || null;
}

export const pickName = (users, typed) => listedUser(users, typed)?.name || ((typed || '').trim() && nameMatches(users, typed)[0]?.name) || '';
