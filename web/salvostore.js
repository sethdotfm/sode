// Salvos: named sets of takes, sent together as one confirm and one batch.
// Global ones live in the config (cfg.salvos, edited with the settings PIN,
// like filter sets); this browser's own in localStorage. Stored as numbers
// only (internal source/destination numbers, UI levels, [] = default
// levels), never names, because names are re-polled from the router.
// A salvo: { name, items: [{ src, dsts: [n], levels: [ui] }] }.

import { S, on, onMsg, emit, off, srcItem, dstItem } from './core.js';
import { readJSON, writeJSON } from './store.js';

export const MAX_SALVOS = 30;
export const MAX_ITEMS = 100;
export const MAX_DSTS = 100;
const KEY = 'sode-salvos';

const ints = (a) => [].concat(a ?? []).map(Number).filter((n) => Number.isInteger(n));
export const normItem = (it) => ({
  src: Number.isInteger(+it?.src) ? +it.src : -1,
  dsts: [...new Set(ints(it?.dsts).filter((n) => n >= 0))],
  levels: [...new Set(ints(it?.levels).filter((n) => n >= 1))].sort((a, b) => a - b),
});
export const normSalvo = (s) => ({
  name: String(s?.name ?? '').trim(),
  items: [].concat(s?.items ?? []).map(normItem),
});

export const globalSalvos = () => (S.cfg?.salvos || []).filter((s) => s?.name).map((s) => ({ scope: 'global', ...normSalvo(s) }));
export const localSalvos = () => [].concat(readJSON(KEY, [])).filter((s) => s?.name).map((s) => ({ scope: 'local', ...normSalvo(s) }));
export const allSalvos = () => [...globalSalvos(), ...localSalvos()];
export const findSalvo = (scope, name) =>
  allSalvos().find((s) => s.scope === scope && s.name.toLowerCase() === String(name).toLowerCase());

// Why a salvo can't be saved under this name in this scope, or ''. `old` is
// the name being edited (it may be kept, or only change case).
export function nameProblem(list, name, old = '') {
  const n = name.trim();
  if (!n) return 'Give the salvo a name.';
  if ([...n].length > 32) return 'Names are at most 32 characters.';
  if (list.some((s) => s.name.toLowerCase() === n.toLowerCase() && s.name.toLowerCase() !== old.toLowerCase())) {
    return `There is already a salvo named “${n}”.`;
  }
  if (!old && list.length >= MAX_SALVOS) return `At most ${MAX_SALVOS} salvos.`;
  return '';
}

// A salvo that can be saved: every item has a source and a destination.
export const salvoProblem = (s) => {
  if (!s.items.length) return 'Add at least one take.';
  if (s.items.length > MAX_ITEMS) return `At most ${MAX_ITEMS} sources.`;
  for (const it of s.items) {
    if (it.src < 0) return 'Every row needs a source.';
    if (!it.dsts.length) return 'Every row needs a destination.';
    if (it.dsts.length > MAX_DSTS) return `At most ${MAX_DSTS} destinations per source.`;
  }
  return salvoTotalProblem(s) || salvoConflict(s);
};

export const MAX_TOTAL = 200;
export const salvoTotalProblem = (s) =>
  (s.items.reduce((a, it) => a + it.dsts.length, 0) > MAX_TOTAL ? `A salvo can take at most ${MAX_TOTAL} destinations in total.` : '');

// Two items taking different sources to one destination on overlapping
// levels (blank = the default levels, which overlap with everything) would
// race. The same source twice merges, so it is fine. '' or a message.
export function salvoConflict(s) {
  const clash = (a, b) => !a.length || !b.length || a.some((l) => b.includes(l));
  for (let i = 0; i < s.items.length; i++) {
    for (const b of s.items.slice(i + 1)) {
      const a = s.items[i];
      if (a.src === b.src || !clash(a.levels, b.levels)) continue;
      const d = a.dsts.find((x) => b.dsts.includes(x));
      if (d != null) {
        return `Destination ${d + off()} is taken from both source ${a.src + off()} and source ${b.src + off()} on the same levels.`;
      }
    }
  }
  return '';
}

// Replaces this browser's salvo `old` (or adds, when old is '') with s.
export function saveLocalSalvo(s, old = '') {
  const o = old.toLowerCase();
  const list = localSalvos().map(({ name, items }) => ({ name, items }));
  const at = list.findIndex((x) => x.name.toLowerCase() === o);
  const next = { name: s.name, items: s.items };
  if (old && at >= 0) list[at] = next;
  else list.push(next);
  writeJSON(KEY, list);
  emit('salvos');
}
export function deleteLocalSalvo(name) {
  const list = localSalvos().filter((s) => s.name.toLowerCase() !== name.toLowerCase()).map(({ name: n, items }) => ({ name: n, items }));
  writeJSON(KEY, list.length ? list : null);
  emit('salvos');
}

// ---- typed references: "12, 20-24, cam 3" → numbers

// Display numbers are internal + off(); a bare number or range a-b is a
// display number, anything else is looked up by name with lookup(token).
// -> { ns: [internal], bad: [tokens] }
export function parseRefs(text, offset, lookup) {
  const ns = [], bad = [];
  const add = (n) => { if (!ns.includes(n)) ns.push(n); };
  for (const raw of String(text).split(',')) {
    const t = raw.trim();
    if (!t) continue;
    let m;
    if (/^\d+$/.test(t)) add(+t - offset);
    else if ((m = /^(\d+)\s*-\s*(\d+)$/.exec(t)) && +m[2] >= +m[1] && +m[2] - +m[1] < 1000) {
      for (let n = +m[1]; n <= +m[2]; n++) add(n - offset);
    } else {
      const n = lookup(t);
      if (n == null) bad.push(t);
      else add(n);
    }
  }
  return { ns, bad };
}

// ---- turning a salvo into takes

// Which of a salvo's takes can go now. has: { src(n), dst(n), level(u) },
// defaults: the default levels (UI). Returns the takes grouped per
// destination, [{ dst, groups: [{ src, levels }] }], and what was left out:
// missing = [{ kind: 'source' | 'destination' | 'level', n }].
export function resolveSalvo(salvo, has, defaults) {
  const byDst = new Map();
  const missing = [];
  const miss = (kind, n) => { if (!missing.some((m) => m.kind === kind && m.n === n)) missing.push({ kind, n }); };
  for (const it of salvo.items) {
    if (!has.src(it.src)) { miss('source', it.src); continue; }
    const want = it.levels.length ? it.levels : defaults;
    const levels = want.filter((u) => has.level(u));
    for (const u of want) if (!has.level(u)) miss('level', u);
    if (!levels.length) continue;
    for (const d of it.dsts) {
      if (!has.dst(d)) { miss('destination', d); continue; }
      if (!byDst.has(d)) byDst.set(d, []);
      const groups = byDst.get(d);
      const g = groups.find((x) => x.src === it.src);
      if (g) g.levels = [...new Set([...g.levels, ...levels])].sort((a, b) => a - b);
      else groups.push({ src: it.src, levels: [...levels] });
    }
  }
  return { takes: [...byDst].map(([dst, groups]) => ({ dst, groups })), missing };
}

// A plan for plans.js (verb 'salvo': leaves the patch boxes alone), with
// plan.missing = text lines for what could not be resolved.
export function salvoPlan(salvo, defaults) {
  const r = resolveSalvo(salvo, {
    src: (n) => S.srcBy.has(n), dst: (n) => S.dstBy.has(n), level: (u) => S.levelBy.has(u),
  }, defaults);
  const num = (n) => String(n + off());
  const lines = r.missing.map((m) => (m.kind === 'level' ? `Level ${m.n} does not exist` : `${m.kind[0].toUpperCase()}${m.kind.slice(1)} ${num(m.n)} does not exist`) + ' on the router now');
  // plan.error: a salvo saved before these checks existed; nothing is sent.
  const takes = r.takes.reduce((a, t) => a + t.groups.length, 0);
  const error = salvoConflict(salvo) || (takes > MAX_TOTAL ? salvoTotalProblem({ items: [{ dsts: new Array(takes) }] }) : salvoTotalProblem(salvo));
  return {
    verb: 'salvo', salvo: salvo.name, missing: lines, error,
    items: error ? [] : r.takes.map((t) => ({
      dst: dstItem(t.dst), skipped: [], untouched: [], undo: null, retry: null,
      groups: t.groups.map((g) => ({ src: srcItem(g.src), levels: g.levels })),
    })),
  };
}

// ---- making a salvo from a patch or from history

// One item per source from takes [{ src, dst, levels }] (history entries or
// the patch boxes), in first-seen order, destinations in first-seen order.
export function itemsFromTakes(takes) {
  const by = new Map();
  for (const t of takes) {
    const key = `${t.src}|${t.levels.join(',')}`;
    if (!by.has(key)) by.set(key, { src: t.src, dsts: [], levels: [...t.levels] });
    const it = by.get(key);
    if (!it.dsts.includes(t.dst)) it.dsts.push(t.dst);
  }
  return [...by.values()].map(normItem);
}

// The newest take made in Sode with the rest of its folder (same group),
// from history newest first.
export function latestFolderTakes(hist) {
  const e = hist.find((x) => !x.ext && x.undo == null && x.retry == null);
  if (!e) return [];
  const es = e.group && e.groupN > 1 ? hist.filter((x) => x.group === e.group && !x.ext && x.undo == null) : [e];
  return es.slice().reverse().map((x) => ({ src: x.src, dst: x.dst, levels: x.levels }));
}

// Global salvos changed for everyone.
onMsg('salvos', (m) => {
  S.cfg = { ...S.cfg, salvos: m.salvos };
  emit('salvos');
});
on('helloData', () => emit('salvos'));
