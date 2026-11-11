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

// A row of exclusive choices on patchbay's pb-btn-group, for two or three settings. Same radio-plus-label pairs as the
// link-kind picker (panelkeytab.js). onChange gets the chosen value; set() never fires it.
export const segmented = (id, text, options, onChange) => {
  const group = el('div', 'pb-btn-group');
  group.id = id;
  group.setAttribute('role', 'radiogroup');
  group.setAttribute('aria-label', text);
  const radios = options.map(([value, label]) => {
    const r = el('input', 'pb-btn-check');
    r.type = 'radio';
    r.name = id;
    r.id = `${id}-${value}`;
    r.value = value;
    r.addEventListener('change', () => r.checked && onChange?.(value));
    group.append(r, Object.assign(el('label', 'pb-btn pb-btn--outline pb-btn--sm', label), { htmlFor: r.id }));
    return r;
  });
  return {
    node: group,
    set: (v) => radios.forEach((r) => (r.checked = r.value === v)),
    disable: (off) => radios.forEach((r) => (r.disabled = off)),
  };
};
