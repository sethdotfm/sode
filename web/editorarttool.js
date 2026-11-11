// The Artwork item in the panel editor's smart menu, below Find: one Icon A for every selected cell, Icon B one
// glyph per cell in Find's fill order, or Clear artwork. The plan is arttoolrules.js; this file is the DOM.
// initArtTool(editor, find) takes what runEditor and initFind return.
import { el, icon, $ } from './core.js';
import { btn } from './formbits.js';
import { artPane, uploadArt } from './editorart.js';
import { artFilter, ART_ICONS } from './sidebarrules.js';
import { ART_B_CHOICES, textHidden, withDefault } from './panellook.js';
import { artNode } from './panelview.js';
import { prepareArt } from './artcompress.js';
import { cellAt } from './gridmodel.js';
import { createOrderPicker } from './editororder.js';
import { flyoutShift } from './editorrules.js';
import { artToolPlan, applyArtTool } from './arttoolrules.js';

const EDGE_GAP = 8; // keep in step with GAP in editormenu.js and EDGE_GAP in editorfind.js
const UNSAVED = 'Save the panel before uploading an image.';

const plural = (n, one, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

// What a plan does, said before (preview) or after (done) it runs.
function describe(plan, choice, done) {
  if (!plan) return '';
  const n = plan.slots.length;
  const cells = plural(n, 'cell');
  let text;
  if (!n) text = 'No cell in the selection can take it.';
  else if (choice.clear) text = `${done ? 'Cleared' : 'Clears'} the artwork of ${cells}.`;
  else if ('b' in choice) {
    const first = plan.slots[0].patch.artB;
    const last = plan.slots[n - 1].patch.artB;
    text = `${done ? 'Set' : 'Sets'} Icon B ${n > 1 ? `${first} to ${last}` : first} on ${cells}.`;
  } else text = `${done ? 'Set' : 'Sets'} Icon A on ${cells}.`;
  if (plan.left) text += ` ${plural(plan.left, 'cell')} left without Icon B.`;
  if (plan.empty) text += ` ${plural(plan.empty, 'empty square')} skipped.`;
  return text;
}

export function initArtTool(editor, find) {
  let preview = [];
  let shown = null; // the choice the preview or the last change is for
  let quiet = false; // a change just landed, so no preview of it again until the choice changes
  let hovered = false;
  let rebuilding = false;

  const note = el('p', 'sode-find__note');
  note.setAttribute('role', 'status');

  const keyOf = (choice) => (choice.clear ? 'clear' : 'b' in choice ? `b:${choice.b}` : `a:${choice.a}`);
  const a = artPane(
    'Icon A',
    ART_ICONS,
    (n) => icon(n),
    (v) => apply({ a: `icon:${v}` }),
  );
  const b = artPane(
    'Icon B',
    ART_B_CHOICES,
    (g) => el('span', 'sode-side__glyph', g),
    (v) => apply({ b: v }),
  );
  for (const x of a.grid.querySelectorAll('button')) x.dataset.choice = `a:icon:${x.getAttribute('aria-label')}`;
  for (const x of b.grid.querySelectorAll('button')) x.dataset.choice = `b:${x.getAttribute('aria-label')}`;

  const search = el('input', 'pb-input sode-side__find');
  search.type = 'search';
  search.placeholder = 'Search icons';
  search.autocomplete = 'off';
  search.setAttribute('aria-label', 'Search icons by name');
  search.addEventListener('input', () => {
    const keep = new Set(artFilter(ART_ICONS, search.value));
    for (const x of a.grid.querySelectorAll('button')) x.hidden = !keep.has(x.getAttribute('aria-label'));
    fit();
  });

  const file = el('input');
  file.type = 'file';
  file.accept = 'image/jpeg,image/png,image/gif,image/svg+xml';
  file.hidden = true;
  const up = btn('pb-btn--outline pb-btn--sm', 'Upload image', () => file.click());
  const clear = btn('pb-btn--outline pb-btn--sm', 'Clear artwork', () => apply({ clear: true }));
  clear.dataset.choice = 'clear';

  const column = (head, p, ...extra) => el('div', 'sode-side__col', el('div', 'sode-side__col-top', el('span', 'sode-side__col-h', head), ...extra), p.grid);
  const picker = el('div', 'sode-side__picker', column('Icon A', a, search), column('Icon B', b));
  const orders = createOrderPicker(find.order(), (id) => find.setOrder(id, { focus: false }));
  find.onOrder((id) => {
    orders.set(id);
    showPreview();
  });
  const tools = el('div', 'sode-side__row', up, clear);
  const panel = el('div', 'pb-menu pb-menu__sub sode-find sode-arttool', picker, tools, orders, note, file);

  const choiceOf = (node) => {
    const key = node?.closest?.('[data-choice]')?.dataset.choice;
    if (!key) return null;
    if (key === 'clear') return { clear: true };
    return key.startsWith('b:') ? { b: key.slice(2) } : { a: key.slice(2) };
  };
  const active = () => panel.dataset.open === '1' || hovered;
  const planFor = (choice) => (choice ? artToolPlan(editor.grid(), choice, find.order()) : null);

  // Measured where CSS put it, then pulled back inside the viewport, as Find's flyout is.
  function fit() {
    if (!panel.isConnected) return;
    panel.style.translate = '';
    const rect = panel.getBoundingClientRect();
    if (!rect.width) return;
    const { x, y } = flyoutShift(rect, { w: document.documentElement.clientWidth, h: document.documentElement.clientHeight }, EDGE_GAP);
    if (x || y) panel.style.translate = `${x}px ${y}px`;
  }

  // ---- preview in the grid: each square drawn as it would be, Default artwork and the Icon B badge included

  function clearPreview() {
    for (const p of preview) {
      p.node.className = p.cls;
      p.node.replaceChildren(...p.kids);
      if (p.hide === undefined) delete p.node.dataset.hideText;
      else p.node.dataset.hideText = p.hide;
    }
    preview = [];
  }
  function showPreview() {
    clearPreview();
    const plan = active() && shown && !quiet ? planFor(shown) : null;
    if (!quiet) note.textContent = describe(plan, shown, false);
    if (!plan) return;
    const g = editor.grid();
    for (const s of plan.slots) {
      const node = $('ed-grid')?.querySelector(`.sode-ed__sq[data-r="${s.r}"][data-c="${s.c}"]`);
      if (!node) continue;
      preview.push({ node, cls: node.className, kids: [...node.childNodes], hide: node.dataset.hideText });
      const cell = { ...cellAt(g, s.r, s.c), ...s.patch };
      for (const k of Object.keys(s.patch)) if (cell[k] === undefined) delete cell[k];
      const drawn = withDefault(cell, editor.defaults());
      node.classList.add('is-preview', 'is-replaced');
      node.querySelector(':scope > .sode-panel__art')?.remove();
      const row = artNode(drawn, editor.art());
      if (row) node.prepend(row);
      if (textHidden(drawn)) node.dataset.hideText = '1';
      else delete node.dataset.hideText;
    }
  }
  const hover = (choice) => {
    if (choice && shown && keyOf(choice) === keyOf(shown)) return;
    shown = choice;
    quiet = false;
    showPreview();
  };

  function apply(choice) {
    const plan = planFor(choice);
    if (!plan) return;
    clearPreview();
    shown = choice;
    quiet = true;
    note.textContent = describe(plan, choice, true);
    editor.status(note.textContent);
    if (plan.slots.length) editor.setGrid(applyArtTool(editor.grid(), plan), { what: 'artwork' });
  }

  file.addEventListener('change', async () => {
    const f = file.files[0];
    file.value = '';
    if (!f || !editor.panel()) return;
    note.textContent = 'Uploading.';
    try {
      const { blob, problem } = await prepareArt(f);
      if (problem) return void (note.textContent = problem);
      // One upload, one id on every cell: the panel stores the image once.
      const id = await uploadArt(editor.panel().id, f, blob);
      if (id) apply({ a: id });
    } catch (e) {
      note.textContent = e.message;
    }
  });

  panel.addEventListener('pointerover', (e) => {
    const c = choiceOf(e.target);
    if (c) hover(c);
  });
  panel.addEventListener('focusin', (e) => {
    panel.dataset.open = '1';
    up.disabled = !editor.panel();
    if (!editor.panel() && !note.textContent) note.textContent = UNSAVED;
    const c = choiceOf(e.target);
    if (c) hover(c);
    else showPreview();
    fit();
  });

  // Arrows walk a grid of choices (up and down by its row); Down from the search box enters the icons.
  panel.addEventListener('keydown', (e) => {
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) return;
    const grid = e.target === search ? a.grid : e.target.closest?.('.sode-side__icons');
    if (!grid) return;
    const list = [...grid.querySelectorAll('button')].filter((x) => !x.hidden);
    if (!list.length) return;
    e.preventDefault();
    e.stopPropagation();
    if (e.target === search) {
      if (e.key === 'ArrowDown') list[0].focus();
      return;
    }
    const i = list.indexOf(e.target);
    const across = list.filter((x) => x.offsetTop === list[0].offsetTop).length || 1;
    const step = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -across, ArrowDown: across }[e.key];
    const j = i + step;
    if (j < 0 && grid === a.grid && e.key === 'ArrowUp') search.focus();
    else list[Math.max(0, Math.min(list.length - 1, j))].focus();
  });

  // ---- open and close

  const close = () => {
    delete panel.dataset.open;
    hovered = false;
    shown = null;
    quiet = false;
    clearPreview();
    note.textContent = '';
  };
  panel.addEventListener('pointerenter', () => {
    hovered = true;
    up.disabled = !editor.panel();
    fit();
  });
  panel.addEventListener('pointerleave', () => {
    hovered = false;
    if (!panel.contains(document.activeElement)) close();
    else {
      shown = choiceOf(document.activeElement);
      showPreview();
    }
  });
  panel.addEventListener('focusout', (e) => {
    if (rebuilding || (e.relatedTarget && panel.contains(e.relatedTarget))) return;
    if (!hovered) close();
  });

  function open() {
    panel.dataset.open = '1';
    up.disabled = !editor.panel();
    search.focus();
    fit();
  }

  // The menu rebuilds after every grid change and detaches this panel: put focus back on the same choice.
  editor.addMenuSection(() => {
    const had = panel.contains(document.activeElement) ? document.activeElement : null;
    rebuilding = true;
    preview = []; // the grid has repainted
    queueMicrotask(() => {
      rebuilding = false;
      if (had && !panel.contains(document.activeElement)) had.focus({ preventScroll: true });
      if (!had && !hovered && panel.dataset.open === '1') close();
      showPreview();
      fit();
    });
    return [{ label: 'Artwork', icon: 'image', panel, run: open }];
  });
  editor.refresh();
}
