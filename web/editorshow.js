// The Cell tab's Show checkboxes for a Page Name or Info cell. Rules are pagecells.js.
import { el } from './core.js';
import { SHOW_PARTS, PART_WORDS, showOf } from './pagecells.js';

// patch(fields) changes the selected cell. Every box is stored as the full list, so a cell never falls back to a default
// once it was set.
export function createShowBoxes({ ed, patch }) {
  const list = el('div', 'sode-side__checks');
  const node = el('fieldset', 'sode__field', el('legend', 'sode__field-label', 'Show'), list);
  node.id = 'ed-cell-show';
  let kind = '';
  const boxes = new Map();

  function draw(k) {
    kind = k;
    boxes.clear();
    list.replaceChildren(
      ...SHOW_PARTS[k].map((part) => {
        const box = el('input', 'pb-check__input');
        box.type = 'checkbox';
        box.dataset.part = part;
        box.addEventListener('change', () => patch({ show: SHOW_PARTS[kind].filter((p) => boxes.get(p).checked) }));
        boxes.set(part, box);
        return el('label', 'pb-check', box, el('span', 'pb-check__label', PART_WORDS[part]));
      }),
    );
  }

  return {
    node,
    update(c) {
      node.hidden = !SHOW_PARTS[c.kind];
      if (node.hidden) return;
      if (kind !== c.kind) draw(c.kind);
      const on = showOf(c);
      for (const [part, box] of boxes) {
        box.checked = on.includes(part);
        box.disabled = ed.readOnly();
      }
    },
  };
}
