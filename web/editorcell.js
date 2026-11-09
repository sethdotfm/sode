// The sidebar's Cell tab: caption, levels, font size and artwork of the one cell selected. Fields are built once
// and updated in place, so typing a caption keeps the caret while the grid repaints.
import { el } from './core.js';
import { field, check } from './formbits.js';
import { single, setCell, selCells } from './gridmodel.js';
import { kindLabel } from './editorrules.js';
import { fontPercent, levelsPatch } from './sidebarrules.js';
import { createArtControl } from './editorart.js';

const CAPTION_MAX = 40;
const LEVEL_KINDS = ['source', 'destination'];

// Keep what the user is typing: a focused field is only overwritten when another cell is shown.
const put = (input, value, force) => {
  if ((force || input !== document.activeElement) && input.value !== value) input.value = value;
};

export function createCellTab(ed, levelList) {
  const node = el('div', 'sode-side__tab');
  const hint = el('p', 'sode__hint');
  const form = el('div', 'sode-side__form');
  let at = '';
  let levels = [];

  const kindLine = el('p', 'sode-side__kind');
  const caption = el('input', 'pb-input');
  caption.id = 'ed-cell-caption';
  caption.maxLength = CAPTION_MAX;
  caption.autocomplete = 'off';
  const captionField = field('ed-cell-caption', 'Caption', caption, 'Replaces the Router name on this cell.');
  const levelBox = el('div', 'sode-side__checks');
  const levelField = el('div', 'sode__field', el('span', 'sode__field-label', 'Levels'), levelBox, el('span', 'sode__hint', 'None ticked means every level the panel allows.'));
  const size = el('input', 'pb-input sode-ed__num');
  size.id = 'ed-cell-size';
  size.type = 'number';
  size.min = '50';
  size.max = '200';
  size.inputMode = 'numeric';
  const sizeNote = el('span', 'sode__hint', '50 to 200 percent of the panel font size.');
  sizeNote.setAttribute('role', 'status');
  const sizeField = field('ed-cell-size', 'Font size, percent', size, '');
  sizeField.append(sizeNote);
  const art = createArtControl({ ed, icons: true, set: (v) => patch({ art: v }) });
  form.append(kindLine, captionField, levelField, sizeField, art.node);
  node.append(hint, form);

  const cell = () => single(ed.grid());
  function patch(p) {
    const c = cell();
    if (c) ed.setGrid(setCell(ed.grid(), { r: c.r, c: c.c }, p));
  }

  caption.addEventListener('input', () => patch({ caption: caption.value.trim() ? caption.value : undefined }));
  size.addEventListener('input', () => {
    const t = size.value.trim();
    if (t === '') return void ((sizeNote.textContent = '50 to 200 percent of the panel font size.'), patch({ fontSize: undefined }));
    const r = fontPercent(t);
    sizeNote.textContent = r.error || '50 to 200 percent of the panel font size.';
    if (!r.error) patch({ fontSize: r.value === 100 ? undefined : r.value });
  });

  function drawLevels(chosen) {
    const all = levels.map((l) => l.level);
    levelBox.replaceChildren(
      ...levels.map((l) => {
        const c = check(l.label || `Level ${l.level}`, {
          checked: chosen.includes(l.level),
          onChange: (on) => {
            const now = on ? [...chosen, l.level] : chosen.filter((x) => x !== l.level);
            patch({ levels: levelsPatch(all, now) });
          },
        });
        c.input.disabled = ed.readOnly();
        return c.label;
      }),
    );
  }

  return {
    node,
    setLevels(list) {
      levels = list;
      at = ''; // redraw the ticks for the cell on show
    },
    update() {
      const g = ed.grid();
      const c = cell();
      const n = selCells(g).length;
      hint.hidden = !!c;
      form.hidden = !c;
      if (!c) {
        hint.textContent = n > 1 ? 'Several cells are selected. Select one to change its settings.' : n === 1 ? 'Select the whole cell to change its settings.' : 'Select a square to change its settings. An empty square gets its type from the menu beside it.';
        return;
      }
      const key = `${c.r},${c.c}`;
      const fresh = key !== at;
      const chosen = Array.isArray(c.levels) ? c.levels : [];
      kindLine.textContent = `${kindLabel(c.kind)} at row ${c.r}, column ${c.c}`;
      put(caption, c.caption || '', fresh);
      put(size, c.fontSize ? String(c.fontSize) : '', fresh);
      levelField.hidden = !LEVEL_KINDS.includes(c.kind) || !levels.length;
      if (fresh || levelBox.dataset.shown !== chosen.join(',')) {
        drawLevels(chosen);
        levelBox.dataset.shown = chosen.join(',');
      }
      at = key;
      for (const i of [caption, size]) i.disabled = ed.readOnly();
      art.update(c.art || '', c.caption || '');
    },
  };
}
