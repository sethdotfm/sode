// Keys inside a dialog. No imports, shared by widgets.js and the lock chip.

// Keys stay in a dialog: Esc calls onEsc, Tab wraps round its controls. They
// are stopped at root so the layouts' own keys don't act behind it.
export function trapDialog(root, dialog, onEsc) {
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
