// Salvos: named sets of takes, sent together as one confirm and one batch.
// Global ones live in the config (cfg.salvos, edited with the settings PIN,
// like filter sets); this browser's own in localStorage. Stored as numbers
// only (internal source/destination numbers, UI levels, [] = default
// levels), never names, because names are re-polled from the router.
// A salvo: { name, items: [{ src, dsts: [n], levels: [ui] }] }.

import { S, on, onMsg, emit, off, search } from './core.js';
import { readJSON, writeJSON } from './store.js';

// The limits the server enforces too (internal/config/salvos.go); the
// shared cases in test/salvo_cases.json keep the two from drifting.
export const MAX_SALVOS = 30;
export const MAX_ITEMS = 100;
export const MAX_DSTS = 100;
export const MAX_TOTAL = 200;
export const MAX_NUM = 0xFFFE;   // a source or destination number
export const MAX_LEVEL = 64;     // a UI level
export const MAX_BYTES = 48 * 1024; // all salvos as JSON
const KEY = 'sode-salvos';
const PINS = 'sode-salvo-pins'; // this browser's pin overrides of everyone's salvos { lowercase name: bool }

// Numbers only, duplicates dropped; out-of-range values stay so that
// salvoProblem can name them rather than quietly changing the salvo.
const ints = (a) => [].concat(a ?? []).map(Number).filter((n) => Number.isInteger(n));
export const normItem = (it) => ({
  src: Number.isInteger(+it?.src) ? +it.src : -1,
  dsts: [...new Set(ints(it?.dsts))],
  levels: [...new Set(ints(it?.levels))].sort((a, b) => a - b),
});
export const normSalvo = (s) => ({
  name: String(s?.name ?? '').trim(),
  items: [].concat(s?.items ?? []).map(normItem),
});

// pinned: shown in the strip above Recent; a salvo saved before the flag existed is pinned.
// A salvo as it is stored and sent: no scope.
export const saved = ({ name, items, pinned }) => ({ name, items, pinned: pinned !== false });

export const globalSalvos = () => (S.cfg?.salvos || []).filter((s) => s?.name).map((s) => ({ scope: 'global', ...normSalvo(s), pinned: s.pinned !== false }));
export const localSalvos = () => [].concat(readJSON(KEY, [])).filter((s) => s?.name).map((s) => ({ scope: 'local', ...normSalvo(s), pinned: s.pinned !== false }));
export const allSalvos = () => [...globalSalvos(), ...localSalvos()];
export const findSalvo = (scope, name) =>
  allSalvos().find((s) => s.scope === scope && s.name.toLowerCase() === String(name).toLowerCase());

// Why a salvo (normalised) can't be saved, or ''. The name is checked with
// nameProblem (core.js) and the list as a whole with salvosProblem.
export function salvoProblem(s) {
  if (!s.items.length) return 'Add at least one take.';
  if (s.items.length > MAX_ITEMS) return `At most ${MAX_ITEMS} sources.`;
  let total = 0;
  for (const it of s.items) {
    if (it.src < 0) return 'Every row needs a source.';
    if (it.src > MAX_NUM) return `Source ${it.src + off()} is out of range.`;
    if (!it.dsts.length) return 'Every row needs a destination.';
    if (it.dsts.length > MAX_DSTS) return `At most ${MAX_DSTS} destinations per source.`;
    const d = it.dsts.find((n) => n < 0 || n > MAX_NUM);
    if (d != null) return `Destination ${d + off()} is out of range.`;
    const l = it.levels.find((u) => u < 1 || u > MAX_LEVEL);
    if (l != null) return `Level ${l} must be 1-${MAX_LEVEL}.`;
    total += it.dsts.length;
  }
  if (total > MAX_TOTAL) return `A preset can take at most ${MAX_TOTAL} destinations in total.`;
  return salvoConflict(s);
}

// Why this list of salvos (as it would be saved) is too much, or ''.
export function salvosProblem(list) {
  if (list.length > MAX_SALVOS) return `At most ${MAX_SALVOS} presets.`;
  const bytes = JSON.stringify(list.map(saved)).length;
  return bytes > MAX_BYTES ? `The presets are too big to save (${Math.ceil(bytes / 1024)} KiB, at most ${MAX_BYTES / 1024} KiB).` : '';
}

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

// list ({ name, items } each) with s replacing the salvo named `old`, or
// added when old is '' (or no longer there).
export function replaceSalvo(list, s, old = '') {
  const next = list.map(saved);
  const at = next.findIndex((x) => x.name.toLowerCase() === old.toLowerCase());
  const entry = saved(s);
  if (old && at >= 0) next[at] = entry;
  else next.push(entry);
  return next;
}

// Replaces this browser's salvo `old` (or adds, when old is '') with s.
export function saveLocalSalvo(s, old = '') {
  writeJSON(KEY, replaceSalvo(localSalvos(), s, old));
  emit('salvos');
}
export function deleteLocalSalvo(name) {
  const list = localSalvos().filter((s) => s.name.toLowerCase() !== name.toLowerCase()).map(saved);
  writeJSON(KEY, list.length ? list : null);
  emit('salvos');
}

// ---- pinning: the strip above Recent shows pinned salvos only. A salvo
// carries its own flag (everyone's in the config, this browser's own in its
// storage); a browser may override it for everyone's salvos, by name.

const pinKey = (name) => String(name).toLowerCase();
// This browser's overrides, names it still has only (an unknown name, or a
// value that isn't true/false, is ignored). list: salvos with scope.
export function pinOverrides(list = globalSalvos()) {
  const raw = readJSON(PINS, {});
  const out = {};
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return out;
  for (const s of list) {
    if (s.scope !== 'global') continue;
    const v = raw[pinKey(s.name)];
    if (typeof v === 'boolean') out[pinKey(s.name)] = v;
  }
  return out;
}
// Whether s shows in the strip: this browser's override of an everyone
// salvo, else the salvo's own flag.
export function effectivePin(s, overrides = pinOverrides()) {
  if (s.scope === 'global') {
    const v = overrides[pinKey(s.name)];
    if (typeof v === 'boolean') return v;
  }
  return s.pinned !== false;
}
export function pinnedSalvos(list = allSalvos()) {
  const o = pinOverrides(list);
  return list.filter((s) => effectivePin(s, o));
}
// Pins or unpins s for this browser only. A global salvo gets an override
// (dropped again when it equals the salvo's own flag); a local one changes
// its flag.
export function setLocalPin(s, on) {
  if (s.scope === 'global') {
    const raw = readJSON(PINS, {});
    const o = raw && typeof raw === 'object' && !Array.isArray(raw) ? { ...raw } : {};
    if (on === (s.pinned !== false)) delete o[pinKey(s.name)];
    else o[pinKey(s.name)] = on;
    writeJSON(PINS, Object.keys(o).length ? o : null);
  } else {
    writeJSON(KEY, localSalvos().map((x) => saved(x.name.toLowerCase() === s.name.toLowerCase() ? { ...x, pinned: on } : x)));
  }
  emit('salvos');
}
// After a salvo is pinned or unpinned for everyone this browser follows it.
export function clearPinOverride(name) {
  const raw = readJSON(PINS, {});
  if (!raw || typeof raw !== 'object' || !(pinKey(name) in raw)) return;
  const o = { ...raw };
  delete o[pinKey(name)];
  writeJSON(PINS, Object.keys(o).length ? o : null);
  emit('salvos');
}
// Everyone's list with salvo `name` pinned or not, to send.
export const withGlobalPin = (name, on) =>
  globalSalvos().map((s) => saved(s.name.toLowerCase() === name.toLowerCase() ? { ...s, pinned: on } : s));

// ---- the popup's list: all salvos, or the best matches for what is typed
// (names, by core.js search). Matches keep their order among equal scores.
export function rankSalvos(list, q) {
  if (!String(q).trim()) return list;
  const items = list.map((s, n) => ({ num: '', lname: s.name.toLowerCase(), n }));
  return search(items, q, list.length).map((it) => list[it.n]);
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
  const take = (x) => !x.ext && x.undo == null && x.retry == null;
  const e = hist.find(take);
  if (!e) return [];
  const es = e.group && e.groupN > 1 ? hist.filter((x) => x.group === e.group && take(x)) : [e];
  return es.slice().reverse().map((x) => ({ src: x.src, dst: x.dst, levels: x.levels }));
}

// Global salvos changed for everyone.
onMsg('salvos', (m) => {
  S.cfg = { ...S.cfg, salvos: m.salvos };
  emit('salvos');
});
on('helloData', () => emit('salvos'));
