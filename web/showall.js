// "Show all" button under a patch box: an empty or confirmed box lists nothing on focus; this (or arrows) opens the whole list.

// box: { input, cb, list }; chosen(): the confirmed item. sync() is for the combobox onRender and focus; open() for the button and arrows.
export function showAll(box, btn, chosen) {
  const sync = () => { btn.hidden = box.cb.open || document.activeElement !== box.input; };
  const open = () => {
    box.input.focus();
    box.cb.browse(box.list(), chosen()?.n);
  };
  btn.addEventListener('mousedown', (e) => e.preventDefault()); // the box keeps focus
  btn.addEventListener('click', open);
  box.input.addEventListener('blur', sync);
  return { sync, open };
}
