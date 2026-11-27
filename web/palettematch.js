// Palette matches that the plain ranking (match in paletterules.js) does not find. They are a search word, one typo,
// and query words that the name and the path hold between them. rankRows asks moreMatch only for a row that match
// does not find, so these rows come after the label and prefix matches. Pure, so Node tests load it.

// Search words: a word that people type for a row whose name does not hold it. A key that ends in '/' names every
// setting of that Settings section. Edit the table here; the Palette reads no other list. A row that is not in the
// catalogue now (Sign in while signed in) is left out. So one word can name a different row in each state.
export const SEARCH_WORDS = Object.freeze([
  { words: ['dismiss', 'toast', 'notification', 'notifications'], keys: ['setting:confirm/cfg-dismiss'] },
  { words: ['users', 'user list', 'add user', 'new user', 'manage users', 'edit users', 'delete user'], keys: ['setting:users/users-manage'] },
  { words: ['?', 'key list'], keys: ['action:shortcuts'] },
  { words: ['unlock'], keys: ['action:signIn'] },
  { words: ['undo', 'revert'], keys: ['action:undo'] },
  // Signed in, there is no Sign in row: the user wants to sign in as a different person. Signed out, Sign in has these
  // words itself and there is no Sign out row.
  { words: ['login', 'log in', 'sign in', 'switch user'], keys: ['action:signOut'] },
  { words: ['switch user'], keys: ['action:stepIn'] },
  { words: ['ip', 'router address', 'router ip', 'host'], keys: ['setting:routers/cfg-rt-address'] },
]);

// One edit (Damerau: a swap of two letters next to each other counts as one).
export function oneEdit(a, b) {
  if (a === b) return true;
  if (Math.abs(a.length - b.length) > 1) return false;
  let i = 0;
  while (i < a.length && i < b.length && a[i] === b[i]) i++;
  if (a.length === b.length) return a.slice(i + 1) === b.slice(i + 1) || (a[i] === b[i + 1] && a[i + 1] === b[i] && a.slice(i + 2) === b.slice(i + 2));
  return a.length > b.length ? a.slice(i + 1) === b.slice(i) : a.slice(i) === b.slice(i + 1);
}

const clean = (q) =>
  String(q || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, ' ');
const wordList = (text) =>
  String(text || '')
    .toLowerCase()
    .split(/[^\p{L}\p{N}]+/u)
    .filter(Boolean);

// A typo counts only from this length, so a short word does not match half the list.
export const TYPO_MIN = 4;

// Whether the query is a search word of the row. Three letters or more can be the start of the word ("dism").
export function searchWordHit(row, q) {
  q = clean(q);
  if (!q) return false;
  return SEARCH_WORDS.some(
    (e) => e.keys.some((k) => (k.endsWith('/') ? row.key.startsWith(k) : row.key === k)) && e.words.some((w) => w === q || (q.length >= 3 && w.startsWith(q))),
  );
}

// How a typed word matches a row word: 2 the start of it, 1 one typo (TYPO_MIN letters or more), 0 none. A typo can
// also be in the start of a longer word ("adres" in "address").
function wordHit(w, rw) {
  if (rw.startsWith(w)) return 2;
  if (w.length < TYPO_MIN) return 0;
  return oneEdit(w, rw) || oneEdit(w, rw.slice(0, w.length)) || oneEdit(w, rw.slice(0, w.length + 1)) ? 1 : 0;
}

// The query words against the row's name, path (hint) and words together. Each query word must hit a word. One of them
// must hit a word of the name, so a path alone ("routers") does not list a whole section. null for no match, else
// { typo }: true when a word needed a typo.
export function wordsMatch(row, q) {
  const qw = wordList(q);
  if (!qw.length) return null;
  const name = wordList(row.label);
  const all = [...name, ...wordList(row.hint), ...(row.words || []).flatMap(wordList)];
  let typo = false;
  let inName = false;
  for (const w of qw) {
    let best = 0;
    for (const rw of all) best = Math.max(best, wordHit(w, rw));
    if (!best) return null;
    if (best === 1) typo = true;
    if (name.some((rw) => wordHit(w, rw))) inName = true;
  }
  return inName ? { typo } : null;
}

// Scores for matchAll (paletterules.js), which maps kind to a strength. A search word is as good as a label word. The
// other two come after the hint matches: words across the name and path first, then a typo.
const SEARCH_WORD_HIT = { tier: 1, s: 1500, kind: 'word' };
const ACROSS = { tier: 0, s: 40, kind: 'across' };
const TYPO = { tier: 0, s: 20, kind: 'typo' };

// The match for a row that match (paletterules.js) did not find, or null.
export function moreMatch(row, q) {
  if (searchWordHit(row, q)) return SEARCH_WORD_HIT;
  const m = wordsMatch(row, q);
  if (!m) return null;
  return m.typo ? TYPO : ACROSS;
}
