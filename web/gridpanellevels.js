// Levels on a Grid panel. Pure; levels and numbering come only from the adapter's GET /tally answer, never from S.
import { levelStyle, parseLevelsIn } from './levels.js';
import { levelsOn } from './routerlevels.js';

// The router's levels the Rights allow; the Levels box offers these only.
// Rights levels name their router: tally.router.
export function offeredLevels(tally, rights) {
  const allowed = levelsOn(rights?.levels, tally?.router);
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
  const want = levelsOn(settings?.levels, tally?.router);
  if (!want?.length) return null;
  const list = stageLevels(want, tally, rights);
  return list.length ? list : null;
}

// What a press stages on a router after the first. The Levels box holds levels of the first router. So a route on
// another router takes the box's levels that its router has, inside its Rights and the panel's level subset for it.
// An empty box is the Default levels: the panel's subset for that router, else its own Default levels. t is that
// router's part of the tally (tallyAt in routerids.js), or null while it has not come: then nothing stages.
export function otherRouterLevels(box, boxEmpty, t, rights, settings) {
  if (!t) return [];
  const offered = offeredLevels(t, rights).map((l) => l.level);
  const own = levelsOn(settings?.levels, t.router);
  const subset = own?.length ? own : null;
  const want = boxEmpty ? subset || parseLevelsIn('', { levels: t.levels, numbering: t.levelNumbering, defaultLevels: t.defaultLevels || '' }) || [] : box;
  // A router with one level takes any levels as its one level (levels.js parseLevelsIn).
  const list = t.levels.length === 1 && want.length ? [1] : want;
  return list.filter((l) => offered.includes(l) && (!subset || subset.includes(l)));
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

// A fixed slice per router level, so a slice's place tells the level and color is never the only signal.
// '' when no level is on.
export function pieFill(levels, on) {
  if (!on.length) return '';
  const n = levels.length;
  const parts = levels.map((l, i) => `${on.includes(l.level) ? levelStyle(l).color : 'transparent'} ${turn(i / n)} ${turn((i + 1) / n)}`);
  return `conic-gradient(${parts.join(', ')})`;
}
