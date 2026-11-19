// The Copy to dialog: pick the Page to copy onto by number. An empty Page gets the copy immediately; one with cells
// asks first. A plain dialog, not the churning-letter Confirm, since undo reverses a Copy to (as editorpasteedge.js).
import { el } from './core.js';
import { btn, opt, labeled } from './formbits.js';
import { question } from './modal.js';
import { pageLabel } from './panelpages.js';

let shown = null;

// Answers the target Page's index, or null (Cancel, Escape, the backdrop or the Lock). back gets focus on close.
export function askCopyTo(pages, from, back) {
  shown?.close(null);
  const pick = el('select', { class: 'pb-select', id: 'ed-copyto-page' });
  pick.append(...pages.map((p, i) => (i === from ? null : opt(String(i), `Page ${i + 1}${p.name?.trim() ? `, ${pageLabel(p, i)}` : ''}`))).filter(Boolean));
  const warn = el('p', { class: 'pb-note pb-note--warn', id: 'ed-copyto-warn', hidden: true });
  // The question about cells it replaces stands until the pick changes.
  let asked = -1;
  const go = btn(
    'pb-btn--ok',
    'Copy',
    () => {
      const to = Number(pick.value);
      const n = pages[to]?.cells?.length || 0;
      if (!n || asked === to) return q.close(to);
      asked = to;
      warn.textContent = `Page ${to + 1} has ${n} cell${n > 1 ? 's' : ''}. The copy replaces ${n > 1 ? 'them' : 'it'}.`;
      warn.hidden = false;
      go.textContent = 'Replace';
      go.focus();
    },
    'ed-copyto-go',
  );
  pick.addEventListener('change', () => {
    asked = -1;
    warn.hidden = true;
    go.textContent = 'Copy';
  });
  const q = (shown = question({
    back,
    titleId: 'ed-copyto-title',
    title: `Copy Page ${from + 1} to`,
    rootId: 'ed-copyto',
    body: [labeled('ed-copyto-page', 'Page', el('span', 'pb-select-wrap', pick)), warn],
    foot: [go, btn('pb-btn--outline', 'Cancel', () => q.close(null), 'ed-copyto-cancel')],
  }));
  pick.focus();
  return q.answer;
}
