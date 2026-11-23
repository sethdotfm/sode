import { off, routerData } from './core.js';
import { allPresets } from './presetstore.js';
import { allSets } from './filterstore.js';
import { presetMarks, filterSetMarks, localPresetMarks, localFilterMarks, statusOf } from './brokenrules.js';

export { statusBadge } from './brokenbadge.js';

const nameMaps = () => {
  const map = (by) => new Map([...(by || new Map())].map(([n, it]) => [n, it?.name || '']));
  return { src: map(routerData('').srcBy), dst: map(routerData('').dstBy) };
};

export const presetKey = (s) => `preset:${s.scope}:${s.id || s.name}`;
export const setKey = (s) => `filterSet:${s.scope}:${s.id}`;

export function presetStatus(s, names = null) {
  const m = s.scope === 'local' ? localPresetMarks(s, names || nameMaps(), routerData('').uid) : presetMarks(s);
  return { ...m, status: statusOf(m) };
}

export function setStatus(s, names = null) {
  const m = s.scope === 'local' ? localFilterMarks(s, names || nameMaps(), routerData('').uid) : filterSetMarks(s, off());
  return { ...m, status: statusOf(m) };
}

// key -> {key, kind, scope, id, name, broken, moved, status}
export function pageItems() {
  const names = nameMaps();
  const out = new Map();
  for (const s of allPresets()) {
    const k = presetKey(s);
    out.set(k, { key: k, kind: 'preset', scope: s.scope, id: s.id, name: s.name, ...presetStatus(s, names) });
  }
  for (const s of allSets()) {
    if (s.scope === 'all') continue;
    const k = setKey(s);
    out.set(k, { key: k, kind: 'filterSet', scope: s.scope, id: s.id, name: s.name, ...setStatus(s, names) });
  }
  return out;
}
