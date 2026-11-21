// The Show PIN eye inside every PIN box. Imports only core.js.
import { el, icon } from './core.js';

function paint(input, b) {
  const shown = input.type === 'text';
  b.replaceChildren(icon(shown ? 'eye-off' : 'eye'));
  b.dataset.pbTip = shown ? 'Hide PIN' : 'Show PIN';
  b.setAttribute('aria-label', b.dataset.pbTip);
  b.setAttribute('aria-pressed', String(shown));
}

// Gives the box that holds input and its eye; put it where the input went.
export function pinShow(input) {
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__pinshow-btn');
  b.type = 'button'; // in a form, a plain button submits it
  b.classList.add('pb-tip--end');
  paint(input, b);
  // mousedown keeps the focus in the box, so typing goes on.
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    input.type = input.type === 'text' ? 'password' : 'text';
    paint(input, b);
  });
  return el('span', 'sode__pinshow', input, b);
}

// A PIN box that closes or clears hides its PIN again, so the next person does not see it.
export function hidePin(input) {
  input.type = 'password';
  const b = input.parentElement?.querySelector('.sode__pinshow-btn');
  if (b) paint(input, b);
}
