// The level boxes of the Panel and Cell tabs: one compact row of numbered boxes. No stored subset ticks every
// box, so the state shows instead of being explained, and the last ticked box stays on (sidebarrules.js levelLocked).
import { el } from './core.js';
import { check } from './formbits.js';
import { levelsToggle, levelsShown, levelLocked } from './sidebarrules.js';

// opts: {ed, onPatch(levels | undefined)}. draw(list, stored) repaints only when what is shown changed.
export function createLevelBoxes({ ed, onPatch }) {
  const box = el('div', 'sode-side__levels');
  let shown = '';
  return {
    node: box,
    reset: () => (shown = ''),
    draw(list, stored) {
      const all = list.map((l) => l.level);
      const on = levelsShown(all, stored);
      const key = `${all.join(',')}|${on.join(',')}|${ed.readOnly()}`;
      if (key === shown) return;
      shown = key;
      box.replaceChildren(
        ...list.map((l) => {
          const c = check(String(l.label || l.level), { checked: on.includes(l.level), onChange: (v) => onPatch(levelsToggle(all, stored, l.level, v)) });
          c.input.disabled = ed.readOnly() || levelLocked(all, stored, l.level);
          c.label.dataset.pbTip = `Level ${l.level}`;
          return c.label;
        }),
      );
    },
  };
}
