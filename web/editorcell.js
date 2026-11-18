// The sidebar's Cell tab: the selected cells' settings, or on a Grid panel the selected rows' artwork. Fields are built
// one time and updated in place, so typing a caption keeps the caret.
import { el, S } from './core.js';
import { field } from './formbits.js';
import { single, setCells, selCells } from './gridmodel.js';
import { kindLabel } from './editorrules.js';
import { fontPercent } from './sidebarrules.js';
import { createLevelBoxes } from './editorlevels.js';
import { createArtControl } from './editorart.js';
import { createDefaultRow } from './editordefault.js';
import { createPageTarget } from './editorpagetarget.js';
import { createShowBoxes } from './editorshow.js';
import { defaultFor, withDefault } from './panellook.js';
import { isGridPanel } from './gridpaneledges.js';
import { artToolPlan, applyArtTool, leftNote } from './arttoolrules.js';
import { plural } from './gridclip.js';

const CAPTION_MAX = 40;
const LEVEL_KINDS = ['source', 'destination'];

// Keep what the user is typing: a focused field is only overwritten when another cell is shown.
const put = (input, value, force) => {
  if ((force || input !== document.activeElement) && input.value !== value) input.value = value;
};
// The value every target has, or '' when they differ.
const same = (xs, k) => (xs.every((x) => (x[k] ?? '') === (xs[0][k] ?? '')) ? (xs[0][k] ?? '') : '');

// picked() gives the Grid panel's list with selected rows ({ list, rows }), or null.
export function createCellTab(ed, { picked }) {
  const node = el('div', 'sode-side__tab');
  const hint = el('p', 'sode__hint');
  const form = el('div', 'sode-side__form');
  let at = '';

  const kindLine = el('p', 'sode-side__kind');
  kindLine.id = 'ed-cell-kind';
  const caption = el('input', 'pb-input');
  caption.id = 'ed-cell-caption';
  caption.maxLength = CAPTION_MAX;
  caption.autocomplete = 'off';
  const captionField = field('ed-cell-caption', 'Text override', caption);
  const levelBoxes = createLevelBoxes({ ed, onPatch: (v) => patch({ levels: v }, null, LEVEL_KINDS) });
  const levelNote = el('span', 'sode__hint');
  const levelField = el('div', 'sode__field', el('span', 'sode__field-label', 'Levels'), levelBoxes.node, levelNote);
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
  const artNote = el('p', 'sode__hint');
  artNote.id = 'ed-cell-art-note';
  artNote.setAttribute('role', 'status');
  const art = createArtControl({
    ed,
    icons: true,
    set: (v) => artChoice({ a: v }),
    setB: (v) => artChoice({ b: v }),
    // false is stored only against a default, which would otherwise hide the text (panellook.js withDefault).
    setHideText: (v) =>
      isGridPanel(ed.fields()) ? picked()?.list.hideText(v) : patch((x) => ({ hideText: v ? true : defaultFor(x, ed.defaults()) ? false : undefined })),
  });
  const deflt = createDefaultRow({ ed, patch, done: () => update() });
  const target = createPageTarget({ ed, patch });
  const show = createShowBoxes({ ed, patch });
  const cellOnly = [captionField, sizeRow, levelField, target.node, show.node];
  form.append(kindLine, target.node, show.node, captionField, sizeRow, levelField, art.node, artNote, deflt.node);
  node.append(hint, form);

  // The cells the fields edit: the one cell, or every whole cell in the selection.
  const cells = () => {
    const c = single(ed.grid());
    return c ? [c] : selCells(ed.grid());
  };
  // p is a patch, or a function of the cell giving one. kinds limits it to cells of those kinds. join: typing in one
  // field is one undo step, not one per key.
  function patch(p, join, kinds) {
    const cs = cells().filter((x) => !kinds || kinds.includes(x.kind));
    if (!cs.length) return;
    const key = cs.map((x) => `${x.r} ${x.c}`).join(' ');
    ed.setGrid(
      setCells(ed.grid(), cs, (x) => (typeof p === 'function' ? p(x) : p)),
      { what: 'cell edit', join: join && `${join} ${key}` },
    );
  }

  // One artwork choice on the selection: a Grid panel's selected rows, or the selected cells.
  function artChoice(choice) {
    const g = picked();
    let left = 0;
    let unit = 'cell';
    if (isGridPanel(ed.fields())) {
      if (!g) return;
      left = g.list.art(choice);
      unit = 'row';
    } else {
      const plan = artToolPlan(ed.grid(), choice, ed.fillOrder());
      if (!plan) return;
      ed.setGrid(applyArtTool(ed.grid(), plan), { what: 'artwork' });
      left = plan.left;
    }
    artNote.textContent = leftNote(left, unit);
  }

  caption.addEventListener('input', () => patch({ caption: caption.value.trim() ? caption.value : undefined }, 'caption'));
  size.addEventListener('input', () => {
    const t = size.value.trim();
    if (t === '') return void ((sizeNote.textContent = ''), patch({ fontSize: undefined }, 'size'));
    const r = fontPercent(t);
    sizeNote.textContent = r.error || '';
    if (!r.error) patch({ fontSize: r.value === 100 ? undefined : r.value }, 'size');
  });

  return {
    node,
    resetLevels() {
      at = '';
      levelBoxes.reset();
    },
    openPicker: () => art.openPicker(),
    update,
  };

  function showForm(on) {
    hint.hidden = on;
    form.hidden = !on;
  }

  // A Grid panel: the artwork of the selected rows.
  function updateRows() {
    const g = picked();
    for (const n of [...cellOnly, deflt.node]) n.hidden = true;
    showForm(!!g);
    if (!g) {
      hint.textContent = 'Select sources or destinations in the lists to set their artwork.';
      return;
    }
    const off = ed.routerNames()?.numberOffset || 0;
    const { what } = g.list;
    const one = g.rows.length === 1 ? g.rows[0] : null;
    kindLine.textContent = one ? `${kindLabel(what)} ${one.n + off}, ${one.name || `${what} ${one.n + off}`}` : `${plural(g.rows.length, what)} selected`;
    const items = g.rows.map((r) => g.list.item(r.n));
    const key = `rows ${g.rows.map((r) => r.n).join()}`;
    if (key !== at) artNote.textContent = '';
    at = key;
    // As for cells: the controls edit what the rows set, and Hide text shows as drawn.
    const drawn = g.rows.map((r, i) => withDefault({ kind: what, n: r.n, ...items[i] }, ed.defaults()));
    art.update({
      value: same(items, 'art'),
      artB: same(items, 'artB'),
      caption: g.rows[0].name || '',
      hideText: drawn.every((x) => x.hideText === true),
      drawn: drawn.some((x) => x.art || x.artB),
    });
  }

  function update() {
    if (isGridPanel(ed.fields())) return updateRows();
    const cs = cells();
    const c = cs.length === 1 && single(ed.grid());
    const several = cs.length > 1;
    showForm(!!c || several);
    if (!c && !several) {
      hint.textContent =
        cs.length === 1
          ? 'Select the whole cell to change its settings.'
          : 'Select a square to change its settings. An empty square gets its type from the menu beside it.';
      return;
    }
    const first = cs[0];
    const key = cs.map((x) => `${x.r},${x.c}`).join(' ');
    const fresh = key !== at;
    if (fresh) {
      levelBoxes.reset();
      artNote.textContent = '';
    }
    const kinds = new Set(cs.map((x) => x.kind));
    const oneKind = kinds.size === 1;
    kindLine.textContent = several
      ? `${cs.length} ${oneKind ? `${kindLabel(first.kind)} ` : ''}cells selected. A change here sets them all.`
      : `${kindLabel(c.kind)} at row ${c.r}, column ${c.c}`;
    captionField.hidden = sizeRow.hidden = false;
    caption.placeholder = several && same(cs, 'caption') === '' && cs.some((x) => x.caption) ? 'Several values' : '';
    put(caption, same(cs, 'caption'), fresh);
    put(size, String(same(cs, 'fontSize') || 100), fresh); // shown, so the arrows step from the default
    const levelCells = cs.filter((x) => LEVEL_KINDS.includes(x.kind));
    levelField.hidden = !levelCells.length || !S.levels.length;
    const mixed = new Set(levelCells.map((x) => JSON.stringify(x.levels ?? null))).size > 1;
    levelNote.textContent = mixed ? 'The cells have different levels. A change sets these on all of them.' : '';
    levelBoxes.draw(levelCells[0]?.levels);
    // Page and Show belong to one kind of cell, so they show when every cell is that kind.
    target.node.hidden = show.node.hidden = !oneKind;
    if (oneKind) {
      target.update(first);
      show.update(first);
    }
    at = key;
    for (const i of [caption, size]) i.disabled = ed.readOnly();
    // The controls edit what the cells set; Hide text shows as drawn, so a default that hides the text reads as hidden.
    const drawn = cs.map((x) => withDefault(x, ed.defaults()));
    art.update({
      value: same(cs, 'art'),
      caption: several ? '' : first.caption || '',
      hideText: drawn.every((x) => x.hideText === true),
      artB: same(cs, 'artB'),
      drawn: drawn.some((x) => x.art || x.artB),
    });
    deflt.node.hidden = several;
    if (!several) deflt.update(c);
  }
}
