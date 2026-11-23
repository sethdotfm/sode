// Routing views: who feeds a destination and what a source feeds.

import { el, routerData, routerOfItem, tallyText } from './core.js';
import { protectNodes } from './protects.js';
import { levelBar, allLevels } from './levelview.js';

// src null = unknown, -1 = nothing. uid: the router (routerData); blank is the first router.
export function bySource(dst, levels, uid = '') {
  const t = routerData(uid).tally.get(dst) || new Map();
  const groups = new Map();
  for (const u of levels) {
    const s = t.has(u) ? t.get(u) : null;
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  return [...groups].map(([src, lv]) => ({ src, levels: lv }));
}

export function fedBy(src, levels, uid = '') {
  const fed = [];
  for (const [d, t] of routerData(uid).tally) {
    if (levels.some((u) => t.get(u) === src)) fed.push(d);
  }
  return fed.sort((a, b) => a - b);
}

// Button when onPick is given and src is routed (>= 0; null unknown, -1 nothing), else plain text.
export const srcEnd = (src, text, onPick) =>
  onPick && src != null && src >= 0 ? pickEnd('src', text, 'Use as source', () => onPick(src)) : el('span', 'sode__end sode__end--src', text);

export const srcRow = (levels, srcText, src = null, onPick = null) => el('div', 'sode__route', levelBar(levels), srcEnd(src, srcText, onPick));

export function routeRow(levels, srcText, dstText) {
  return el(
    'div',
    'sode__route',
    levelBar(levels),
    el('span', 'sode__end sode__end--src', srcText),
    el('span', 'sode__arrow', '→'),
    el('span', 'sode__end sode__end--dst', dstText),
  );
}

export function pickEnd(side, text, title, onPick) {
  const b = el('button', `sode__end sode__end--${side} sode__end--pick`, text);
  b.type = 'button';
  b.tabIndex = -1;
  b.dataset.pbTip = title;
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', onPick);
  return b;
}

// confirm: the route view of one destination with no source offers Protect or Free Protect through it. onPick gets the
// source number and the router of dst.
export function currentlyNodes(dst, onPick, confirm = null) {
  const all = allLevels();
  const uid = routerOfItem(dst);
  const pick = onPick && ((s) => onPick(s, uid));
  const rows = bySource(dst.n, all, uid).map((r) => srcRow(r.levels, tallyText(r.src, uid), r.src, pick));
  return [el('div', 'sode__label', `Currently on ${dst.label}`), ...rows, ...protectNodes(dst, confirm)];
}
