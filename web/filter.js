// Recent filter language for sources and destinations. No imports (Node tests).
// Comma-separated terms are OR'd, then ! terms remove; only ! terms means everything but those.
// Terms: 65, 1-50 (also "1 thru 50", "through", "to"), PIP (name contains), 'NOT USED' ('' = quote), "CAM 1" (exact), /re/ (flags optional), !term ("not" works before a number, quote or regex).
// Rule: { not, kind: 'num' | 'range' | 'name' | 'exact' | 're', a, b, text, flags, quoted }.

const NUM = /^\d+$/;
// "not" as a word, only where what follows can't be part of a name.
const NOT_WORD = /^not\s+(?=['"/\d])/i;
// Flags that make a regex remember where it stopped, so the same name
// would match one time and not the next.
const stateless = (flags) => (flags || '').replace(/[gy]/g, '');
const RANGE = /^(\d+)\s*(?:-|\s(?:thru|through|to)\s)\s*(\d+)$/i;

// Parses filter text: { rules } or { error, at } (at = index in text).
export function parseFilter(text) {
  const rules = [];
  let i = 0;
  const n = text.length;
  const ws = () => { while (i < n && /\s/.test(text[i])) i++; };
  while (i < n) {
    ws();
    if (i >= n) break;
    if (text[i] === ',') { i++; continue; }
    const start = i;
    let not = false;
    if (text[i] === '!') { not = true; i++; ws(); }
    else if (NOT_WORD.test(text.slice(i))) { not = true; i += 3; ws(); }
    let rule;
    if (text[i] === '"' || text[i] === "'") {
      const q = text[i];
      let s = '';
      i++;
      for (;;) {
        if (i >= n) return { error: 'Unclosed quote', at: start };
        if (text[i] === q) {
          if (text[i + 1] === q) { s += q; i += 2; continue; }
          i++;
          break;
        }
        s += text[i++];
      }
      if (!s) return { error: 'Empty quotes', at: start };
      rule = q === '"' ? { not, kind: 'exact', text: s } : { not, kind: 'name', text: s, quoted: true };
    } else if (text[i] === '/') {
      let s = '';
      i++;
      for (;;) {
        if (i >= n) return { error: 'Unclosed /regex/', at: start };
        if (text[i] === '\\' && i + 1 < n) { s += text[i] + text[i + 1]; i += 2; continue; }
        if (text[i] === '/') { i++; break; }
        s += text[i++];
      }
      let flags = '';
      while (i < n && /[a-z]/i.test(text[i])) flags += text[i++];
      try {
        new RegExp(s, stateless(flags) || 'i');
      } catch (e) {
        return { error: `Bad regex /${s}/${flags}`, at: start };
      }
      rule = { not, kind: 're', text: s, flags };
    } else {
      let end = text.indexOf(',', i);
      if (end < 0) end = n;
      const t = text.slice(i, end).trim();
      i = end;
      if (!t) return { error: 'Nothing after !', at: start };
      let m;
      if (NUM.test(t)) rule = { not, kind: 'num', a: Number(t) };
      else if ((m = t.match(RANGE))) {
        const [a, b] = [Number(m[1]), Number(m[2])].sort((x, y) => x - y);
        rule = { not, kind: 'range', a, b };
      } else rule = { not, kind: 'name', text: t };
    }
    ws();
    if (i < n && text[i] !== ',') return { error: 'Expected a comma', at: i };
    rules.push(rule);
  }
  return { rules };
}

const needsQuote = (t) => NUM.test(t.trim()) || RANGE.test(t.trim()) || /[,"']/.test(t) ||
  /^\s|\s$/.test(t) || /^[!/]/.test(t) || NOT_WORD.test(t);
const quote = (t, q) => q + t.replaceAll(q, q + q) + q;
function slashes(t) {
  let out = '';
  for (let i = 0; i < t.length; i++) {
    if (t[i] === '\\') { out += t[i] + (t[i + 1] ?? ''); i++; continue; }
    out += t[i] === '/' ? '\\/' : t[i];
  }
  return out;
}

export function filterText(rules) {
  return rules.map((r) => {
    const body = r.kind === 'num' ? String(r.a)
      : r.kind === 'range' ? `${r.a}-${r.b}`
      : r.kind === 're' ? `/${slashes(r.text)}/${r.flags || ''}`
      : r.kind === 'exact' ? quote(r.text, '"')
      : r.quoted || needsQuote(r.text) ? quote(r.text, "'") : r.text;
    return (r.not ? '!' : '') + body;
  }).join(', ');
}

// Fold for fuzzy: lower case, no spaces or punctuation, no leading zeros ("PROD MON03" -> "prodmon3").
const fold = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '').replace(/(^|[a-z])0+(?=\d)/g, '$1');

// Every word of the fuzzy text is in the folded name.
function fuzzy(text) {
  const words = text.split(/\s+/).map(fold).filter(Boolean);
  return (num, name) => {
    const n = fold(name);
    return words.every((w) => n.includes(w));
  };
}

function test(r, fuzzyNames) {
  switch (r.kind) {
    case 'num': return (num) => num === r.a;
    case 'range': return (num) => num >= r.a && num <= r.b;
    case 're': {
      const re = new RegExp(r.text, stateless(r.flags) || 'i');
      return (num, name) => re.test(name);
    }
    case 'exact': {
      const t = r.text.toLowerCase();
      return (num, name) => name.toLowerCase() === t;
    }
    default: {
      // Quoted text is literal, even when searching.
      if (fuzzyNames && !r.quoted) return fuzzy(r.text);
      const t = r.text.toLowerCase();
      return (num, name) => name.toLowerCase().includes(t);
    }
  }
}

// No rules matches everything; fuzzy makes bare name terms fuzzy (for searching).
export function compileFilter(rules, { fuzzy = false } = {}) {
  const inc = rules.filter((r) => !r.not).map((r) => test(r, fuzzy));
  const exc = rules.filter((r) => r.not).map((r) => test(r, fuzzy));
  return (num, name = '') => (!inc.length || inc.some((f) => f(num, name))) && !exc.some((f) => f(num, name));
}

export function filterFn(text, opts) {
  const p = parseFilter(text || '');
  return p.rules ? compileFilter(p.rules, opts) : null;
}

// Filter sets: x = ext (made elsewhere); sn/sname/dn/dname = source/destination number+name; hiddenDst = Settings hides that destination.
export function entryAllows(f, { srcOk, dstOk }, x) {
  if (x.ext ? !f.ext : !f.sode) return false;
  if (x.ext && !f.hidden && x.hiddenDst) return false;
  return srcOk(x.sn, x.sname) && dstOk(x.dn, x.dname);
}

// 'or': any set allows it; else every set must. Excludes and kinds count only within their set.
export const stackAllows = (mode, sets, x) =>
  (mode === 'or' ? sets.some((s) => entryAllows(s.f, s, x)) : sets.every((s) => entryAllows(s.f, s, x)));
