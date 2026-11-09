// The sidebar's Cell tab: caption, levels, font size and artwork of the one cell selected. Fields are built once
// and updated in place, so typing a caption keeps the caret while the grid repaints.
import { el, S } from './core.js';
import { field } from './formbits.js';
import { single, setCell, selCells } from './gridmodel.js';
import { kindLabel } from './editorrules.js';
import { fontPercent } from './sidebarrules.js';
import { createLevelBoxes } from './editorlevels.js';
import { createArtControl } from './editorart.js';

const CAPTION_MAX = 40;
const LEVEL_KINDS = ['source', 'destination'];

// Keep what the user is typing: a focused field is only overwritten when another cell is shown.
const put = (input, value, force) => {
  if ((force || input !== document.activeElement) && input.value !== value) input.value = value;
};

export function createCellTab(ed) {
  const node = el('div', 'sode-side__tab');
  const hint = el('p', 'sode__hint');
  const form = el('div', 'sode-side__form');
  let at = '';

  const kindLine = el('p', 'sode-side__kind');
  const caption = el('input', 'pb-input');
  caption.id = 'ed-cell-caption';
  caption.maxLength = CAPTION_MAX;
  caption.autocomplete = 'off';
  const captionField = field('ed-cell-caption', 'Caption', caption, 'Replaces the Router name on this cell.');
  const levelBoxes = createLevelBoxes({ ed, onPatch: (v) => patch({ levels: v }) });
  const levelField = el('div', 'sode__field', el('span', 'sode__field-label', 'Levels'), levelBoxes.node);
  const size = el('input', 'pb-input sode-ed__num');
  size.id = 'ed-cell-size';
  size.type = 'number';
  size.min = '50';
  size.max = '200';
  size.inputMode = 'numeric';
  size.placeholder = '100';
  size.title = '50 to 200 percent of the panel font size.';
  const sizeNote = el('span', 'sode__hint');
  sizeNote.setAttribute('role', 'status');
  const sizeField = el(
    'div',
    'sode__field sode-side__inline',
    Object.assign(el('label', null, 'Font size'), { htmlFor: 'ed-cell-size' }),
    size,
    el('span', 'sode__hint', '%'),
  );
  const sizeRow = el('div', 'sode-side__sizerow', sizeField, sizeNote);
  const art = createArtControl({ ed, icons: true, set: (v) => patch({ art: v }), setHideText: (v) => patch({ hideText: v ? true : undefined }) });
  form.append(kindLine, captionField, levelField, sizeRow, art.node);
  node.append(hint, form);

  const cell = () => single(ed.grid());
  function patch(p) {
    const c = cell();
    if (c) ed.setGrid(setCell(ed.grid(), { r: c.r, c: c.c }, p));
  }

  caption.addEventListener('input', () => patch({ caption: caption.value.trim() ? caption.value : undefined }));
  size.addEventListener('input', () => {
    const t = size.value.trim();
    if (t === '') return void ((sizeNote.textContent = ''), patch({ fontSize: undefined }));
    const r = fontPercent(t);
    sizeNote.textContent = r.error || '';
    if (!r.error) patch({ fontSize: r.value === 100 ? undefined : r.value });
  });

  return {
    node,
    resetLevels() {
      at = '';
      levelBoxes.reset();
    },
    update() {
      const g = ed.grid();
      const c = cell();
      const n = selCells(g).length;
      hint.hidden = !!c;
      form.hidden = !c;
      if (!c) {
        hint.textContent =
          n > 1
            ? 'Several cells are selected. Select one to change its settings.'
            : n === 1
              ? 'Select the whole cell to change its settings.'
              : 'Select a square to change its settings. An empty square gets its type from the menu beside it.';
        return;
      }
      const key = `${c.r},${c.c}`;
      const fresh = key !== at;
      if (fresh) levelBoxes.reset();
      kindLine.textContent = `${kindLabel(c.kind)} at row ${c.r}, column ${c.c}`;
      put(caption, c.caption || '', fresh);
      put(size, String(c.fontSize || 100), fresh); // shown, so the arrows step from the default
      levelField.hidden = !LEVEL_KINDS.includes(c.kind) || !S.levels.length;
      levelBoxes.draw(c.levels);
      at = key;
      for (const i of [caption, size]) i.disabled = ed.readOnly();
      art.update(c.art || '', c.caption || '', c.hideText === true);
    },
  };
}
