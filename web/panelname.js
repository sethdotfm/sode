// The instance name in the panel header. A click turns it into a field; Enter or leaving saves, Escape drops the edit.
import { el, icon } from './core.js';

// box is the header slot; rename(name) returns true when the name changed. Returns show(name, editable).
export function initName(box, rename) {
  const draw = (name, editable) => {
    box.hidden = false;
    if (!editable) return box.replaceChildren(el('span', 'sode-panels__device', name));
    const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode-panels__device', name, icon('pencil'));
    b.type = 'button';
    b.dataset.pbTip = 'Rename this screen';
    b.setAttribute('aria-label', `Screen name ${name}. Rename`);
    b.addEventListener('click', () => edit(name));
    box.replaceChildren(b);
  };
  function edit(name) {
    const input = el('input', 'pb-input sode-panels__device-input');
    input.type = 'text';
    input.maxLength = 40;
    input.value = name;
    input.autocomplete = 'off';
    input.setAttribute('aria-label', 'Screen name');
    let done = false;
    const finish = (save) => {
      if (done) return;
      done = true;
      if (!save || !rename(input.value)) draw(name, true);
    };
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') finish(true);
      else if (e.key === 'Escape') finish(false);
    });
    input.addEventListener('blur', () => finish(true));
    box.replaceChildren(input);
    input.focus();
    input.select();
  }
  return draw;
}
