// The grip at the selection's bottom right corner: drag it to continue the selection's numbering. The rules are
// gridmodel.js (dragRect, extendPlan, applyExtend) and editorrules.js (gripTip); this file is the pointer and the
// preview, which marks real grid squares in place like Find's and is wiped by the next repaint.
import { el, icon, $ } from './core.js';
import { dragRect, extendPlan, applyExtend } from './gridmodel.js';
import { gripTip } from './editorrules.js';

const KIND_ICON = { source: 'arrow-right-from-line', destination: 'arrow-right-to-line' };

export function initGrip(editor) {
  const host = $('ed-grid');
  const grip = el('div', 'sode-ed__grip');
  grip.setAttribute('aria-hidden', 'true');
  const tip = el('div', 'sode-ed__tip');
  tip.setAttribute('role', 'status');
  let preview = [];
  let dragging = false;

  function clearPreview() {
    for (const p of preview) {
      p.node.className = p.cls;
      p.node.replaceChildren(...p.kids);
    }
    preview = [];
    tip.hidden = true;
  }

  // The square under a pointer, from the grid's own geometry, so a merged cell does not skew the count.
  function squareAt(x, y) {
    const g = editor.grid();
    const box = host.getBoundingClientRect();
    const st = getComputedStyle(host);
    const pad = parseFloat(st.paddingLeft) || 0;
    const gap = parseFloat(st.columnGap) || 0;
    const cw = (box.width - 2 * pad - (g.cols - 1) * gap) / g.cols + gap;
    const ch = (box.height - 2 * pad - (g.rows - 1) * gap) / g.rows + gap;
    const clamp = (v, max) => Math.max(1, Math.min(max, v));
    return { r: clamp(Math.floor((y - box.top - pad) / ch) + 1, g.rows), c: clamp(Math.floor((x - box.left - pad) / cw) + 1, g.cols) };
  }

  function planAt(x, y) {
    const g = editor.grid();
    const { r, c } = squareAt(x, y);
    const to = dragRect(g, r, c);
    return to ? extendPlan(g, to) : null;
  }

  function show(plan, x, y) {
    clearPreview();
    if (!plan) return;
    const names = editor.names();
    for (const s of plan.slots) {
      const node = host.querySelector(`.sode-ed__sq[data-r="${s.r}"][data-c="${s.c}"]`);
      if (!node) continue;
      preview.push({ node, cls: node.className, kids: [...node.childNodes] });
      node.classList.add('is-preview');
      node.replaceChildren(icon(KIND_ICON[s.kind] || 'square-dashed'), el('span', 'sode-ed__text', names.of({ kind: s.kind, n: s.n })));
    }
    tip.textContent = gripTip(plan, names.of);
    tip.hidden = false;
    const box = host.getBoundingClientRect();
    tip.style.left = `${x - box.left + 16}px`;
    tip.style.top = `${y - box.top + 16}px`;
  }

  grip.addEventListener('pointerdown', (e) => {
    if (e.button !== 0 || editor.readOnly()) return;
    e.preventDefault();
    e.stopPropagation();
    dragging = true;
    document.body.dataset.edDrag = '1'; // the menu would swallow the drag
  });
  // On the document, so the drag follows the pointer off the grip and survives the grip being repainted.
  document.addEventListener('pointermove', (e) => {
    if (dragging) show(planAt(e.clientX, e.clientY), e.clientX, e.clientY);
  });
  const finish = (e, apply) => {
    if (!dragging) return;
    dragging = false;
    delete document.body.dataset.edDrag;
    const plan = apply ? planAt(e.clientX, e.clientY) : null;
    clearPreview();
    if (plan?.slots.length) editor.setGrid(applyExtend(editor.grid(), plan));
  };
  document.addEventListener('pointerup', (e) => finish(e, true));
  document.addEventListener('pointercancel', (e) => finish(e, false));

  // Every repaint drops the grip and the preview with the old squares, so both are put back here.
  editor.onPaint((g, box) => {
    preview = [];
    if (!box || editor.readOnly()) return;
    const h = host.getBoundingClientRect();
    grip.style.left = `${box.right - h.left}px`;
    grip.style.top = `${box.bottom - h.top}px`;
    host.append(grip, tip);
    tip.hidden = true;
  });
  editor.refresh();
}
