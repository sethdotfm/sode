// The "Show all sources / destinations" button under a patch box. An empty or
// confirmed box lists nothing on focus; this button (or ↓/↑) opens its whole
// list instead. It shows whenever the focused box has no list open: empty,
// confirmed, or a search that found nothing.

// box: { input, cb, list }; menu: the list's <ul>; btn: the button's row;
// chosen(): the confirmed item, if any. Returns { sync, open } for the page.
export function showAll(box, menu, btn, chosen) {
  const sync = () => { btn.hidden = box.cb.open || document.activeElement !== box.input; };
  // The whole list, on the chosen row (or the first, its cursor hidden until ↓).
  const open = () => {
    box.input.focus();
    box.cb.browse(box.list(), chosen()?.n);
  };
  new MutationObserver(sync).observe(menu, { attributes: true, attributeFilter: ['hidden'] });
  btn.addEventListener('mousedown', (e) => e.preventDefault()); // the box keeps focus
  btn.addEventListener('click', open);
  box.input.addEventListener('blur', sync);
  return { sync, open };
}
