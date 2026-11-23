// Levels on screen and in boxes (reads S). The pure parts, with levels and numbering as arguments, are in levels.js.

import { el, S, on } from './core.js';
import { levelLabelOf, levelInputOf, levelTextOf, parseLevelsIn, levelsBoxShown } from './levels.js';
import { levelColor, levelPicks } from './levelbox.js';

export { levelColor };

// What levelBox reads: null before the hello has the config.
export const levelSource = () => (S.cfg ? { ...parseCtx(), prev: S.prevNumbering } : null);

export const allLevels = () => S.levels.map((l) => l.ui);

// The desktop main page's Levels box: hidden while no router has more than one level. Focus paths into it check this.
export const levelsShown = () => levelsBoxShown(S.cfg ? [S.levels.length] : []);

// The patch row's columns follow it (desktop.css .sode__boxes[data-levels]).
on('hello', () => {
  const row = document.querySelector('.sode__boxes');
  if (row) row.dataset.levels = levelsShown() ? 'on' : 'off';
});

// Display and typing follow cfg.levelNumbering; internally 1..N.
const numbering = () => S.cfg.levelNumbering;
const parseCtx = () => ({ levels: S.levels, numbering: numbering(), defaultLevels: S.cfg.defaultLevels });

// No mode: the server's Level.Label; a mode previews an unsaved numbering.
export const levelLabel = (ui, mode) => levelLabelOf(S.levels, numbering(), ui, mode);
export const parseLevels = (text, none = false) => parseLevelsIn(text, parseCtx(), none);
export const levelText = (list) => levelTextOf(list, S.levels, numbering());
export const levelInput = (list, mode) => levelInputOf(list, S.levels, numbering(), mode);

export const defaultLevelsWord = () => ((parseLevels('') || []).length === S.levels.length ? 'All' : 'default');

// Fixed row of slots, filled when on, so rows line up and color is never the only signal.
export function levelBar(on) {
  const set = new Set(on);
  const row = el(
    'span',
    'sode__bar',
    ...S.levels.map((l) => {
      const slot = el('span', 'sode__slot' + (set.has(l.ui) ? ' sode__slot--on' : ''), l.label);
      slot.dataset.hint = `Level ${l.label}`; // where the slot is too small for its label
      return levelColor(slot, l);
    }),
  );
  // Two equal rows; an odd first level (0 or V) stands on its own to the left.
  const odd = S.levels.length % 2;
  if (odd) row.classList.add('sode__bar--odd');
  row.style.setProperty('--sode-bar-cols', Math.floor(S.levels.length / 2) + odd);
  const sorted = [...set].sort((a, b) => a - b);
  row.setAttribute('role', 'img');
  row.setAttribute('aria-label', sorted.length ? `Levels ${levelText(sorted)}` : 'No levels');
  return row;
}

export const pickButtons = (onToggle) => levelPicks(S.levels, onToggle);
