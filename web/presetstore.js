// Presets: { id, name, items }. Global ones live in cfg.presets (a Trusted user changes them), this
// browser's own in localStorage. Stored items hold router references (presetref.js); in memory they
// are numbers (internal numbers, UI levels, [] = default levels). The hidden id survives a rename,
// so pins and pin order name a preset by it.

import { S, on, onMsg, emit, off, search } from './core.js';
import { readJSON, writeJSON } from './store.js';
import { profile, profileName, saveProfile } from './profile.js';
import { profileProblem } from './profilesrules.js';
import { itemNums, itemPlain, itemRefs, newPresetId, storedItemNums } from './presetref.js';

// Limits the server enforces too (internal/config/presets.go); test/preset_cases.json keeps the
// two from drifting.
const MAX_PRESETS = 30;
const MAX_ITEMS = 100;
const MAX_DSTS = 100;
const MAX_TOTAL = 200;
const MAX_NUM = 0xfffe;
const MAX_LEVEL = 64;
export const MAX_BYTES = 48 * 1024;
const KEY = 'sode-presets';
const ORDER = 'sode-preset-order';
const PINS = 'sode-preset-pins';

export const normPreset = (s, read = itemNums) => ({
  id: typeof s?.id === 'string' ? s.id.trim() : '',
  name: String(s?.name ?? '').trim(),
  items: [].concat(s?.items ?? []).map(read),
  // The server's Broken marks ride along, so a fire leaves those parts out (presetPlan). Never sent back.
  ...(Array.isArray(s?.broken) && s.broken.length ? { broken: s.broken } : {}),
});
// A stored preset (server, profile, This browser, a file): a bare number is no reference.
export const storedPreset = (s) => normPreset(s, storedItemNums);

const liveName = (kind, n) => (kind === 'src' ? S.srcBy : S.dstBy)?.get(n)?.name || '';
// The stored and sent form: references named as the router names them now. A preset saved before
// the pinned flag existed is pinned. Stored and sent without scope. A new Everyone or Me preset
// goes with id '' and the server makes one.
export const saved = ({ id, name, items, pinned }) => ({
  id: id || '',
  name,
  items: items.map((it) => itemRefs(itemNums(it), S.routerUid, liveName)),
  pinned: pinned !== false,
});
const withId = (s) => (s.id ? s : { ...s, id: newPresetId() });

export const globalPresets = () =>
  (S.cfg?.presets || []).filter((s) => s?.name).map((s) => ({ scope: 'global', ...storedPreset(s), pinned: s.pinned !== false }));
export const localPresets = () =>
  []
    .concat(readJSON(KEY, []))
    .filter((s) => s?.name)
    .map((s) => ({ scope: 'local', ...storedPreset(s), pinned: s.pinned !== false }));
export const userPresets = () =>
  (profileName() ? profile().presets : []).filter((s) => s?.name).map((s) => ({ scope: 'user', ...storedPreset(s), pinned: s.pinned !== false }));
export const allPresets = () => [...globalPresets(), ...userPresets(), ...localPresets()];
const PRESET_LISTS = { global: globalPresets, user: userPresets, local: localPresets };
export const presetsIn = (scope) => (PRESET_LISTS[scope] || (() => []))();
export const findPresetById = (scope, id) => (id ? allPresets().find((s) => s.scope === scope && s.id === id) : undefined);
export const findPreset = (scope, name) => allPresets().find((s) => s.scope === scope && s.name.toLowerCase() === String(name).toLowerCase());

export function presetProblem(s) {
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
  if (total > MAX_TOTAL) return `A preset can hold at most ${MAX_TOTAL} destinations in total.`;
  return presetConflict(s);
}

export function presetsProblem(list) {
  if (list.length > MAX_PRESETS) return `At most ${MAX_PRESETS} presets.`;
  // A new preset has no id until the server makes one; count it as one.
  const bytes = JSON.stringify(list.map((s) => saved(s.id ? s : { ...s, id: 'x'.repeat(16) }))).length;
  return bytes > MAX_BYTES ? `The presets are too big to save (${Math.ceil(bytes / 1024)} KiB, at most ${MAX_BYTES / 1024} KiB).` : '';
}

// Different sources to one destination on overlapping levels (blank = default, overlaps
// everything) would race. The same source twice merges, so is fine.
function presetConflict(s) {
  const clash = (a, b) => !a.length || !b.length || a.some((l) => b.includes(l));
  for (let i = 0; i < s.items.length; i++) {
    for (const b of s.items.slice(i + 1)) {
      const a = s.items[i];
      if (a.src === b.src || !clash(a.levels, b.levels)) continue;
      const d = a.dsts.find((x) => b.dsts.includes(x));
      if (d != null) {
        return `Destination ${d + off()} gets both source ${a.src + off()} and source ${b.src + off()} on the same levels.`;
      }
    }
  }
  return '';
}

// old: the name of the entry s replaces. A replaced entry keeps its id unless s has its own.
export function replacePreset(list, s, old = '') {
  const next = list.map(saved);
  const at = old ? next.findIndex((x) => x.name.toLowerCase() === old.toLowerCase()) : -1;
  const entry = saved(s);
  if (at >= 0) next[at] = { ...entry, id: entry.id || next[at].id };
  else next.push(entry);
  return next;
}

export function saveLocalPreset(s, old = '') {
  writeJSON(KEY, replacePreset(localPresets(), s, old).map(withId));
  emit('presets');
}
// As the server's WithPresetExtras: an entry without an id gets the id of this browser's preset
// of its name. That id must be free in the list. So a Replace import keeps pins and order.
export function putLocalPresets(list) {
  const was = localPresets();
  const taken = new Set(list.map((s) => s.id).filter(Boolean));
  const kept = list.map((s) => {
    if (s.id) return s;
    const old = was.find((x) => x.name.toLowerCase() === String(s.name).trim().toLowerCase());
    if (!old?.id || taken.has(old.id)) return s;
    taken.add(old.id);
    return { ...s, id: old.id };
  });
  writeJSON(KEY, kept.length ? kept.map(saved).map(withId) : null);
  emit('presets');
}
export function deleteLocalPreset(name) {
  const list = localPresets()
    .filter((s) => s.name.toLowerCase() !== name.toLowerCase())
    .map(saved);
  writeJSON(KEY, list.length ? list : null);
  emit('presets');
}

// ---- pinning: a preset carries its own flag; a browser can override it for everyone's presets, by id.

// Ids it still has only; an unknown id or a non-boolean value is ignored.
export function pinOverrides(list = globalPresets()) {
  const raw = readJSON(PINS, {});
  const out = {};
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return out;
  for (const s of list) {
    if (s.scope !== 'global' || !s.id) continue;
    const v = raw[s.id];
    if (typeof v === 'boolean') out[s.id] = v;
  }
  return out;
}
export function effectivePin(s, overrides = pinOverrides()) {
  if (s.scope === 'global') {
    const v = overrides[s.id];
    if (typeof v === 'boolean') return v;
  }
  return s.pinned !== false;
}
// Pin order keys (Profile.PinOrder, sode-preset-order) name a preset by scope and id.
export const presetKey = (s) => `${s.scope}:${s.id}`;
export function pinOrder() {
  const raw = profileName() ? profile().pinOrder : readJSON(ORDER, []);
  return Array.isArray(raw) ? raw.filter((k) => typeof k === 'string') : [];
}
export function orderPins(list, order = pinOrder()) {
  const at = new Map(order.map((k, i) => [k, i]));
  const rank = (s) => at.get(presetKey(s)) ?? Infinity;
  return list
    .map((s, i) => [s, i])
    .sort((a, b) => (rank(a[0]) === rank(b[0]) ? a[1] - b[1] : rank(a[0]) - rank(b[0])))
    .map((x) => x[0]);
}
export function moveKey(keys, key, to) {
  const rest = keys.filter((k) => k !== key);
  rest.splice(Math.max(0, Math.min(to, rest.length)), 0, key);
  return rest;
}
// The order shows at once; a caller whose save fails redraws from the real one (saveProfile
// puts the old copy back).
export function setPinOrder(keys) {
  if (profileName()) {
    const why = profileProblem(profile(), { pinOrder: keys });
    if (why) return Promise.resolve({ ok: false, err: why });
    const saved = saveProfile({ pinOrder: keys });
    emit('presets');
    return saved;
  }
  writeJSON(ORDER, keys.length ? keys : null);
  emit('presets');
  return Promise.resolve({ ok: true });
}
export function movePin(key, where) {
  const keys = pinnedPresets().map(presetKey);
  const i = keys.indexOf(key);
  if (i < 0) return null;
  let to;
  if ('by' in where) {
    to = i + where.by;
    if (to < 0 || to >= keys.length) return null;
  } else {
    if (where.before === key) return null;
    to = keys.filter((k) => k !== key).indexOf(where.before);
    if (to < 0) return null;
  }
  return setPinOrder(moveKey(keys, key, to));
}
export function pinnedPresets(list = allPresets()) {
  const o = pinOverrides(list);
  return orderPins(list.filter((s) => effectivePin(s, o)));
}
// A global preset gets an override (dropped when it equals the preset's own flag). A scope
// 'user' preset is saved to the profile instead (withUserPin).
export function setLocalPin(s, on) {
  if (s.scope === 'user') return;
  if (s.scope === 'global') {
    const raw = readJSON(PINS, {});
    const o = raw && typeof raw === 'object' && !Array.isArray(raw) ? { ...raw } : {};
    if (on === (s.pinned !== false)) delete o[s.id];
    else o[s.id] = on;
    writeJSON(PINS, Object.keys(o).length ? o : null);
  } else {
    writeJSON(
      KEY,
      localPresets().map((x) => withId(saved(x.id === s.id ? { ...x, pinned: on } : x))),
    );
  }
  emit('presets');
}
export function clearPinOverride(id) {
  const raw = readJSON(PINS, {});
  if (!raw || typeof raw !== 'object' || !(id in raw)) return;
  const o = { ...raw };
  delete o[id];
  writeJSON(PINS, Object.keys(o).length ? o : null);
  emit('presets');
}
export const withGlobalPin = (id, on) => globalPresets().map((s) => saved(s.id === id ? { ...s, pinned: on } : s));

export const withUserPin = (id, on) => userPresets().map((s) => saved(s.id === id ? { ...s, pinned: on } : s));

// ---- the popup's list: all presets, or the best matches (core.js search). Matches keep their order among equal scores.
export function rankPresets(list, q) {
  if (!String(q).trim()) return list;
  const items = list.map((s, n) => ({ num: '', lname: s.name.toLowerCase(), n }));
  return search(items, q, list.length).map((it) => list[it.n]);
}

// ---- typed references: a bare number or range a-b is a display number (internal + off()), anything else is looked up by name with lookup(token). → { ns: [internal], bad: [tokens] }

export function parseRefs(text, offset, lookup) {
  const ns = [],
    bad = [];
  const add = (n) => {
    if (!ns.includes(n)) ns.push(n);
  };
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

export const isBlankRow = (r) => !String(r.src).trim() && !String(r.dsts).trim() && !String(r.levels).trim();

export function presetTip(s, srcLabel, dstLabel) {
  const srcs = [...new Set(s.items.map((it) => it.src))];
  const dsts = [...new Set(s.items.flatMap((it) => it.dsts))];
  if (!srcs.length || !dsts.length) return 'No takes';
  const from = srcs.length === 1 ? srcLabel(srcs[0]) : `${srcs.length} Sources`;
  const to = dsts.length === 1 ? dstLabel(dsts[0]) : `${dsts.length} Destinations`;
  return `${from} to ${to}`;
}

// has: { src(n), dst(n), level(u) }; defaults: default levels (UI). Returns takes grouped per
// destination plus missing = [{ kind: 'source' | 'destination' | 'level', n }].
export function resolvePreset(preset, has, defaults) {
  const byDst = new Map();
  const missing = [];
  const miss = (kind, n) => {
    if (!missing.some((m) => m.kind === kind && m.n === n)) missing.push({ kind, n });
  };
  for (const it of preset.items) {
    if (!has.src(it.src)) {
      miss('source', it.src);
      continue;
    }
    const want = it.levels.length ? it.levels : defaults;
    const levels = want.filter((u) => has.level(u));
    for (const u of want) if (!has.level(u)) miss('level', u);
    if (!levels.length) continue;
    for (const d of it.dsts) {
      if (!has.dst(d)) {
        miss('destination', d);
        continue;
      }
      if (!byDst.has(d)) byDst.set(d, []);
      const groups = byDst.get(d);
      const g = groups.find((x) => x.src === it.src);
      if (g) g.levels = [...new Set([...g.levels, ...levels])].sort((a, b) => a - b);
      else groups.push({ src: it.src, levels: [...levels] });
    }
  }
  return { routes: [...byDst].map(([dst, groups]) => ({ dst, groups })), missing };
}

// What Shift-click fills: the first take that still exists, built on resolvePreset so the rules
// match a fire. { fill, partial, many, missing }: levels [] = default. partial: something of
// that take is gone. many: more takes than the boxes hold. missing: what the router lacks,
// listed when no take fits.
export function fillFrom(preset, has, defaults) {
  const missing = [];
  for (const it of preset.items) {
    const r = resolvePreset({ items: [it] }, has, defaults);
    if (r.routes.length) {
      return {
        fill: { src: it.src, dsts: r.routes.map((t) => t.dst), levels: it.levels.length ? r.routes[0].groups[0].levels : [] },
        partial: r.missing.length > 0,
        many: preset.items.length > 1,
        missing: [],
      };
    }
    missing.push(...r.missing);
  }
  return { fill: null, partial: false, many: preset.items.length > 1, missing };
}

export function itemsFromEntries(entries) {
  const by = new Map();
  for (const t of entries) {
    const key = `${t.src}|${t.levels.join(',')}`;
    if (!by.has(key)) by.set(key, { src: t.src, dsts: [], levels: [...t.levels] });
    const it = by.get(key);
    if (!it.dsts.includes(t.dst)) it.dsts.push(t.dst);
  }
  return [...by.values()].map(itemPlain);
}

onMsg('presets', (m) => {
  S.cfg = { ...S.cfg, presets: m.presets };
  emit('presets');
});
on('helloData', () => emit('presets'));
on('profile', () => emit('presets'));
