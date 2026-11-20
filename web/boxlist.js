// A list of destinations typed in the Destination box: "11-13", "11,12,13", "Rec 1-4", "Rec 1-Rec 4", "11-13, 20".
// Imports nothing, so node tests load it alone. desktop.js shows the matches as one multiselection; the Source box
// does not take lists.

// More than this in one list is a typing error, not a plan (a range such as 1-9999).
export const LIST_MAX = 100;

const PART = /^(.*?)\s*(\d+)(?:\s*-\s*(.*?)\s*(\d+))?$/;
const norm = (s) => s.trim().replace(/\s+/g, ' ').toLowerCase();

// null when the text is not a list: one item with no range, or a part that does not read. Else { parts, count } with
// each part { stem, from, to } (from <= to) and tooMany when the list has more than LIST_MAX items. A stem is a
// name's text before its number ("Rec"); an empty stem means a router number.
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
    const a = +m[2];
    const b = m[4] === undefined ? a : +m[4];
    parts.push({ stem, from: Math.min(a, b), to: Math.max(a, b), range: m[4] !== undefined });
  }
  if (parts.length < 2 && !parts[0]?.range) return null;
  const count = parts.reduce((n, p) => n + p.to - p.from + 1, 0);
  return { parts: parts.map(({ stem, from, to }) => ({ stem, from, to })), count, tooMany: count > LIST_MAX };
}

// True when a name starts with the text: then the text is a search, not a list ("MV 1-4 PIP" is named, so "MV 1-4"
// finds it).
export function namedAs(text, items) {
  const t = norm(text);
  return !!t && items.some((it) => norm(it.name || '').startsWith(t));
}

const NAME = /^(.*?)\s*(\d+)$/;

// items: [{ n, num, name }] in router order, num the shown number. found keeps the list's order without repeats;
// missing names each item of the list that no item has.
export function listMatches(list, items) {
  const byStem = new Map();
  for (const it of items) {
    const m = NAME.exec((it.name || '').trim());
    if (!m) continue;
    const key = `${norm(m[1])}\u0000${+m[2]}`;
    if (!byStem.has(key)) byStem.set(key, []);
    byStem.get(key).push(it);
  }
  const byNum = new Map(items.map((it) => [+it.num, it]));
  const found = [];
  const seen = new Set();
  const missing = [];
  for (const p of list.parts) {
    for (let k = p.from; k <= p.to; k++) {
      const hits = p.stem ? byStem.get(`${norm(p.stem)}\u0000${k}`) || [] : byNum.has(k) ? [byNum.get(k)] : [];
      if (!hits.length) missing.push(p.stem ? `${p.stem} ${k}` : String(k));
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

// The destination the next add on an empty Destination box adds: the lowest router number above the last added one
// that is not added yet. By number, not by name: the router's numbers are what the match list shows, and the box
// shows the name before the press. null when no such destination is left.
export function nextAdd(after, items, added) {
  let best = null;
  for (const it of items) if (it.n > after && !added(it) && (!best || it.n < best.n)) best = it;
  return best;
}
