// The main page's fuzzy scoring. No imports, so Node tests and the Grid panel can load it without core.js.
// it is { num, lname }: the number as shown and the name in lower case. q is trimmed and in lower case.
// -1 is no match; a higher score is a better match.
export function fuzzyScore(it, q) {
  if (it.num === q) return 1000;
  if (/^\d+$/.test(q) && it.num.startsWith(q)) return 800 - it.num.length;
  if (it.lname === q) return 900;
  if (it.lname.startsWith(q)) return 700 - it.lname.length;
  const idx = it.lname.indexOf(q);
  if (idx >= 0) return 600 - idx;
  // Subsequence over "num name", rewarding runs and word starts.
  const t = it.num + ' ' + it.lname;
  const qq = q.replace(/\s+/g, '');
  let qi = 0,
    pts = 0,
    prev = -2;
  for (let i = 0; i < t.length && qi < qq.length; i++) {
    if (t[i] !== qq[qi]) continue;
    pts += (i === prev + 1 ? 3 : 1) + (i === 0 || t[i - 1] === ' ' ? 2 : 0);
    prev = i;
    qi++;
  }
  return qi < qq.length ? -1 : 100 + pts - t.length / 100;
}
