// The dialog for a paste past the edge of the grid: Clip, Wrap or Cancel. A plain dialog, not the churning-letter
// Confirm, since undo reverses either choice. The counts and words are gridedge.js edgeChoices.
import { el } from './core.js';
import { btn } from './formbits.js';
import { question } from './modal.js';

let shown = null;

// Answers 'clip', 'wrap' or null (Cancel, Escape, the backdrop or the Lock). back gets focus on close.
export function askPasteEdge(choices, outside, back) {
  shown?.close(null);
  const answer = (v) => () => q.close(v);
  const row = [
    btn('pb-btn--outline', 'Clip', answer('clip'), 'ed-edge-clip'),
    btn('pb-btn--outline', 'Wrap', answer('wrap'), 'ed-edge-wrap'),
    btn('pb-btn--outline', 'Cancel', answer(null), 'ed-edge-cancel'),
  ];
  const q = (shown = question({
    back,
    titleId: 'ed-edge-title',
    title: 'Paste past the edge',
    rootId: 'ed-edge',
    describedBy: 'ed-edge-body',
    body: el(
      'div',
      { class: 'pb-modal__body', id: 'ed-edge-body' },
      el('p', 'sode__hint', `${outside === 1 ? '1 cell runs' : `${outside} cells run`} past the edge of the grid.`),
      el('p', null, choices.clip.text),
      el('p', null, choices.wrap.text),
      el('p', 'sode__hint', `Fill order: ${choices.orderName}`),
    ),
    foot: row,
  }));
  // Left and right (and up and down) step through the buttons; Enter presses the focused one, as any button does.
  q.dialog.addEventListener('keydown', (e) => {
    const step = { ArrowLeft: -1, ArrowUp: -1, ArrowRight: 1, ArrowDown: 1 }[e.key];
    if (!step) return;
    e.preventDefault();
    const i = row.indexOf(document.activeElement);
    row[(i < 0 ? 0 : i + step + row.length) % row.length].focus();
  });
  row[0].focus();
  return q.answer;
}
