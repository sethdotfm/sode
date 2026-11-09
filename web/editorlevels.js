// The level picker of the Panel and Cell tabs: the Sode main page's own look (base.css .sode__pick, desktop.css
// .sode__picker) built by levelview.js pickButtons, so labels, colors and numbering follow Settings. No stored
// subset lights every level, so the state shows instead of being explained, and the last lit level stays on
// (sidebarrules.js levelLocked).
import { el, S } from './core.js';
import { pickButtons } from './levelview.js';
import { levelsToggle, levelsShown, levelLocked, levelsPatch } from './sidebarrules.js';

// opts: {ed, onPatch(levels | undefined)}. draw(stored) repaints only when what is shown changed.
export function createLevelBoxes({ ed, onPatch }) {
  const grid = el('div', 'sode__levels');
  grid.setAttribute('role', 'group');
  grid.setAttribute('aria-label', 'Pick levels');
  const allBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__all', el('span', null, 'Select'), el('span', null, 'All'));
  allBtn.type = 'button';
  const node = el('div', 'sode__picker', grid, allBtn);
  let shown = '';
  let stored;
  const nums = () => S.levels.map((l) => l.ui);
  allBtn.addEventListener('click', () => onPatch(levelsPatch(nums(), nums())));
  return {
    node,
    reset: () => (shown = ''),
    draw(now) {
      stored = now;
      const all = nums();
      const on = levelsShown(all, stored);
      const key = `${all.join(',')}|${S.levels.map((l) => l.label).join(',')}|${on.join(',')}|${ed.readOnly()}`;
      if (key === shown) return;
      shown = key;
      const odd = all.length % 2;
      grid.classList.toggle('sode__levels--odd', !!odd); // a first level of 0 or V stands alone on the left, as on the main page
      grid.style.setProperty('--sode-bar-cols', Math.floor(all.length / 2) + odd);
      const picks = pickButtons((ui) => onPatch(levelsToggle(all, stored, ui, !on.includes(ui))));
      for (const b of picks) {
        b.tabIndex = 0; // pickButtons leaves them out of the tab order for the main page's keyboard mode
        const ui = +b.dataset.ui;
        b.setAttribute('aria-pressed', String(on.includes(ui)));
        b.disabled = ed.readOnly() || levelLocked(all, stored, ui);
      }
      grid.replaceChildren(...picks);
      allBtn.disabled = ed.readOnly() || on.length === all.length;
    },
  };
}
