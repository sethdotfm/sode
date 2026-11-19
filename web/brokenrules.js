// The server's marks last until the next name read, so a mark counts only while the item still holds that reference. Pure (Node-tested).
import { parseFilter, filterText } from './filter.js';

const SIDE = { source: 'src', destination: 'dst' };
const upper = (s) => s[0].toUpperCase() + s.slice(1);
const same = (a, b) =>
  String(a ?? '')
    .trim()
    .toLowerCase() ===
  String(b ?? '')
    .trim()
    .toLowerCase();

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

// A panel's source or destination cell: the status of the panel's mark on its number (panelstore.go keeps the marks).
export function cellStatus(panel, cell) {
  if (cell?.kind !== 'source' && cell?.kind !== 'destination') return '';
  const on = (l) => list(l).some((m) => m.kind === cell.kind && m.ref.n === cell.n);
  return on(panel?.broken) ? 'broken' : on(panel?.moved) ? 'moved' : '';
}

// ---- Replace term (filter editor): a Broken number or exact term, offered with the name a name read saw replace it.

const isMarkTerm = (t, m, offset) => !t.not && (m.ref.n >= 0 ? t.kind === 'num' && t.a === m.ref.n + offset : t.kind === 'exact' && same(t.text, m.ref.name));

// The text with the first positive term for mark m replaced by next: digits make a number term, else an exact term.
export function replaceTerm(text, m, offset, next) {
  const r = parseFilter(String(text ?? ''));
  const v = String(next ?? '').trim();
  if (!r.rules || !v) return text;
  const i = r.rules.findIndex((t) => isMarkTerm(t, m, offset));
  if (i < 0) return text;
  r.rules[i] = /^\d+$/.test(v) ? { not: false, kind: 'num', a: Number(v) } : { not: false, kind: 'exact', text: v };
  return filterText(r.rules);
}

// now: the new name a name read found, or ''.
export function replaceRows(s, offset) {
  const rows = [];
  for (const m of filterSetMarks(s, offset).broken) {
    const side = SIDE[m.kind];
    if (!side) continue;
    const t = (parseFilter(String(s.f?.[side] ?? '')).rules || []).find((x) => isMarkTerm(x, m, offset));
    if (t) rows.push({ side, mark: m, term: filterText([t]), now: String(m.now || '') });
  }
  return rows;
}

// role: the lockState Role, '' under a Lock (policy.go refuses brokenCount then). Everyone's count is Administrator only.
export const countAsks = (role, me) => ({ all: role === 'administrator', mine: !!role && !!me });

export const statusOf = (m) => (m.broken.length ? 'broken' : m.moved.length ? 'moved' : '');
export const STATUS_TEXT = { broken: 'Broken', moved: 'Name moved' };

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

// A dismissed entry stays dismissed while its marks stay the same, for the page's lifetime.

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

// A gone or fixed item leaves the state, so a later break is new.
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
