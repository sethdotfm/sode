// The toolbar's Name: the saved name as text beside a Rename button. A click swaps in the field (#ed-name stays the one
// source of the name for the rest of the editor); Enter or leaving swaps it back.
import { $, el } from './core.js';

export function initNameEdit() {
  const input = $('ed-name');
  const field = input.closest('label');
  const text = el('span', 'sode-ed__name-text');
  const btn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Rename');
  btn.type = 'button';
  btn.id = 'ed-name-btn';
  const shown = el('span', 'sode-ed__name', text, btn);
  field.before(shown);
  const draw = () => {
    text.textContent = input.value || 'Untitled panel';
  };
  const show = (editing) => {
    field.hidden = !editing;
    shown.hidden = editing;
    if (editing) {
      input.focus();
      input.select();
    } else draw();
  };
  show(false);
  btn.addEventListener('click', () => show(true));
  input.addEventListener('blur', () => show(false));
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === 'Escape') {
      e.preventDefault();
      show(false);
      btn.focus();
    }
  });
  input.addEventListener('input', draw);
  return { draw };
}
