// Broken and Name moved in the page's lists and the Main-page banner (rules in brokenrules.js). The page's own
// presets and filter sets (Everyone, Me, This browser) with their marks now, by banner key.
import { S, el, icon, off } from './core.js';
import { allPresets } from './presetstore.js';
import { allSets } from './filterstore.js';
import { presetMarks, filterSetMarks, localPresetMarks, localFilterMarks, statusOf, STATUS_TEXT } from './brokenrules.js';

const ICON = { broken: 'unlink', moved: 'arrow-right-left' };

// A status icon with its text, or null for an item with no marks.
export function statusBadge(status) {
  if (!status) return null;
  const b = el('span', `pb-badge ${status === 'broken' ? 'pb-badge--warn' : 'pb-badge--info'} sode__status-badge`, icon(ICON[status]), STATUS_TEXT[status]);
  b.dataset.status = status;
  return b;
}

const nameMaps = () => {
  const map = (by) => new Map([...(by || new Map())].map(([n, it]) => [n, it?.name || '']));
  return { src: map(S.srcBy), dst: map(S.dstBy) };
};

export const presetKey = (s) => `preset:${s.scope}:${s.id || s.name}`;
export const setKey = (s) => `filterSet:${s.scope}:${s.name}`;

export function presetStatus(s, names = null) {
  const m = s.scope === 'local' ? localPresetMarks(s, names || nameMaps(), S.routerUid) : presetMarks(s);
  return { ...m, status: statusOf(m) };
}

export function setStatus(s, names = null) {
  const m = s.scope === 'local' ? localFilterMarks(s, names || nameMaps(), S.routerUid) : filterSetMarks(s, off());
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
    out.set(k, { key: k, kind: 'filterSet', scope: s.scope, name: s.name, ...setStatus(s, names) });
  }
  return out;
}
