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

// A press closes a dialog only when the mouse went down AND came up on the
// backdrop (root itself), the targets of mousedown and mouseup. The click's
// own target is not enough: a press on the backdrop released inside the
// dialog (or the reverse) fires its click on the common ancestor, root, and a
// text selection dragged out of the dialog does the same.
export function backdropClose(downTarget, upTarget, root) {
  return downTarget === root && upTarget === root;
}

// Stacking: scrims share one z-index, so DOM order decides. Moving root to the
// end of its parent puts it in front. A no-op when already last (focus and
// scroll survive), and focus inside it is put back if the move dropped it.
export function bringToFront(root) {
  const parent = root.parentNode;
  if (!parent || parent.lastElementChild === root) return;
  const had = root.contains(document.activeElement) ? document.activeElement : null;
  parent.appendChild(root);
  if (had && document.activeElement !== had) had.focus({ preventScroll: true });
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
  let down = null, up = null;
  root.addEventListener('mousedown', (e) => { down = e.target; up = null; });
  root.addEventListener('mouseup', (e) => { up = e.target; });
  root.addEventListener('click', (e) => {
    const d = down, u = up;
    down = up = null;
    if (e.target === root && backdropClose(d, u, root)) onEsc();
  });
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
    show() { root.hidden = false; bringToFront(root); entry.order = ++shows; },
    hide() { root.hidden = true; },
  };
}
