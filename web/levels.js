// The levels language: what's typed in a levels box → UI levels. Pure, no
// imports, so it's tested in Node (web/test/levels.test.mjs).

// One typed level ("3", or in "va" also "v" and "a2" = 2) → UI level, or
// NaN.
function levelTok(tok, mode) {
  const t = tok.toLowerCase();
  if (mode === 'va') {
    if (t === 'v') return 1;
    const a = t.match(/^a(\d+)$/);
    if (a) return +a[1] >= 1 ? +a[1] + 1 : NaN;
  }
  return /^\d+$/.test(t) ? +t + (mode === '1' ? 0 : 1) : NaN;
}

// "1-3,5" (or "v,1-4") in mode → sorted UI levels within 1..max, or null.
// A part starting with ! is left out: "!0" is every level but 0, and
// "1-8 !3" (or "1-8!3") is 1-8 without 3. + is the same as a comma, t,
// thru or through as -, and n or not as !: "1 thru 8 not 3". For a phone's
// number pad, . is a comma too and .. a -: "1..8.10"; and is a comma.
export function parseLevelsAs(text, mode, max) {
  const out = new Set(), not = new Set();
  const norm = text.replace(/and/gi, ',').replace(/not|n/gi, '!').replace(/through|thru|t|\.{2,}/gi, '-').replace(/\./g, ',')
    .replace(/\s*-\s*/g, '-').replace(/!\s*/g, ' !');
  for (const part of norm.split(/[\s,+]+/).filter(Boolean)) {
    const m = part.match(/^(!?)([^-!]+)(?:-([^-!]+))?$/);
    if (!m) return null;
    let a = levelTok(m[2], mode), b = m[3] ? levelTok(m[3], mode) : a;
    if (!Number.isInteger(a) || !Number.isInteger(b)) return null;
    if (a > b) [a, b] = [b, a];
    for (let i = a; i <= b; i++) {
      if (i < 1 || i > max) return null;
      (m[1] ? not : out).add(i);
    }
  }
  if (!out.size && not.size) for (let i = 1; i <= max; i++) out.add(i);
  for (const i of not) out.delete(i);
  return out.size ? [...out].sort((x, y) => x - y) : null;
}
