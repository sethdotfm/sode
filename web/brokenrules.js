// Broken and Name moved in the page (GLOSSARY.md; docs/agents/presets.md, Broken banner): which marks still apply,
// the left-out lines and the Main-page banner rule. The server's marks (brokencheck.go) are kept until the next name
// read, so a mark counts only while the item still holds that reference. Pure (Node-tested); filter.js has no imports.
import { parseFilter } from './filter.js';

const SIDE = { source: 'src', destination: 'dst' };
const upper = (s) => s[0].toUpperCase() + s.slice(1);
const same = (a, b) =>
  String(a ?? '')
    .trim()
    .toLowerCase() ===
  String(b ?? '')
    .trim()
    .toLowerCase();

// A preset item in memory (presetref.js itemNums) holds mark m.
function holds(it, m) {
  const n = m?.ref?.n;
  switch (m?.kind) {
    case 'source':
      return it.src === n;
    case 'destination':
      return it.dsts.includes(n);
    case 'level':
      return it.levels.includes(n);
    case 'router':
      return (it.router || '') === (m.ref.router || '');
    default:
      return true;
  }
}

const list = (x) => (Array.isArray(x) ? x.filter((m) => m && typeof m === 'object' && m.ref) : []);

// {broken, moved}: the preset's marks that still apply. Kinds the page cannot place (an import's skipped preset) stay.
export function presetMarks(p) {
  const keep = (m) => (['source', 'destination', 'level', 'router'].includes(m.kind) ? (p.items || []).some((it) => holds(it, m)) : true);
  return { broken: list(p.broken).filter(keep), moved: list(p.moved).filter(keep) };
}

// The positive number and exact terms of one side's text: {nums: Set of shown numbers, exact: [text]}.
function terms(text) {
  const r = parseFilter(String(text ?? ''));
  const out = { nums: new Set(), exact: [] };
  for (const t of r.rules || []) {
    if (t.not) continue;
    if (t.kind === 'num') out.nums.add(t.a);
    if (t.kind === 'exact') out.exact.push(t.text);
  }
  return out;
}

// A filter set entry (filterstore.js: {f, broken}). offset: numberOffset, since the text holds shown numbers.
export function filterSetMarks(s, offset) {
  const by = { source: terms(s.f?.src), destination: terms(s.f?.dst) };
  const keep = (m) => {
    const t = by[m.kind];
    if (!t) return true;
    return m.ref.n >= 0 ? t.nums.has(m.ref.n + offset) : t.exact.some((x) => same(x, m.ref.name));
  };
  return { broken: list(s.broken).filter(keep), moved: [] };
}

export const statusOf = (m) => (m.broken.length ? 'broken' : m.moved.length ? 'moved' : '');
export const STATUS_TEXT = { broken: 'Broken', moved: 'Name moved' };

// What was left out and why, one line per mark.
export function leftOutLines(marks, offset) {
  return marks.map((m) => {
    const r = m.ref || {};
    const name = String(r.name || '').trim();
    switch (m.kind) {
      case 'router':
        return 'Its router is not in Sode';
      case 'level':
        return `Level ${r.n}: not on the router now`;
      case 'preset':
        return name ? `Preset "${name}": not in Sode` : 'A preset: not in Sode';
      case 'source':
      case 'destination':
        return r.n >= 0
          ? `${upper(m.kind)} ${r.n + offset}${name ? ` ${name}` : ''}: not on the router now`
          : `${upper(m.kind)} "${name}": no ${m.kind} has this name now`;
      default:
        return `${upper(String(m.kind || 'part'))}: not there`;
    }
  });
}

// ---- Banner: one entry per used Broken item. A dismissed entry stays dismissed while its marks stay the same, for
// the page's lifetime. So a fire after Dismiss does not bring it back, but a new missing part does.

const sig = (marks) =>
  marks
    .map((m) => `${m.kind}:${m.ref.router || ''}:${m.ref.n}:${m.ref.name || ''}`)
    .sort()
    .join('|');

export const newBannerState = () => new Map(); // key -> {sig, dismissed}

// item: {key, broken, moved}, its marks as presetMarks or filterSetMarks gives them.
export function useItem(st, item) {
  if (!item.broken.length) return;
  const s = sig(item.broken);
  const was = st.get(item.key);
  if (was?.dismissed && was.sig === s) return;
  st.set(item.key, { sig: s, dismissed: false });
}

export function dismissItem(st, key) {
  const was = st.get(key);
  if (was) was.dismissed = true;
}

// The entries to show, in the order they were used. now: key -> the item with its marks now. An item that is gone or
// fixed leaves the state, so a later break is new.
export function bannerItems(st, now) {
  const out = [];
  for (const [key, v] of st) {
    const it = now.get(key);
    if (!it || !it.broken.length) {
      st.delete(key);
      continue;
    }
    if (!v.dismissed) out.push(it);
  }
  return out;
}

// ---- This browser's items (no server marks): the same rules against the names the page has. The page's names keep a
// gone number's old name, so this check cannot see a gone number. It finds Name moved for presets. For filter sets
// it finds an exact term that no name matches.

const at = (map, name, not) => {
  for (const [n, v] of map) if (n !== not && same(v, name)) return true;
  return false;
};

// names: {src, dst} Maps of number to name. router: the hello's router UID.
export function localPresetMarks(p, names, router) {
  const moved = [];
  const check = (kind, n, stored, r) => {
    const map = names[SIDE[kind]];
    const now = map.get(n);
    if (!stored || now == null || same(now, stored) || !at(map, stored, n)) return;
    if (!moved.some((m) => m.kind === kind && m.ref.n === n)) moved.push({ kind, ref: { router: r, n, name: stored } });
  };
  for (const it of p.items || []) {
    const r = it.router || router;
    if (r !== router) continue;
    check('source', it.src, it.srcName, r);
    for (const d of it.dsts) check('destination', d, it.dstNames?.[d], r);
  }
  return { broken: [], moved };
}

export function localFilterMarks(s, names, router) {
  if (s.f?.router && s.f.router !== router) return { broken: [], moved: [] };
  const broken = [];
  for (const [kind, text] of [
    ['source', s.f?.src],
    ['destination', s.f?.dst],
  ]) {
    const map = names[SIDE[kind]];
    for (const name of terms(text).exact) if (!at(map, name, null)) broken.push({ kind, ref: { router, n: -1, name } });
  }
  return { broken, moved: [] };
}
