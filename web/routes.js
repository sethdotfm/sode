// Routing views: who feeds a destination and what a source feeds, as rows
// of the level bar and a route.

import { el, S, tallyText, protectedOn, protectedLabel } from './core.js';
import { levelBar, allLevels } from './levelview.js';

// One entry per source feeding dst on any of levels, in order of each
// source's first level: [{src, levels}]. src null = unknown, -1 = nothing.
export function bySource(dst, levels) {
  const t = S.tally.get(dst) || new Map();
  const groups = new Map();
  for (const u of levels) {
    const s = t.has(u) ? t.get(u) : null;
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  return [...groups].map(([src, lv]) => ({ src, levels: lv }));
}

// Destinations src feeds on any of levels, sorted.
export function fedBy(src, levels) {
  const fed = [];
  for (const [d, t] of S.tally) {
    if (levels.some((u) => t.get(u) === src)) fed.push(d);
  }
  return fed.sort((a, b) => a - b);
}

// A source's end of a route: a button that calls onPick(src) if there is an
// onPick and the source is routed (src >= 0; null is unknown, -1 nothing),
// else plain text.
export const srcEnd = (src, text, onPick) => (onPick && src != null && src >= 0
  ? pickEnd('src', text, 'Use as source', () => onPick(src))
  : el('span', 'sode__end sode__end--src', text));

export const srcRow = (levels, srcText, src = null, onPick = null) =>
  el('div', 'sode__route', levelBar(levels), srcEnd(src, srcText, onPick));

export function routeRow(levels, srcText, dstText) {
  return el('div', 'sode__route',
    levelBar(levels),
    el('span', 'sode__end sode__end--src', srcText),
    el('span', 'sode__arrow', '→'),
    el('span', 'sode__end sode__end--dst', dstText));
}

// A route row's end as a button, for lists where a name picks it.
export function pickEnd(side, text, title, onPick) {
  const b = el('button', `pb-btn pb-btn--ghost pb-btn--sm sode__end sode__end--${side}`, text);
  b.type = 'button';
  b.tabIndex = -1;
  b.title = title;
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', onPick);
  return b;
}

// The whole of dst: a heading, one row per source, and any protect.
// onPick(src), if given, makes each routed source a button.
export function currentlyNodes(dst, onPick) {
  const all = allLevels();
  const rows = bySource(dst.n, all).map((r) => srcRow(r.levels, tallyText(r.src), r.src, onPick));
  const p = protectedOn(dst.n, all);
  const prot = p.levels.length
    ? el('div', 'sode__route', levelBar(p.levels), el('span', 'pb-badge pb-badge--warn', protectedLabel(p.devices)))
    : null;
  return [el('div', 'sode__label', `Currently on ${dst.label}`), ...rows, prot].filter(Boolean);
}
