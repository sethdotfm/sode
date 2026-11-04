// Keys inside a dialog. No imports, shared by widgets.js and the lock chip.

// Dialogs can stack (the users editor, a Profile on top of it, Export / import
// on top of that). Each is registered by trapDialog, which hands back
// { show, hide }; show() opens it and marks it the newest. Only the newest
// open one handles Esc, wherever focus is. Open-ness is read from the root's
// `hidden` at Esc time, so a dialog something else hid (lock.js closeModals)
// is simply skipped.
const stack = []; // { root, onEsc, order }
let shows = 0;
let listening = false;

// The top dialog of entries: open ones only, the most recently shown wins.
export function topDialog(entries) {
  let top = null;
  for (const e of entries) {
    if (e.root.hidden || !e.root.isConnected) continue;
    if (!top || e.order > top.order) top = e;
  }
  return top;
}

function listen() {
  if (listening) return;
  listening = true;
  // Capture, so it runs before a page's own key handlers. Only a key that is
  // NOT inside the top dialog is taken over here (focus left behind on the
  // page, or still inside a dialog that now has another on top of it). Inside
  // the top dialog the key goes the usual way, so a field's own Esc (clearing
  // a box first) still comes before the dialog's.
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    const top = topDialog(stack);
    if (!top || top.root.contains(e.target)) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    top.onEsc();
  }, true);
}

// Keys stay in a dialog: Esc calls onEsc, Tab wraps round its controls. They
// are stopped at root so the layouts' own keys don't act behind it. Returns
// { show(), hide() }: a dialog opens and closes through it.
export function trapDialog(root, dialog, onEsc) {
  const entry = { root, onEsc, order: 0 };
  stack.push(entry);
  listen();
  root.addEventListener('keydown', (e) => {
    e.stopPropagation();
    if (e.key === 'Escape') { e.preventDefault(); onEsc(); return; }
    if (e.key !== 'Tab') return;
    const all = [...dialog.querySelectorAll('button, input, select')].filter((x) => !x.disabled && x.offsetParent);
    const first = all[0], last = all[all.length - 1];
    if (e.shiftKey && (document.activeElement === first || document.activeElement === dialog)) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  });
  return {
    show() { root.hidden = false; entry.order = ++shows; },
    hide() { root.hidden = true; },
  };
}
