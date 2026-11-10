// The Copy to dialog: pick the Page to copy onto by number. An empty Page takes the copy at once; one with cells
// asks first. A plain dialog, not the churning-letter Confirm, since undo reverses a Copy to (as editorpasteedge.js).
import { el, on } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { opt } from './formbits.js';
import { pageLabel } from './panelpages.js';

let closeOpen = null;
on('locking', () => closeOpen?.());

// Answers the target Page's index, or null (Cancel, Escape, the backdrop or the Lock). back gets focus on close.
export function askCopyTo(pages, from, back) {
  return new Promise((done) => {
    closeOpen?.();
    const button = (id, cls, text) => Object.assign(el('button', cls, text), { type: 'button', id });
    const go = button('ed-copyto-go', 'pb-btn pb-btn--ok', 'Copy');
    const cancel = button('ed-copyto-cancel', 'pb-btn pb-btn--outline', 'Cancel');
    const pick = el('select', 'pb-select');
    pick.id = 'ed-copyto-page';
    pick.append(...pages.map((p, i) => (i === from ? null : opt(String(i), `Page ${i + 1}${p.name?.trim() ? `, ${pageLabel(p, i)}` : ''}`))).filter(Boolean));
    const warn = el('p', 'pb-note pb-note--warn');
    warn.id = 'ed-copyto-warn';
    warn.hidden = true;

    const root = el('div', 'pb-modal-backdrop sode__scrim');
    root.id = 'ed-copyto';
    const dialog = el('div', 'pb-modal');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    dialog.setAttribute('aria-labelledby', 'ed-copyto-title');
    dialog.tabIndex = -1;
    dialog.append(
      el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', `Copy Page ${from + 1} to`), { id: 'ed-copyto-title' })),
      el(
        'div',
        'pb-modal__body',
        el('div', 'sode__field', Object.assign(el('label', null, 'Page'), { htmlFor: 'ed-copyto-page' }), el('span', 'pb-select-wrap', pick)),
        warn,
        el('p', 'sode__hint', 'Go to Page cells in the copy keep their targets. Undo reverses a Copy to.'),
      ),
      el('div', 'pb-modal__footer', go, cancel),
    );
    root.append(dialog);
    document.body.append(root);
    const dlg = trapDialog(root, dialog, () => close(null));
    function close(answer) {
      closeOpen = null;
      dlg.hide();
      root.remove();
      if (back?.isConnected) back.focus({ preventScroll: true });
      done(answer);
    }
    closeOpen = () => close(null);
    // The question about cells it replaces stands until the pick changes.
    let asked = -1;
    pick.addEventListener('change', () => {
      asked = -1;
      warn.hidden = true;
      go.textContent = 'Copy';
    });
    go.addEventListener('click', () => {
      const to = Number(pick.value);
      const n = pages[to]?.cells?.length || 0;
      if (!n || asked === to) return close(to);
      asked = to;
      warn.textContent = `Page ${to + 1} has ${n} cell${n > 1 ? 's' : ''}. The copy replaces ${n > 1 ? 'them' : 'it'}.`;
      warn.hidden = false;
      go.textContent = 'Replace';
      go.focus();
    });
    cancel.addEventListener('click', () => close(null));
    dlg.show();
    pick.focus();
  });
}
