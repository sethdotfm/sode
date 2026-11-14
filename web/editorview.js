// Draws the editor's grid from the grid model and turns pointer and arrow keys into select calls. Holds no
// grid state: paint is called after every change. Squares that a merged cell covers are not drawn.
import { el, icon } from './core.js';
import { cellAt, rectOf, isUnassigned } from './gridmodel.js';
import { kindLabel } from './editorrules.js';
import { artNode } from './panelview.js';
import { cellScale, textHidden, withDefault } from './panellook.js';
import { goToState, navText } from './panelnav.js';
import { PAGE_KINDS, pageCellText } from './pagecells.js';

// keep in step with findrules.js CATEGORY_ICON
const KIND_ICON = {
  source: 'arrow-right-from-line',
  destination: 'arrow-right-to-line',
  take: 'zap',
  preset: 'bookmark',
  clear: 'eraser',
  undo: 'undo-2',
  current: 'eye',
  panelName: 'panels-top-left',
  instanceName: 'tablet-smartphone',
  pageName: 'file-type',
  info: 'info',
  goToPage: 'file-symlink',
  nextPage: 'chevron-right',
  previousPage: 'chevron-left',
};

// A cell's text in the editor: the caption, else the router name, else what is still to be chosen. pages is the
// panel's Page list, for a Go to Page cell's target; page is the id of the Page the cell sits on.
export function cellText(cell, names, pages = [], page = '') {
  if (cell.caption) return cell.caption;
  if (PAGE_KINDS.includes(cell.kind)) return pageCellText(cell, { pages, page }).value || kindLabel(cell.kind);
  if (cell.kind === 'goToPage') {
    const s = goToState(pages, cell);
    return s === 'unassigned' ? 'Choose a Page' : s === 'missing' ? 'Page deleted' : navText(cell, pages);
  }
  if (cell.kind === 'nextPage' || cell.kind === 'previousPage') return kindLabel(cell.kind);
  if (['take', 'clear', 'undo', 'panelName', 'instanceName'].includes(cell.kind)) return kindLabel(cell.kind);
  if (cell.kind === 'current') return 'Currently';
  if (cell.kind === 'preset') return cell.name || 'Choose a preset';
  if (!Number.isInteger(cell.n)) return `Choose a ${kindLabel(cell.kind).toLowerCase()}`;
  return names.of(cell);
}

// host is the grid element. on.select(r, c, extend) and on.key(event) go to the page. Pointer handling is
// delegated, so a repaint during a drag does not end it.
export function createEditorGrid(host, on) {
  host.tabIndex = 0;
  host.setAttribute('role', 'grid');
  let dragging = false;
  let last = '';

  const squareAt = (x, y) => {
    const n = document.elementFromPoint(x, y)?.closest('[data-r]');
    return n && host.contains(n) ? { r: Number(n.dataset.r), c: Number(n.dataset.c) } : null;
  };
  host.addEventListener('pointerdown', (e) => {
    if (e.button !== 0) return;
    const q = squareAt(e.clientX, e.clientY);
    if (!q) return;
    dragging = true;
    document.body.dataset.edDrag = '1'; // the menu sits over neighbouring squares and would swallow the drag
    last = `${q.r},${q.c}`;
    on.select(q.r, q.c, e.shiftKey);
  });
  host.addEventListener('pointermove', (e) => {
    if (!dragging) return;
    const q = squareAt(e.clientX, e.clientY);
    if (!q || `${q.r},${q.c}` === last) return;
    last = `${q.r},${q.c}`;
    on.select(q.r, q.c, true);
  });
  const stop = () => {
    dragging = false;
    delete document.body.dataset.edDrag;
  };
  document.addEventListener('pointerup', stop);
  document.addEventListener('pointercancel', stop);
  host.addEventListener('keydown', (e) => on.key(e));

  return {
    // art is createArt's answer for the panel, for cells with uploaded artwork; defaults is Default artwork by item.
    // pages is the Page list: a Go to Page cell whose Page was deleted is dashed like an unassigned one. page is
    // the id of the Page drawn, for Page Name and Info cells.
    paint(g, names, { readOnly = false, art: artSource = null, defaults = null, pages = [], page = '' } = {}) {
      host.replaceChildren();
      host.style.gridTemplate = `repeat(${g.rows}, 1fr) / repeat(${g.cols}, 1fr)`;
      host.style.aspectRatio = `${g.cols} / ${g.rows}`;
      host.dataset.readonly = readOnly ? '1' : '';
      const q = rectOf(g);
      const inSel = (r, c, rs = 1, cs = 1) => q && !(r + rs - 1 < q.r1 || r > q.r2 || c + cs - 1 < q.c1 || c > q.c2);
      for (let r = 1; r <= g.rows; r++) {
        for (let c = 1; c <= g.cols; c++) {
          const cell = cellAt(g, r, c);
          if (cell && (cell.r !== r || cell.c !== c)) continue;
          const rs = cell?.rs || 1;
          const cs = cell?.cs || 1;
          const b = el('div', 'sode-ed__sq');
          b.dataset.r = String(r);
          b.dataset.c = String(c);
          b.style.gridArea = `${r} / ${c} / span ${rs} / span ${cs}`;
          b.setAttribute('role', 'gridcell');
          if (inSel(r, c, rs, cs)) b.classList.add('is-sel');
          if (cell) {
            b.classList.add('has-cell', `sode-ed__sq--${cell.kind}`);
            if (isUnassigned(cell) || (cell.kind === 'goToPage' && goToState(pages, cell) === 'missing')) b.classList.add('is-unassigned');
            if (rs * cs > 1) b.classList.add('is-merged');
            const drawn = withDefault(cell, defaults);
            const art = artNode(drawn, artSource);
            if (art) b.append(art);
            if (cell.fontSize) b.style.setProperty('--cell-fs', cellScale(cell));
            if (textHidden(drawn)) b.dataset.hideText = '1'; // the square hides its words as the panel page will (panels.css)
            b.append(icon(KIND_ICON[cell.kind] || 'square-dashed'), el('span', 'sode-ed__text', cellText(cell, names, pages, page)));
          }
          host.append(b);
        }
      }
    },
    // The selection's box on screen, for the smart menu to sit beside.
    selectionBox() {
      const sel = [...host.querySelectorAll('.is-sel')];
      if (!sel.length) return null;
      const rects = sel.map((n) => n.getBoundingClientRect());
      return {
        left: Math.min(...rects.map((x) => x.left)),
        top: Math.min(...rects.map((x) => x.top)),
        right: Math.max(...rects.map((x) => x.right)),
        bottom: Math.max(...rects.map((x) => x.bottom)),
      };
    },
  };
}
