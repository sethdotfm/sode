// The one modal scaffold. Esc, a backdrop click and a Lock (closeShown in dialogkeys.js) all run onEsc.
import { el } from './core.js';
import { trapDialog } from './dialogkeys.js';

export const dialogAttrs = (titleId) => ({ role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': titleId, tabIndex: -1 });

// body is a list of nodes for a pb-modal__body (none: no body), or one node used as it is. A modal built per open calls
// remove() when it closes, so nothing it holds outlives it.
export function modal({ titleId, title, cls = '', rootCls = '', rootId, dialogId, head = [], headCls = '', body = [], foot = [], onEsc, describedBy }) {
  const titleEl = el('span', { class: 'pb-modal__title', id: titleId }, title);
  const dialog = el(
    'div',
    { class: `pb-modal ${cls}`.trim(), id: dialogId, ...dialogAttrs(titleId), 'aria-describedby': describedBy },
    el('div', `pb-modal__header ${headCls}`.trim(), titleEl, ...head),
    ...(Array.isArray(body) ? (body.length ? [el('div', 'pb-modal__body', ...body)] : []) : [body]),
    el('div', 'pb-modal__footer', ...foot),
  );
  const root = el('div', { class: `pb-modal-backdrop sode__scrim ${rootCls}`.trim(), id: rootId, hidden: true }, dialog);
  document.body.append(root);
  const { show, hide } = trapDialog(root, dialog, onEsc);
  return {
    root,
    dialog,
    title: titleEl,
    show,
    hide,
    shown: () => !root.hidden,
    remove() {
      hide();
      root.remove();
    },
  };
}

// A modal built for one answer and removed with it: close(v) resolves answer with v; Esc, the backdrop and a Lock
// give null. back gets focus again.
export function question({ back, ...opts }) {
  let done;
  const answer = new Promise((r) => (done = r));
  const m = modal({ ...opts, onEsc: () => close(null) });
  function close(v) {
    if (!m.root.isConnected) return;
    m.remove();
    if (back?.isConnected) back.focus({ preventScroll: true });
    done(v);
  }
  m.show();
  return { ...m, answer, close };
}
