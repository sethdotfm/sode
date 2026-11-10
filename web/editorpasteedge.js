// The dialog for a paste past the edge of the grid: Clip, Wrap or Cancel. A plain dialog, not the churning-letter
// Confirm, since undo reverses either choice. The counts and words are gridedge.js edgeChoices.
import { el, on } from './core.js';
import { trapDialog } from './dialogkeys.js';

let closeOpen = null;
on('locking', () => closeOpen?.());

// Answers 'clip', 'wrap' or null (Cancel, Escape, the backdrop or the Lock). back gets focus on close.
export function askPasteEdge(choices, outside, back) {
  return new Promise((done) => {
    closeOpen?.();
    const btn = (id, cls, text) => {
      const b = el('button', cls, text);
      b.type = 'button';
      b.id = id;
      return b;
    };
    const clip = btn('ed-edge-clip', 'pb-btn pb-btn--outline', 'Clip');
    const wrap = btn('ed-edge-wrap', 'pb-btn pb-btn--outline', 'Wrap');
    const cancel = btn('ed-edge-cancel', 'pb-btn pb-btn--outline', 'Cancel');
    const row = [clip, wrap, cancel];

    const root = el('div', 'pb-modal-backdrop sode__scrim');
    root.id = 'ed-edge';
    const dialog = el('div', 'pb-modal');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    dialog.setAttribute('aria-labelledby', 'ed-edge-title');
    dialog.setAttribute('aria-describedby', 'ed-edge-body');
    dialog.tabIndex = -1;
    const body = el(
      'div',
      'pb-modal__body',
      el('p', 'sode__hint', `${outside === 1 ? '1 cell runs' : `${outside} cells run`} past the edge of the grid.`),
      el('p', null, choices.clip.text),
      el('p', null, choices.wrap.text),
      el('p', 'sode__hint', `Fill order: ${choices.orderName}, as picked in Find. Undo reverses either choice.`),
    );
    body.id = 'ed-edge-body';
    dialog.append(
      el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Paste past the edge'), { id: 'ed-edge-title' })),
      body,
      el('div', 'pb-modal__footer', ...row),
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
    clip.addEventListener('click', () => close('clip'));
    wrap.addEventListener('click', () => close('wrap'));
    cancel.addEventListener('click', () => close(null));
    // Left and right (and up and down) step through the buttons; Enter presses the focused one, as any button does.
    dialog.addEventListener('keydown', (e) => {
      const step = { ArrowLeft: -1, ArrowUp: -1, ArrowRight: 1, ArrowDown: 1 }[e.key];
      if (!step) return;
      e.preventDefault();
      const i = row.indexOf(document.activeElement);
      row[(i < 0 ? 0 : i + step + row.length) % row.length].focus();
    });
    dlg.show();
    clip.focus();
  });
}
