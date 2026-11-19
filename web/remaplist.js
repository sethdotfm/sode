// Keys and pointer for the remap step's one list of rows (importremap.js). The rows carry data-row; the model is
// remapselectrules.js. Focus sits on a row, not its select: on a closed select Space opens it and arrows change it.

import { addClick, chord, mac } from './keys.js';
import * as Sel from './remapselectrules.js';

export const LIST_HELP =
  '↑ ↓ move. Space selects. ⇧ with ↑ ↓ or a click selects a range. ' +
  `${mac ? 'Cmd' : 'Ctrl'}-click adds a row. Drag selects the rows it passes. Enter opens the choice. Esc clears.`;

const FIELD = 'select, input, button, a';

// box holds the rows; s is the selection, kept by the caller across renders.
export function selectList(box, s) {
  const rows = () => [...box.querySelectorAll('[data-row]')];
  Sel.setKeys(
    s,
    rows().map((r) => r.dataset.row),
  );
  box.setAttribute('role', 'listbox');
  box.setAttribute('aria-multiselectable', 'true');
  box.setAttribute('aria-label', 'Router names');

  const paint = () => {
    const cur = s.keys[s.cursor] ?? s.keys[0];
    for (const r of rows()) {
      const on = s.sel.has(r.dataset.row);
      r.classList.toggle('sode__xi-name--sel', on);
      r.classList.toggle('sode__xi-name--cur', s.keys[s.cursor] === r.dataset.row);
      r.setAttribute('aria-selected', String(on));
      r.tabIndex = r.dataset.row === cur ? 0 : -1;
    }
  };
  const focusCur = () =>
    rows()
      .find((r) => r.dataset.row === s.keys[s.cursor])
      ?.focus();

  box.addEventListener('keydown', (e) => {
    const c = chord(e);
    if (c === 'escape') {
      if (!s.sel.size) return;
      e.preventDefault();
      e.stopPropagation();
      Sel.clear(s);
      paint();
      return;
    }
    if (e.target.closest(FIELD)) return;
    const row = e.target.closest('[data-row]');
    if (row && s.keys[s.cursor] !== row.dataset.row) s.cursor = s.keys.indexOf(row.dataset.row);
    const step = { down: 1, up: -1, 'shift+down': 1, 'shift+up': -1 }[c];
    if (step) {
      Sel.move(s, step, c.startsWith('shift+'));
      focusCur();
    } else if (c === ' ') Sel.toggle(s);
    else if (c === 'enter') {
      const sel = rows()
        .find((r) => r.dataset.row === s.keys[s.cursor])
        ?.querySelector('select');
      if (!sel) return;
      sel.focus();
      try {
        sel.showPicker();
      } catch {
        // showPicker needs a user gesture in some browsers; the focused select still opens on Space.
      }
    } else return;
    e.preventDefault();
    paint();
  });

  box.addEventListener('pointerdown', (e) => {
    const row = e.target.closest('[data-row]');
    if (!row || e.button !== 0 || e.target.closest(FIELD)) return;
    const add = addClick(e);
    if (e.shiftKey || add) Sel.click(s, row.dataset.row, { shift: e.shiftKey, add });
    else {
      Sel.click(s, row.dataset.row);
      Sel.dragFrom(s, row.dataset.row);
    }
    e.preventDefault(); // no text selection while dragging
    row.focus();
    paint();
  });
  box.addEventListener('pointermove', (e) => {
    if (s.drag < 0 || !(e.buttons & 1)) return;
    const row = document.elementFromPoint(e.clientX, e.clientY)?.closest('[data-row]');
    if (!row || !box.contains(row)) return;
    Sel.dragTo(s, row.dataset.row);
    paint();
  });
  const end = () => Sel.dragEnd(s);
  box.addEventListener('pointerup', end);
  box.addEventListener('pointercancel', end);
  box.addEventListener('lostpointercapture', end);
  // A Mac's Ctrl-click is a context menu; Cmd adds there.
  box.addEventListener('contextmenu', (e) => {
    if (mac && e.ctrlKey && e.target.closest('[data-row]') && !e.target.closest(FIELD)) e.preventDefault();
  });
  paint();
}
