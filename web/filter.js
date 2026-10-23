// The Recent filter language, for sources and destinations. No imports,
// so it runs in Node for tests.
//
// Terms are separated by commas and OR'd; ! (or "not") terms then remove.
// Only ! terms means everything but those.
//   65            number is 65 (as shown, so numberOffset applies)
//   1-50          number between, also "1 thru 50", "1 through 50", "1 to 50"
//   PIP           name contains, ignoring case
//   "5D"          quoted: always name contains ("" for a quote inside)
//   /^DSM\d/      name matches the regex (ignoring case unless flags given)
//
// A rule is { not, kind: 'num' | 'range' | 'name' | 're', a, b, text, flags }.

const NUM = /^\d+$/;
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
    else if (/^not\s/i.test(text.slice(i, i + 4))) { not = true; i += 3; ws(); }
    let rule;
    if (text[i] === '"') {
      let s = '';
      i++;
      for (;;) {
        if (i >= n) return { error: 'Unclosed quote', at: start };
        if (text[i] === '"') {
          if (text[i + 1] === '"') { s += '"'; i += 2; continue; }
          i++;
          break;
        }
        s += text[i++];
      }
      if (!s) return { error: 'Empty quotes', at: start };
      rule = { not, kind: 'name', text: s };
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
        new RegExp(s, flags || 'i');
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
const needsQuote = (t) => NUM.test(t.trim()) || RANGE.test(t.trim()) || /[,"]/.test(t) ||
  /^\s|\s$/.test(t) || /^[!/]/.test(t) || /^not\s/i.test(t);

// Rules as code, the way a person would type them.
export function filterText(rules) {
  return rules.map((r) => {
    const body = r.kind === 'num' ? String(r.a)
      : r.kind === 'range' ? `${r.a}-${r.b}`
      : r.kind === 're' ? `/${r.text}/${r.flags || ''}`
      : needsQuote(r.text) ? `"${r.text.replace(/"/g, '""')}"` : r.text;
    return (r.not ? '!' : '') + body;
  }).join(', ');
}

// Fuzzy name text, as the source and destination boxes find things: the
// name contains it, or its letters come in order in "number name"
// (spaces in the text don't count). So "mv28" finds "MV 28".
function fuzzy(text) {
  const q = text.toLowerCase();
  const qq = q.replace(/\s+/g, '');
  return (num, name) => {
    const n = name.toLowerCase();
    if (n.includes(q)) return true;
    const t = `${num} ${n}`;
    let i = 0;
    for (let j = 0; j < t.length && i < qq.length; j++) if (t[j] === qq[i]) i++;
    return i === qq.length;
  };
}

// One rule as a test on (number as shown, name).
function test(r, fuzzyNames) {
  switch (r.kind) {
    case 'num': return (num) => num === r.a;
    case 'range': return (num) => num >= r.a && num <= r.b;
    case 're': {
      const re = new RegExp(r.text, r.flags || 'i');
      return (num, name) => re.test(name);
    }
    default: {
      if (fuzzyNames) return fuzzy(r.text);
      const t = r.text.toLowerCase();
      return (num, name) => name.toLowerCase().includes(t);
    }
  }
}

// rules → (num, name) => bool. No rules matches everything. With fuzzy,
// name terms match fuzzily (see fuzzy), for searching.
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
