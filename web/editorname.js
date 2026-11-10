// The toolbar's Name: the saved name as a button with a pencil. A click swaps in the field (#ed-name stays the one
// source of the name for the rest of the editor); Enter or leaving swaps it back.
import { $, el, icon } from './core.js';

export function initNameEdit() {
  const input = $('ed-name');
  const field = input.closest('label');
  const text = el('span', 'sode-ed__name-text');
  const btn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode-ed__name', text, icon('pencil'));
  btn.type = 'button';
  btn.id = 'ed-name-btn';
  btn.dataset.pbTip = 'Rename this panel';
  field.before(btn);
  const draw = () => {
    text.textContent = input.value || 'Untitled panel';
    btn.setAttribute('aria-label', `Panel name ${text.textContent}. Rename`);
  };
  const show = (editing) => {
    field.hidden = !editing;
    btn.hidden = editing;
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
