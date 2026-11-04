// Keys inside a dialog. No imports, shared by widgets.js and the lock chip.

// Dialogs can stack (the users editor, a Profile on top of it, Export / import
// on top of that). They are kept in the order they were shown, and only the
// most recent one handles Esc, wherever focus is. A dialog joins the stack by
// showing its root (`root.hidden = false`) and leaves it by hiding it again,
// which trapDialog watches, so no dialog needs to know about the others.
const open = []; // { root, onEsc }, oldest first
let listening = false;

function listen() {
  if (listening) return;
  listening = true;
  // Capture, so it runs before a page's own key handlers. Only a key that is
  // NOT inside the top dialog is taken over here (focus left behind on the
  // page, or still inside a dialog that now has another on top of it). Inside
  // the top dialog the key goes the usual way, so a field's own Esc (clearing
  // a box first) still comes before the dialog's.
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !open.length) return;
    const top = open[open.length - 1];
    if (top.root.contains(e.target)) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    top.onEsc();
  }, true);
}

// Keys stay in a dialog: Esc calls onEsc, Tab wraps round its controls. They
// are stopped at root so the layouts' own keys don't act behind it.
export function trapDialog(root, dialog, onEsc) {
  const entry = { root, onEsc };
  const sync = () => {
    const at = open.indexOf(entry);
    const shown = !root.hidden && root.isConnected;
    if (shown && at < 0) open.push(entry);
    else if (!shown && at >= 0) open.splice(at, 1);
  };
  new MutationObserver(sync).observe(root, { attributes: true, attributeFilter: ['hidden'] });
  sync();
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
}
