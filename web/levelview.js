// Levels on screen and in boxes (reads S). The pure parts, with levels and numbering as arguments, are in levels.js.

import { el, S, on, routerData } from './core.js';
import { levelLabelOf, levelInputOf, levelTextOf, parseLevelsIn, levelsBoxShown } from './levels.js';
import { levelColor, levelPicks } from './levelbox.js';

export { levelColor };

// Levels belong to a router: uid is its routerData UID, blank for the first router. Typed levels of a route are levels
// of the router of its destination.

// What levelBox reads: null before the hello has the config. prev is the first router's numbering before the last hello.
export const levelSource = (uid = '') => (S.cfg ? { ...parseCtx(uid), prev: routerData(uid).first ? S.prevNumbering : null } : null);

export const allLevels = (uid = '') => routerData(uid).levels.map((l) => l.ui);

// A router with one level cannot break away: its routes are on all levels.
export const oneLevel = (uid = '') => routerData(uid).levels.length === 1;

// The desktop main page's Levels box: hidden while no router has more than one level. Focus paths into it check this.
const levelCounts = () => (S.routers.length ? S.routers.map((r) => routerData(r.uid).levels.length) : [routerData('').levels.length]);
export const levelsShown = () => levelsBoxShown(S.cfg ? levelCounts() : []);

// The patch row's columns follow it (desktop.css .sode__boxes[data-levels]).
on('hello', () => {
  const row = document.querySelector('.sode__boxes');
  if (row) row.dataset.levels = levelsShown() ? 'on' : 'off';
});

// Display and typing follow the router's Level numbering; internally 1..N.
const numbering = () => routerData('').numbering;
const parseCtx = (uid) => {
  const rt = routerData(uid);
  return { levels: rt.levels, numbering: rt.numbering, defaultLevels: rt.defaultLevels };
};

// No mode: the server's Level.Label; a mode previews an unsaved numbering.
export const levelLabel = (ui, mode) => levelLabelOf(routerData('').levels, numbering(), ui, mode);
export const parseLevels = (text, none = false, uid = '') => parseLevelsIn(text, parseCtx(uid), none);
export const levelText = (list, uid = '') => levelTextOf(list, routerData(uid).levels, routerData(uid).numbering);
// "levels 1–4", or "all levels" on a router with one level.
export const levelsPhrase = (list, uid = '') => (oneLevel(uid) ? 'all levels' : `level${list.length === 1 ? '' : 's'} ${levelText(list, uid)}`);
export const levelInput = (list, mode) => levelInputOf(list, routerData('').levels, numbering(), mode);

export const defaultLevelsWord = (uid = '') => ((parseLevels('', false, uid) || []).length === routerData(uid).levels.length ? 'All' : 'default');

// Fixed row of slots, filled when on, so rows line up and color is never the only signal. A router with one level shows
// "All levels" in place of the row.
export function levelBar(on, uid = '') {
  const levels = routerData(uid).levels;
  if (levels.length === 1) {
    const all = el('span', 'sode__bar sode__bar--all', 'All levels');
    all.style.setProperty('--sode-bar-cols', 1);
    return all;
  }
  const set = new Set(on);
  const row = el(
    'span',
    'sode__bar',
    ...levels.map((l) => {
      const slot = el('span', 'sode__slot' + (set.has(l.ui) ? ' sode__slot--on' : ''), l.label);
      slot.dataset.hint = `Level ${l.label}`; // where the slot is too small for its label
      return levelColor(slot, l);
    }),
  );
  // Two equal rows; an odd first level (0 or V) stands on its own to the left.
  const odd = levels.length % 2;
  if (odd) row.classList.add('sode__bar--odd');
  row.style.setProperty('--sode-bar-cols', Math.floor(levels.length / 2) + odd);
  const sorted = [...set].sort((a, b) => a - b);
  row.setAttribute('role', 'img');
  row.setAttribute('aria-label', sorted.length ? `Levels ${levelText(sorted, uid)}` : 'No levels');
  return row;
}

export const pickButtons = (onToggle) => levelPicks(routerData('').levels, onToggle);
