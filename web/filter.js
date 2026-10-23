// The Recent filter language, for sources and destinations. No imports,
// so it runs in Node for tests.
//
// Terms are separated by commas and OR'd; ! terms then remove. Only !
// terms means everything but those.
//   65            number is 65 (as shown, so numberOffset applies)
//   1-50          number between, also "1 thru 50", "1 through 50", "1 to 50"
//   PIP           name contains, ignoring case
//   'NOT USED'    name contains, taken literally ('' for a quote inside)
//   "CAM 1"       name is exactly that, ignoring case ("" for a quote inside)
//   /^DSM\d/      name matches the regex (ignoring case unless flags given)
//   !65, !PIP     excludes; "not" works too before a number, quote or regex
//                 (so a bare NOT USED stays a name)
//
// A rule is { not, kind: 'num' | 'range' | 'name' | 'exact' | 're', a, b,
// text, flags, quoted }.

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

// Whether name text must be quoted to read back as a name.
const needsQuote = (t) => NUM.test(t.trim()) || RANGE.test(t.trim()) || /[,"']/.test(t) ||
  /^\s|\s$/.test(t) || /^[!/]/.test(t) || NOT_WORD.test(t);
const quote = (t, q) => q + t.replaceAll(q, q + q) + q;

// Rules as code, the way a person would type them.
export function filterText(rules) {
  return rules.map((r) => {
    const body = r.kind === 'num' ? String(r.a)
      : r.kind === 'range' ? `${r.a}-${r.b}`
      : r.kind === 're' ? `/${r.text}/${r.flags || ''}`
      : r.kind === 'exact' ? quote(r.text, '"')
      : r.quoted || needsQuote(r.text) ? quote(r.text, "'") : r.text;
    return (r.not ? '!' : '') + body;
  }).join(', ');
}

// A name folded for fuzzy matching: lower case, without spaces or
// punctuation, and numbers without leading zeros ("PROD MON03" →
// "prodmon3").
const fold = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '').replace(/(^|[a-z])0+(?=\d)/g, '$1');

// Fuzzy name text, for searching: every word of it is in the folded name.
// So "prodmon" finds "PROD MON03", "dsm3" "DSM 03" and "mv 28" "MV 28".
function fuzzy(text) {
  const words = text.split(/\s+/).map(fold).filter(Boolean);
  return (num, name) => {
    const n = fold(name);
    return words.every((w) => n.includes(w));
  };
}

// One rule as a test on (number as shown, name).
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

// rules → (num, name) => bool. No rules matches everything. With fuzzy,
// bare name terms match fuzzily (see fuzzy), for searching.
export function compileFilter(rules, { fuzzy = false } = {}) {
  const inc = rules.filter((r) => !r.not).map((r) => test(r, fuzzy));
  const exc = rules.filter((r) => r.not).map((r) => test(r, fuzzy));
  return (num, name = '') => (!inc.length || inc.some((f) => f(num, name))) && !exc.some((f) => f(num, name));
}

// Text → matcher, or null if it doesn't parse. Blank matches everything.
export function filterFn(text, opts) {
  const p = parseFilter(text || '');
  return p.rules ? compileFilter(p.rules, opts) : null;
}
