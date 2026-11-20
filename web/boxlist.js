// A list of destinations typed in the Destination box: "11-13", "11,12,13", "Rec 1-4", "Rec 1-Rec 4", "11-13, 20",
// "MV 1-4 PIP 1". The last is a range with a tail: MV 1 PIP 1 to MV 4 PIP 1. Imports nothing, so node tests load it alone. desktop.js shows the matches as one multiselection; the Source box
// does not take lists.

// More than this in one list is a typing error, not a plan (a range such as 1-9999).
export const LIST_MAX = 100;

// The tail after a range starts with a letter and has no comma or dash: "PIP 1", not "PIP 1-2".
const PART = /^(.*?)\s*(\d+)(?:\s*-\s*(.*?)\s*(\d+)(\s*[^\d\s,-][^,-]*)?)?$/;
const norm = (s) => s.trim().replace(/\s+/g, ' ').toLowerCase();
// A name as a key: lower case, one space between words, and a space between letters and digits, so "MV1 PIP1" and
// "MV 1 PIP 1" are the same. A number loses its leading zeros ("Rec 01" is Rec 1).
const canon = (s) => norm(s.replace(/(\d)(?=[^\d\s])|([^\d\s])(?=\d)/g, '$1$2 ').replace(/\d+/g, (d) => String(+d)));

// null when the text is not a list: one item with no range, or a part that does not read. Else { parts, count } with
// each part { stem, from, to, tail } (from <= to) and tooMany when the list has more than LIST_MAX items. A stem is a
// name's text before its number ("Rec"); an empty stem means a router number. A tail is the name's text after the
// number ("PIP 1"), or empty. Only a range of names has a tail.
export function parseList(text) {
  const raw = String(text)
    .split(',')
    .map((s) => s.trim());
  const parts = [];
  for (const s of raw.filter(Boolean)) {
    const m = PART.exec(s);
    if (!m) return null;
    const stem = m[1].trim();
    // "Rec 1-Rec 4": the second stem, if any, is the first one again. "1-Rec 4" is not a list.
    if (m[3] && norm(m[3]) !== norm(stem)) return null;
    const tail = (m[5] || '').trim();
    // "1-4 PIP 1": router numbers have no names to filter.
    if (tail && !stem) return null;
    const a = +m[2];
    const b = m[4] === undefined ? a : +m[4];
    parts.push({ stem, from: Math.min(a, b), to: Math.max(a, b), tail, range: m[4] !== undefined });
  }
  if (parts.length < 2 && !parts[0]?.range) return null;
  const count = parts.reduce((n, p) => n + p.to - p.from + 1, 0);
  return { parts: parts.map(({ stem, from, to, tail }) => ({ stem, from, to, tail })), count, tooMany: count > LIST_MAX };
}

// True when a name starts with the text: then the text is a search, not a list ("MV 1-4 PIP" is named, so "MV 1-4"
// finds it).
export function namedAs(text, items) {
  const t = norm(text);
  return !!t && items.some((it) => norm(it.name || '').startsWith(t));
}

// items: [{ n, num, name }] in router order, num the shown number. found keeps the list's order without repeats;
// missing names each item of the list that no item has.
export function listMatches(list, items) {
  const byName = new Map();
  for (const it of items) {
    const key = canon(it.name || '');
    if (!key) continue;
    if (!byName.has(key)) byName.set(key, []);
    byName.get(key).push(it);
  }
  const byNum = new Map(items.map((it) => [+it.num, it]));
  const found = [];
  const seen = new Set();
  const missing = [];
  for (const p of list.parts) {
    for (let k = p.from; k <= p.to; k++) {
      const name = [p.stem, k, p.tail].filter((x) => x !== '').join(' ');
      const hits = p.stem ? byName.get(canon(name)) || [] : byNum.has(k) ? [byNum.get(k)] : [];
      if (!hits.length) missing.push(name);
      for (const it of hits) {
        if (seen.has(it.n)) continue;
        seen.add(it.n);
        found.push(it);
      }
    }
  }
  return { found, missing };
}

// The match list's note for the items a list names that the router does not have. Empty when none are missing.
export function missingNote(missing, shown = 4) {
  if (!missing.length) return '';
  const more = missing.length - shown;
  return `Not found: ${missing.slice(0, shown).join(', ')}${more > 0 ? ` and ${more} more` : ''}`;
}

// ("Cam 3" → "Cam", "Prmpt A" → "Prmpt", "Mon 2 B" → "Mon"): trailing digits and single letters set apart by a space.
export function nameStem(name) {
  let s = name.trim();
  for (let prev = ''; s !== prev; ) {
    prev = s;
    s = s.replace(/\s+[A-Za-z]$/, '').replace(/[\s\d]+$/, '');
  }
  return s;
}

// The item after it: the lowest router number above it that skip does not refuse. With find (the Next match behavior
// "Next match of the name", nextFind in plans.js), the lowest such item that find(nameStem) returns comes first. It
// falls back to the number when the name is only a counter or no higher match is left. null when no item is left. The
// Next chip and the next add share it, so both use the one setting.
export function nextAfter(it, items, { skip = () => false, find = null } = {}) {
  const lowest = (list) => {
    let best = null;
    for (const x of list) if (x.n > it.n && !skip(x) && (!best || x.n < best.n)) best = x;
    return best;
  };
  const q = find && nameStem(it.name || '');
  return (q && lowest(find(q))) || lowest(items);
}

// The destination the next add on an empty Destination box adds: nextAfter the last added one, skipping the ones that
// are added. null when no such destination is left.
export const nextAdd = (last, items, added, find = null) => nextAfter(last, items, { skip: added, find });
