// Filter number and range terms in the import's remap step. A term gets a row only when the names at its numbers
// in the file differ from the names on the target. The server takes the rewritten text (importReq.filters,
// remapSet in internal/app/importremap.go). No core.js import, so Node tests load it.

import { parseFilter, filterText } from './filter.js';

const norm = (s) => (s || '').trim().toLowerCase();

const SIDES = [
  ['src', 'sources'],
  ['dst', 'destinations'],
];

const isNum = (r) => r.kind === 'num' || r.kind === 'range';
const span = (r) => (r.kind === 'num' ? [r.a, r.a] : [r.a, r.b]);
const label = (a, b) => (a === b ? String(a) : `${a}-${b}`);

function shownNames(list, offset) {
  const m = new Map();
  for (const x of list || []) m.set(x.n + offset, x.name);
  return m;
}

function inSpan(m, a, b) {
  return [...m.keys()].filter((n) => n >= a && n <= b).sort((x, y) => x - y);
}

// One row per distinct file router, side and number span, shared by every set that uses it. preset: 'names'
// (Convert to names), or 'keep' when the file has no names at those numbers.
export function termRows(router, target) {
  const rows = new Map();
  for (const s of router.sets || []) {
    for (const [side, list] of SIDES) {
      const p = parseFilter(s[side] || '');
      if (!p.rules) continue;
      const file = shownNames(router.names?.[list], router.offset);
      const here = shownNames(target[list], target.offset);
      for (const r of p.rules.filter(isNum)) {
        const [a, b] = span(r);
        const key = `${router.uid}:${side}:${label(a, b)}`;
        const known = rows.get(key);
        if (known) {
          if (!known.sets.includes(s.key)) known.sets.push(s.key);
          continue;
        }
        const nums = [...new Set([...inSpan(file, a, b), ...inSpan(here, a, b)])];
        if (nums.every((n) => norm(file.get(n)) === norm(here.get(n)))) continue;
        const names = [
          ...new Set(
            inSpan(file, a, b)
              .map((n) => file.get(n).trim())
              .filter(Boolean),
          ),
        ];
        rows.set(key, { key, side, label: label(a, b), names, canConvert: names.length > 0, preset: names.length ? 'names' : 'keep', sets: [s.key] });
      }
    }
  }
  return [...rows.values()];
}

export function termChoices(rows) {
  return Object.fromEntries(rows.map((r) => [r.key, r.preset]));
}

function rewrite(router, side, text, byKey, choices) {
  const p = parseFilter(text || '');
  if (!p.rules) return null;
  const out = [];
  for (const r of p.rules) {
    const row = isNum(r) && byKey.get(`${router.uid}:${side}:${label(...span(r))}`);
    const ch = row ? choices[row.key] : 'keep';
    if (!row || ch === 'keep' || (ch === 'names' && !row.canConvert)) out.push(r);
    else if (ch === 'names') out.push(...row.names.map((t) => ({ not: r.not, kind: 'exact', text: t })));
  }
  return out;
}

// importReq.filters for one file router: set key → the sides whose text the choices change.
export function filtersReq(router, rows, choices) {
  const byKey = new Map(rows.map((r) => [r.key, r]));
  const out = {};
  for (const s of router.sets || []) {
    for (const [side] of SIDES) {
      const rules = rewrite(router, side, s[side], byKey, choices);
      if (!rules) continue;
      const text = filterText(rules);
      if (text === filterText(parseFilter(s[side] || '').rules)) continue;
      (out[s.key] ||= {})[side] = text;
    }
  }
  return out;
}

const setName = (s) => (s.user ? `${s.name} (${s.user})` : s.name);

// The review: each term not kept, with its sets by name. open is the sets that lose every include term on a
// side, since that side then matches everything.
export function termChanges(router, rows, choices) {
  const sets = new Map((router.sets || []).map((s) => [s.key, s]));
  const terms = rows
    .filter((r) => choices[r.key] !== 'keep')
    .map((r) => ({ key: r.key, side: r.side, label: r.label, choice: choices[r.key], sets: r.sets.map((k) => setName(sets.get(k))) }));
  const byKey = new Map(rows.map((r) => [r.key, r]));
  const open = [];
  for (const s of router.sets || []) {
    for (const [side] of SIDES) {
      const before = parseFilter(s[side] || '').rules;
      const after = rewrite(router, side, s[side], byKey, choices);
      if (before?.some((r) => !r.not) && after && !after.some((r) => !r.not) && !open.includes(setName(s))) open.push(setName(s));
    }
  }
  return { terms, open };
}
