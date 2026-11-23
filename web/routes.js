// Routing views: who feeds a destination and what a source feeds.

import { el, routerData, routerOfItem, tallyText, endSrcLabel, viaByRouter } from './core.js';
import { protectNodes } from './protects.js';
import { levelBar, allLevels } from './levelview.js';
import { viaGroups, originText, fedThrough } from './viarules.js';
import { viaMark, pieRows } from './viaview.js';

// src null = unknown, -1 = nothing. `via`: the Via row of these levels when a tie line feeds src (viarules.js), else
// null. Levels with one source and two chains are two groups. uid: the router (routerData); blank is the first router.
export function bySource(dst, levels, uid = '') {
  const rd = routerData(uid);
  const t = rd.tally.get(dst) || new Map();
  const v = rd.via.get(dst);
  return viaGroups(
    levels,
    (u) => (t.has(u) ? t.get(u) : null),
    (u) => v?.get(u),
  );
}

export function fedBy(src, levels, uid = '') {
  const fed = [];
  for (const [d, t] of routerData(uid).tally) {
    if (levels.some((u) => t.get(u) === src)) fed.push(d);
  }
  return fed.sort((a, b) => a - b);
}

// fedThroughTies: what source item src feeds through tie lines, on every router: [{ router, dst, levels, row }]
// (viarules.js). router is a routerData UID: blank for the first router.
export const fedThroughTies = (src) =>
  fedThrough({ router: routerData(routerOfItem(src)).uid, n: src.n }, viaByRouter()).map((f) => ({ ...f, router: routerOfItem(f) }));

// Button when onPick is given and src is routed (>= 0; null unknown, -1 nothing), else plain text.
export const srcEnd = (src, text, onPick) =>
  onPick && src != null && src >= 0 ? pickEnd('src', text, 'Use as source', () => onPick(src)) : el('span', 'sode__end sode__end--src', text);

// viaRow: a source that tie lines feed, as its origin and the Via mark: "A.1 CAM 1 Via TIE 3". onPick(n, uid) picks
// the origin. A mixed chain adds its level pie under it.
export function viaRow(levels, via, onPick = null, uid = '') {
  const end = (o, text) => srcEnd(o ? o.n : null, text, o && onPick ? () => onPick(o.n, routerOfItem(o)) : null);
  const text = originText(via, endSrcLabel);
  return [el('div', 'sode__route', levelBar(levels, uid), ...(text ? [end(via.origin, text)] : []), viaMark(via)), ...pieRows(via, end)];
}

// uid: the router of the levels (routerData); blank is the first router.
export const srcRow = (levels, srcText, src = null, onPick = null, uid = '') => el('div', 'sode__route', levelBar(levels, uid), srcEnd(src, srcText, onPick));

export function routeRow(levels, srcText, dstText, uid = '') {
  return el(
    'div',
    'sode__route',
    levelBar(levels, uid),
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
  const uid = routerOfItem(dst);
  const all = allLevels(uid);
  const pick = onPick && ((s) => onPick(s, uid));
  const rows = bySource(dst.n, all, uid).flatMap((r) =>
    r.via ? viaRow(r.levels, r.via, onPick, uid) : [srcRow(r.levels, tallyText(r.src, uid), r.src, pick, uid)],
  );
  return [el('div', 'sode__label', `Currently on ${dst.label}`), ...rows, ...protectNodes(dst, confirm)];
}
