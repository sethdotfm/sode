// Small form builders shared by the Settings panels that build their fields in code.

import { el } from './core.js';

export const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  if (fn) b.addEventListener('click', fn);
  return b;
};

export const opt = (value, text) => Object.assign(el('option', null, text), { value });

// A label tied to a control the caller already wrapped (pb-select-wrap, pb-number-wrap) or left bare.
export const labeled = (id, text, control) => el('div', 'sode__field', Object.assign(el('label', null, text), { htmlFor: id }), control);

export const field = (id, text, control, hint) =>
  el('div', 'sode__field', Object.assign(el('label', null, text), { htmlFor: id }), control, ...(hint ? [el('span', 'sode__hint', hint)] : []));

// A checkbox with its label: {input, label}. onChange gets the new checked state.
export const check = (text, { id, checked = false, onChange } = {}) => {
  const input = el('input', 'pb-check__input');
  input.type = 'checkbox';
  if (id) input.id = id;
  input.checked = checked;
  if (onChange) input.addEventListener('change', () => onChange(input.checked));
  return { input, label: el('label', 'pb-check', input, el('span', 'pb-check__label', text)) };
};
