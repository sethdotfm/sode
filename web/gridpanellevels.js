// Levels on a Grid panel. Pure; levels and numbering come only from the adapter's GET /tally answer, never from S.
import { levelStyle } from './levels.js';

// The router's levels the Rights allow; the Levels box offers these only.
export function offeredLevels(tally, rights) {
  const allowed = Array.isArray(rights?.levels) ? rights.levels : null;
  return (tally?.levels || []).filter((l) => !allowed || allowed.includes(l.level));
}

// What a press stages: the list in the Levels box, or null when its text is invalid. Kept to the offered levels.
// The router's levels and the Rights can change under typed text.
export function stageLevels(list, tally, rights) {
  const have = offeredLevels(tally, rights).map((l) => l.level);
  return (list || []).filter((l) => have.includes(l));
}

// The panel's levels setting as the box's first content, or null to leave the box empty (the Default levels).
export function boxFill(settings, tally, rights) {
  if (!Array.isArray(settings?.levels) || !settings.levels.length) return null;
  const list = stageLevels(settings.levels, tally, rights);
  return list.length ? list : null;
}

// env is envFrom's (panelrules.js). The levels on which dst has src now.
export function pieOn(env, src, dst) {
  const row = env.tally.get(dst);
  if (!row) return [];
  return [...row]
    .filter(([, s]) => s === src)
    .map(([l]) => l)
    .sort((a, b) => a - b);
}

const turn = (x) => `${Number(x.toFixed(4))}turn`;

// A fixed slice per router level, so a slice's place tells the level and colour is never the only signal.
// '' when no level is on.
export function pieFill(levels, on) {
  if (!on.length) return '';
  const n = levels.length;
  const parts = levels.map((l, i) => `${on.includes(l.level) ? levelStyle(l).color : 'transparent'} ${turn(i / n)} ${turn((i + 1) / n)}`);
  return `conic-gradient(${parts.join(', ')})`;
}
